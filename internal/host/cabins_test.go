package host

import (
	"fmt"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/extras"
	"github.com/adamf/wholesky/internal/world"
)

// The cabins a carrier's aircraft have. A network carrier's narrow-bodies
// have a front cabin of two-and-two rows: 12 seats on an A320, 16 on a
// 737-800 and 20 on an A321. A low-cost carrier's have one cabin. A
// wide-body has a business cabin whoever flies it, and the inventory a
// tenant sells from has the same cabins the filler fills.
func TestCabinsByKindOfCarrier(t *testing.T) {
	var network, lowCost string
	for _, c := range []string{"AA", "UA", "DL", "LH", "BA", "AF"} {
		if equipment.ModelOf(c) == equipment.Network {
			network = c
			break
		}
	}
	for _, c := range []string{"WN", "U2", "FR", "B6"} {
		if equipment.ModelOf(c) != equipment.Network {
			lowCost = c
			break
		}
	}
	if network == "" || lowCost == "" {
		t.Fatalf("no carrier of each kind: %q, %q", network, lowCost)
	}
	for _, c := range []struct {
		code       string
		seats      int
		front, all int // a network carrier's front cabin and whole aircraft
	}{{"320", 180, 12, 174}, {"73H", 174, 16, 166}, {"321", 220, 20, 210}} {
		n := Cabins(world.Flight{Carrier: network, Equipment: c.code, Seats: c.seats}, 0)
		if n["C"] != c.front || n["C"]+n["Y"] != c.all || len(n) != 2 {
			t.Errorf("%s's %s: %v, want %d at the front of %d", network, c.code, n, c.front, c.all)
		}
		l := Cabins(world.Flight{Carrier: lowCost, Equipment: c.code, Seats: c.seats}, 0)
		if len(l) != 1 || l["Y"] != c.seats {
			t.Errorf("%s's %s: %v, want one cabin of %d", lowCost, c.code, l, c.seats)
		}
	}
	wide := Cabins(world.Flight{Carrier: lowCost, Equipment: "77W", Seats: 360}, 0)
	if wide["C"] == 0 || wide["C"] != Cabins(world.Flight{Carrier: network, Equipment: "77W", Seats: 360}, 0)["C"] {
		t.Errorf("a 777 has a business cabin whoever flies it: %v", wide)
	}
	// The inventory a tenant sells from, and a leg it markets and another
	// carrier flies: the operating carrier's cabins.
	own := world.Flight{Carrier: network, Number: "0100", From: "ORD", To: "DCA", Equipment: "320", Seats: 180}
	sold := world.Flight{Carrier: lowCost, Number: "0200", From: "DCA", To: "BOS", Equipment: "320", Seats: 180, Marketing: network, MarketingNumber: "5200"}
	capacity := capacityFor(network, []world.Flight{own, sold}, 0)
	if c, ok := capacity(network, "100", "26NOV", "ORD"); !ok || c["C"] != 12 || c["Y"] != 162 {
		t.Errorf("the tenant's own A320: %v", c)
	}
	if c, ok := capacity(network, "5200", "26NOV", "DCA"); !ok || c["C"] != 0 || c["Y"] != 180 {
		t.Errorf("the leg it markets on a low-cost carrier's A320: %v", c)
	}
}

// Who misses a connection. A passenger with an hour in hand over the
// station's minimum makes it when the inbound is on time. One whose
// inbound is 90 minutes late with 30 in hand does not. At the point where
// the delay eats the slack and the ten minutes a passenger really has
// beyond the minimum, half make it. A departure that is itself late
// waits for them without meaning to.
func TestWhoMissesAConnection(t *testing.T) {
	rate := func(slack, inbound, outbound int) float64 {
		missed := 0
		for h := 0; h < 7000; h++ {
			if missesConnection(h, slack, inbound, outbound) {
				missed++
			}
		}
		return float64(missed) / 7000
	}
	if r := rate(60, 0, 0); r != 0 {
		t.Errorf("on time with an hour in hand: %.3f miss", r)
	}
	if r := rate(30, 90, 0); r < 0.999 {
		t.Errorf("90 minutes late with 30 in hand: %.3f miss", r)
	}
	if r := rate(20, 30, 0); r < 0.47 || r > 0.53 {
		t.Errorf("the delay equal to the slack and ten minutes: %.3f miss, want half", r)
	}
	if late, held := rate(20, 60, 0), rate(20, 60, 40); late < 0.99 || held > 0.2 {
		t.Errorf("an hour late: %.3f miss; with the departure 40 minutes late too: %.3f", late, held)
	}
	// The same passenger gets the same answer every time.
	if missesConnection(4321, 20, 30, 0) != missesConnection(4321, 20, 30, 0) {
		t.Error("a passenger both made and missed a connection")
	}
	// The station's own time, where the carrier has been told it.
	tn := &Tenant{connect: func(in, out world.Flight) int {
		if in.To == "LHR" {
			return 75
		}
		return 0
	}}
	if got := tn.connectTime(world.Flight{To: "LHR"}, world.Flight{}); got != 75 {
		t.Errorf("Heathrow's time: %d", got)
	}
	if got := tn.connectTime(world.Flight{To: "XXX"}, world.Flight{}); got != minimumConnection {
		t.Errorf("a station with no time of its own: %d", got)
	}
}

// A late inbound flight on the day. A carrier's departure from Denver has
// one inbound flight that lands 90 minutes before it, which is 45 minutes
// more than the station needs. With the inbound on time the connecting
// parties make the flight. With it 2 hours late they do not, and each is
// counted once however often the day asks. At a station that needs 100
// minutes the 90 were never a connection, and nobody is booked across
// them.
func TestALateInboundFlightOnTheDay(t *testing.T) {
	out := world.Flight{Carrier: "ZZ", Number: "0200", From: "DEN", To: "SFO", DepMin: 600, ArrMin: 750}
	in := world.Flight{Carrier: "ZZ", Number: "0100", From: "ORD", To: "DEN", DepMin: 350, ArrMin: 510}
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	late := 0
	tn := &Tenant{flights: []world.Flight{in, out}, missed: map[string]Missed{},
		inboundDelay: func(world.Flight, time.Time) int { return late }}
	parties := make([]party, 2000)
	for i := range parties {
		parties[i] = party{key: fmt.Sprintf("P%04d", i), pax: make([]*dcs.Passenger, 1+i%3)}
	}
	run := func() (connected, absent int) {
		for _, pt := range parties {
			_, conn, ok := tn.arrival(pt, out, day)
			if conn != nil {
				connected++
			}
			if !ok {
				absent++
			}
		}
		return
	}
	connected, noShows := run()
	if connected < 200 || connected > 360 {
		t.Fatalf("%d of 2000 parties connect; about 14 in 100 should", connected)
	}
	// A party that makes the connection is at the counter by its last
	// wave, 46 minutes out. With 45 minutes in hand over the station's
	// time it comes late in the hour, and it must still be accepted.
	// An inbound flight that lands 50 minutes before the departure, at a
	// station that needs 45: most of its parties make the flight, with 40
	// minutes in hand at the gate.
	short := world.Flight{Carrier: "ZZ", Number: "0101", From: "ORD", To: "DEN", DepMin: 390, ArrMin: 550}
	tight := &Tenant{flights: []world.Flight{short, out}, missed: map[string]Missed{}}
	made := 0
	for _, pt := range parties {
		at, conn, ok := tight.arrival(pt, out, day)
		if !ok || conn == nil {
			continue
		}
		made++
		if at < lastWave {
			t.Fatalf("a connecting party that makes the flight comes %d minutes out, after the counter's last wave at %d: it is never accepted", at, lastWave)
		}
	}
	if made < 50 {
		t.Fatalf("%d parties made a 50 minute connection; most of about 280 should", made)
	}
	// Who stays away depends on the fare. Of 2000 parties on a fare that
	// cannot be refunded about 170 do not come, and of 2000 on full fare
	// about 340.
	if noShows < 130 || noShows > 210 {
		t.Errorf("%d of 2000 parties on a restricted fare do not come; about 8.5 in 100 should", noShows)
	}
	flexible := 0
	for _, pt := range parties {
		pt.lead = &dcs.Passenger{Class: "Y"}
		if _, _, ok := tn.arrival(pt, out, day); !ok {
			flexible++
		}
	}
	if flexible < 290 || flexible > 390 {
		t.Errorf("%d of 2000 parties on full fare do not come; about 17 in 100 should", flexible)
	}
	tn.NewDay()
	if tn.Misconnected() != 0 {
		t.Errorf("the inbound is on time and %d passengers missed", tn.Misconnected())
	}
	late = 120
	after, absent := run()
	if after != 0 || absent != noShows+connected {
		t.Errorf("2 hours late with 45 minutes in hand: %d still connect, %d absent, want none and %d", after, absent, noShows+connected)
	}
	missed := tn.Misconnected()
	if missed < connected || missed > 3*connected {
		t.Errorf("%d parties missed and the count is %d passengers", connected, missed)
	}
	run()
	if tn.Misconnected() != missed {
		t.Errorf("asked twice, the count went from %d to %d", missed, tn.Misconnected())
	}
	for _, m := range tn.Missed() {
		if m.Flight.Number != out.Number || m.Inbound.Number != in.Number || m.Pax < 1 {
			t.Fatalf("a missed connection %+v; want off %s onto %s", m, in.Number, out.Number)
		}
	}
	// The count is a day's.
	tn.NewDay()
	if tn.Misconnected() != 0 {
		t.Errorf("a new day began with %d missed connections", tn.Misconnected())
	}
	// The inbound flight is cancelled. Nobody comes off it, on time or
	// not, and each of its parties has missed the connection.
	late = 0
	tn.inboundCancelled = func(world.Flight, time.Time) bool { return true }
	if still, gone := run(); still != 0 || gone != noShows+connected {
		t.Errorf("with the inbound cancelled %d parties still connect and %d are absent; want none and %d", still, gone, noShows+connected)
	}
	if got := tn.Misconnected(); got < connected || got > 3*connected {
		t.Errorf("with the inbound cancelled %d passengers missed; %d parties were to connect", got, connected)
	}
	tn.inboundCancelled = nil
	tn.NewDay()
	// A late departure waits for them without meaning to.
	tn.departureDelay = func(world.Flight, time.Time) int { return 120 }
	if held, _ := run(); held != connected {
		t.Errorf("the departure is as late as the inbound and %d of %d connect", held, connected)
	}
	// The station needs 100 minutes: 90 are no connection.
	tn.departureDelay, late = nil, 0
	tn.connect = func(in, out world.Flight) int { return 100 }
	if strict, _ := run(); strict != 0 {
		t.Errorf("%d parties were booked across 90 minutes at a station that needs 100", strict)
	}
}

// What a flight's passengers bought beside the seat is counted for those
// who boarded. On an ultra-low-cost carrier nearly every bag pays. 200
// passengers board with 2 bags each, where their journey begins: bags are
// sold. 200 more board with 2 bags each off a connecting flight: they
// paid where they began, and no bag, cabin bag or priority is sold to
// them here. 200 with bags did not come and 200 were denied boarding:
// nothing is sold to them.
func TestExtrasAreSoldToThePassengersWhoFlew(t *testing.T) {
	leg := world.Flight{Carrier: "F9", Number: "0100", From: "DEN", To: "LAS", KM: 1000, DepMin: 600, ArrMin: 720}
	flight := func(status dcs.Status, inbound bool) *dcs.Flight {
		fl := &dcs.Flight{Key: dcs.Key{Flight: "F90100", Date: "26NOV", Board: "DEN"}}
		for i := 0; i < 200; i++ {
			p := &dcs.Passenger{ID: i + 1, Surname: fmt.Sprintf("PAX%03d", i), Given: "AMR", Party: fmt.Sprintf("P%03d", i),
				Class: "Q", Compartment: "Y", Status: status, Bags: []dcs.Bag{{Tag: "1", Weight: 20}, {Tag: "2", Weight: 20}}}
			if inbound {
				p.Inbound = &dcs.Connection{Flight: "F90050", Station: "DEN"}
			}
			fl.Passengers = append(fl.Passengers, p)
		}
		return fl
	}
	began, units := extrasOf(leg, flight(dcs.StatusBoarded, false), false)
	// 3 in 100 have their first bag free by standing, so of 400 bags
	// about 394 pay.
	if units[extras.Bag] < 380 || units[extras.Bag] > 400 || began[extras.Bag] < int64(units[extras.Bag])*6400 {
		t.Errorf("200 passengers with 2 bags each paid for %d bags, %d cents", units[extras.Bag], began[extras.Bag])
	}
	if units[extras.CarryOn] < 70 || units[extras.CarryOn] > 130 || units[extras.Seat] == 0 {
		t.Errorf("%d cabin bags and %d seats sold to 200 passengers; about half pay for each", units[extras.CarryOn], units[extras.Seat])
	}
	on, onUnits := extrasOf(leg, flight(dcs.StatusBoarded, true), false)
	if on[extras.Bag] != 0 || on[extras.CarryOn] != 0 || on[extras.Priority] != 0 {
		t.Errorf("passengers off a connecting flight paid again: %v", onUnits)
	}
	if on[extras.Seat] == 0 || on[extras.Drink] == 0 {
		t.Errorf("passengers off a connecting flight bought no seat and no drink on this flight: %v", onUnits)
	}
	for _, status := range []dcs.Status{dcs.StatusNoShow, dcs.StatusDenied, dcs.StatusOffloaded, dcs.StatusAccepted} {
		if sold, _ := extrasOf(leg, flight(status, false), false); len(sold) != 0 {
			t.Errorf("passengers who are %s bought %v", status, sold)
		}
	}
	// A bag that came off is not paid for.
	off := flight(dcs.StatusBoarded, false)
	for _, p := range off.Passengers {
		for i := range p.Bags {
			p.Bags[i].Offloaded = true
		}
	}
	if sold, _ := extrasOf(leg, off, false); sold[extras.Bag] != 0 {
		t.Errorf("bags that came off the aircraft paid %d cents", sold[extras.Bag])
	}
}
