package sim

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/extras"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rights"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// A flight sold past its seats, from the booking to the scorecard.
//
// The carrier's record says half the passengers of one flight stay away,
// so it lets the cabin hold a fifth more bookings than seats, which is
// the most any record is believed. The cabin is sold to that limit and
// not past it. Then the day comes, and about 9 in 10 of the booked
// passengers do: more than the seats.
//
// The airport must then account for every booked passenger as one of 4:
// flew, did not come, came off, or was denied boarding. No passenger who
// came and found no seat is reported as a no-show. The carrier's record
// of who came gets the true count, and the scorecard charges each denied
// boarding the carrier's offer or what the law gives.
func TestAnOversoldFlightFromBookingToScorecard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	// The flight with the fewest seats, all of them in one cabin, that
	// has a later flight over the same pair.
	var f world.Flight
	seats := 0
	for _, c := range m.Carriers {
		for _, g := range s.Flights[c.Designator] {
			cabins := host.Cabins(g, s.capacity)
			if len(cabins) != 1 || g.Marketing != "" {
				continue
			}
			if wait, overnight := s.nextFlight(g); overnight || wait < 30 {
				continue
			}
			if n := cabins["Y"]; n > 0 && (seats == 0 || n < seats) {
				f, seats = g, n
			}
		}
	}
	if seats == 0 {
		t.Fatal("no flight with one cabin and a later flight over its pair")
	}
	tn := s.Tenants[f.Carrier]
	day := s.BookingDate
	wire := strings.ToUpper(day.Format("02Jan"))

	// The carrier's record of who came, and the limit it gives.
	s.rm = rm.New()
	var classes []string
	for _, cl := range tariff.Ladder() {
		if tn.Inventory != nil && cl != "F" && cl != "J" && cl != "C" && cl != "D" {
			classes = append(classes, cl)
		}
	}
	cab := s.rm.Cabin(rm.Key(f.Carrier, f.Number, f.From, "Y"), classes)
	for d := 0; d < rm.Warm; d++ {
		cab.Show(seats, seats/2)
	}
	limit := cab.Authorize(seats, 0, rm.DeniedCost, 1)
	if limit != seats+seats/5 {
		t.Fatalf("the cabin of %d seats may hold %d bookings, want %d", seats, limit, seats+seats/5)
	}
	before, _, _ := cab.ShowRate(seats)

	// Sell to the limit, and try 1 more. The cabin is sold in the last
	// minutes of its selling window. Earlier in the day the carrier holds
	// its last seats back from a distribution system, for a sale of its
	// own that pays no booking fee, and lets them go as the flight's time
	// comes. The world's clock is the wall clock: with it left alone,
	// this test sold 1 to 5 seats short in the hours before the flight's
	// time, and passed after it.
	s.clock.Sync(time.Now(), float64(max(f.DepMin-46, 0)), s.clock.Warp())
	var locs []string
	for i := 0; i < limit+1; i++ {
		res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("OVER%c%c", 'A'+i/26, 'A'+i%26))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	deadline := time.Now().Add(60 * time.Second)
	for _, l := range locs {
		for {
			if ok, _ := s.Settled(ctx, l); ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s never settled", l)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	sold := 0
	for _, n := range tn.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, "Y") {
		sold += n
	}
	if sold != limit {
		t.Fatalf("%d seats sold in a cabin of %d seats authorised %d: want the limit, and not 1 more", sold, seats, limit)
	}
	// The cabin holds more than its seats, and that is not an oversell:
	// the carrier meant it.
	inv := s.Invariants()
	if !inv.OK() || inv.Overbooked != 1 {
		t.Fatalf("invariants: %d overbooked, oversold %+v; want 1 overbooked and none oversold", inv.Overbooked, inv.Oversold)
	}

	key := dcs.Key{Flight: f.Carrier + f.Number, Date: wire, Board: f.From}
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatalf("SendPNL: %v", err)
	}
	for wait := time.Now().Add(30 * time.Second); ; {
		if fl, err := tn.DCS.Flight(key); err == nil && fl.Counts().Listed == limit {
			break
		}
		if time.Now().After(wait) {
			fl, err := tn.DCS.Flight(key)
			if err != nil {
				t.Fatalf("departure control never opened %s: %v", key, err)
			}
			t.Fatalf("departure control never listed %d: %+v, complete %v, alerts %+v", limit, fl.Counts(), fl.Complete, fl.Alerts)
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, out := range []int{150, 120, 90, 60, 46} {
		if err := tn.CheckIn(ctx, f, day, out); err != nil {
			t.Fatalf("CheckIn at %d: %v", out, err)
		}
	}
	fl, _ := tn.DCS.Flight(key)
	c := fl.Counts()
	came := c.Accepted + c.Waiting
	if c.Accepted != seats || c.Waiting == 0 {
		t.Fatalf("after check-in: %+v; want the %d seats taken and passengers waiting", c, seats)
	}
	if err := tn.CloseCheckIn(ctx, f, day); err != nil {
		t.Fatalf("CloseCheckIn: %v", err)
	}
	if err := tn.Board(ctx, f, day, 14); err != nil {
		t.Fatalf("Board: %v", err)
	}
	// Nothing beside the seat is sold until the door closes on the
	// passengers who flew.
	if early, _ := tn.Extras(); len(early) != 0 {
		t.Errorf("extras were sold before the door closed: %v", early)
	}
	faresOnly := (seatWorld{s}).score(f.Carrier).Revenue
	if err := tn.Close(ctx, f, day); err != nil {
		t.Fatalf("Close: %v", err)
	}
	fl, _ = tn.DCS.Flight(key)
	c = fl.Counts()
	t.Logf("%s%s: %d seats, %d booked, %d came; %d flew, %d did not come, %d came off, %d denied boarding of whom %d volunteers",
		f.Carrier, f.Number, seats, limit, came, c.Boarded, c.NoShow, c.Offload, c.Denied, c.Volunteers)
	if c.Boarded+c.NoShow+c.Offload+c.Denied != limit {
		t.Errorf("%d booked, and %d flew, %d did not come, %d came off and %d were denied boarding", limit, c.Boarded, c.NoShow, c.Offload, c.Denied)
	}
	if c.Boarded > seats || c.Boarded < seats-2 {
		t.Errorf("%d flew in %d seats", c.Boarded, seats)
	}
	if c.NoShow != limit-came {
		t.Errorf("%d reported as no-shows; %d of the %d booked did not come", c.NoShow, limit-came, limit)
	}
	if c.Denied != came-c.Boarded-c.Offload || c.Denied == 0 {
		t.Errorf("%d denied boarding; %d came, %d flew and %d came off", c.Denied, came, c.Boarded, c.Offload)
	}
	// The agent asked for volunteers before anybody was turned away, and
	// on this flight there were too few.
	if c.Volunteers == 0 || c.Volunteers == c.Denied {
		t.Errorf("%d volunteers among %d denied boardings; this flight has some of each", c.Volunteers, c.Denied)
	}
	// The carrier still knows what the flight carried after it has landed
	// and departure control has forgotten it.
	tn.Forget(ctx, f, day)
	if n, ok := tn.Carried(f, day); !ok || n != c.Boarded {
		t.Errorf("the carrier counts %d carried (%v) and the door counted %d", n, ok, c.Boarded)
	}

	// The carrier's record of who came has the flight, with the true
	// count: the passengers denied boarding came.
	if got := cab.Shown(); got != rm.Warm+1 {
		t.Errorf("the record of who came holds %d departures, want %d", got, rm.Warm+1)
	}
	after, _, _ := cab.ShowRate(seats)
	// 7 departures of seats bookings at 0.5, then this one. Each earlier
	// departure keeps 25/27 of its weight at the next.
	w := 0.0
	for d := 0; d < rm.Warm; d++ {
		w = w*25/27 + float64(seats)
	}
	want := (w*25/27*0.5 + float64(came)) / (w*25/27 + float64(limit))
	if after < want-1e-9 || after > want+1e-9 || after <= before {
		t.Errorf("the show-up rate went from %.4f to %.4f, want %.4f: %d of %d came", before, after, want, came, limit)
	}

	// The scorecard: each volunteer has the carrier's offer, and each
	// passenger turned away what the law of the flight gives.
	denied := tn.Denied()
	if len(denied) != c.Denied {
		t.Fatalf("the carrier counts %d denied boardings and departure control %d", len(denied), c.Denied)
	}
	var fare int64
	for _, x := range s.tariff.Fares(f.Carrier, f.From, f.To) {
		if x.Class == "Y" && !x.RoundTrip && len(x.Rule.Sale.Accounts) == 0 && x.OneWay.Amount > fare {
			fare = x.OneWay.Amount
		}
	}
	wait, _ := s.nextFlight(f)
	from, to := s.airports[f.From], s.airports[f.To]
	reg := rights.Of(from.Country, to.Country, s.carriers[f.Carrier].Country)
	law := rights.DeniedBoarding(rights.Bumped{Regime: reg, KM: f.KM, Intra: rights.Intra(reg, from.Country, to.Country), Outbound: true, Fare: fare, Wait: wait})
	var owed int64
	volunteers := 0
	for _, d := range denied {
		if d.Flight.Number != f.Number || d.Class != "Y" || d.Cause == dcs.OversaleEquipment {
			t.Errorf("a denied boarding of %+v", d)
		}
		if d.Voluntary {
			volunteers++
			owed += rights.Offer(fare)
		} else {
			owed += law.Compensation + law.Care
		}
	}
	if volunteers != c.Volunteers {
		t.Errorf("%d volunteers on the carrier's count, %d at departure control", volunteers, c.Volunteers)
	}
	if fare <= 0 || owed <= 0 {
		t.Fatalf("a full fare of %d and %d owed", fare, owed)
	}
	sc := (seatWorld{s}).score(f.Carrier)
	// What the 70 who flew bought beside the seat is on the scorecard,
	// by service, and is revenue on top of the fares: this carrier is not
	// an ultra-low-cost one. Every service sold is on the carrier's own
	// list, and the units of a service sold once are no more than the
	// passengers.
	bought, units := tn.Extras()
	var extra int64
	menu := extras.For(f.Carrier)
	for code, cents := range bought {
		extra += cents
		offered := false
		for _, sv := range menu.Catalogue.Services {
			if sv.Code == code {
				offered = true
				if !sv.PerUnit && units[code] > c.Boarded {
					t.Errorf("%d of %s sold to %d passengers", units[code], code, c.Boarded)
				}
			}
		}
		if !offered || cents <= 0 || units[code] <= 0 {
			t.Errorf("sold %d cents and %d units of %s, which is not on %s's list", cents, units[code], code, f.Carrier)
		}
	}
	if extra <= 0 || sc.Ancillary != extra || len(sc.Extras) != len(bought) {
		t.Errorf("the passengers bought %d cents of extras and the scorecard has %d", extra, sc.Ancillary)
	}
	if menu.AllIn || sc.Revenue != faresOnly+extra {
		t.Errorf("revenue is %d with %d of fares and %d of extras", sc.Revenue, faresOnly, extra)
	}
	// The bags that flew are the bags that could pay.
	flown := 0
	for _, p := range fl.Passengers {
		if p.Status == dcs.StatusBoarded && p.Inbound == nil {
			flown += len(p.Bags)
		}
	}
	if units[extras.Bag] > flown {
		t.Errorf("%d bags paid a fee and %d flew with passengers who began their journey here", units[extras.Bag], flown)
	}
	if sc.Denied != c.Denied || sc.Volunteers != c.Volunteers || sc.Costs["denied boarding"] != owed {
		t.Errorf("the scorecard has %d denied, %d volunteers and %d cents; want %d, %d and %d (fare %d, wait %d minutes, %s)",
			sc.Denied, sc.Volunteers, sc.Costs["denied boarding"], c.Denied, c.Volunteers, owed, fare, wait, reg)
	}
	// The count is a day's.
	tn.NewDay()
	if len(tn.Denied()) != 0 {
		t.Error("a new day began with denied boardings")
	}
	if _, ok := tn.Carried(f, day); ok {
		t.Error("a new day began with the last day's passengers carried")
	}
	if sold, _ := tn.Extras(); len(sold) != 0 {
		t.Errorf("a new day began with the last day's extras: %v", sold)
	}

	// A flight whose counter never opened. The world started after its
	// check-in hours, say, and the door closed on a list nobody was
	// checked in from. Every booking is then a no-show at departure
	// control, and the carrier's record of who came must not take it.
	var g world.Flight
	for _, x := range s.Flights[f.Carrier] {
		if x.Number != f.Number && x.Marketing == "" && len(host.Cabins(x, s.capacity)) == 1 {
			g = x
			break
		}
	}
	if g.Number == "" {
		t.Fatal("no second flight")
	}
	other := s.rm.Cabin(rm.Key(g.Carrier, g.Number, g.From, "Y"), classes)
	var more []string
	for i := 0; i < 6; i++ {
		res, err := s.Book(ctx, g, "Y", 0, fmt.Sprintf("LATE%c", 'A'+i))
		if err != nil {
			t.Fatal(err)
		}
		more = append(more, res.PNR.RecordLocator)
	}
	for _, l := range more {
		for wait := time.Now().Add(30 * time.Second); ; {
			if ok, _ := s.Settled(ctx, l); ok {
				break
			}
			if time.Now().After(wait) {
				t.Fatalf("%s never settled", l)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	gkey := dcs.Key{Flight: g.Carrier + g.Number, Date: wire, Board: g.From}
	if err := tn.SendPNL(ctx, g, day); err != nil {
		t.Fatal(err)
	}
	for wait := time.Now().Add(30 * time.Second); ; {
		if fl, err := tn.DCS.Flight(gkey); err == nil && fl.Counts().Listed == 6 {
			break
		}
		if time.Now().After(wait) {
			t.Fatalf("departure control never listed the 6 on %s", gkey)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := tn.Close(ctx, g, day); err != nil {
		t.Fatalf("Close: %v", err)
	}
	gl, _ := tn.DCS.Flight(gkey)
	if gc := gl.Counts(); gc.NoShow != 6 || gc.Boarded != 0 {
		t.Fatalf("a flight closed with no check-in: %+v; want 6 no-shows", gc)
	}
	if got := other.Shown(); got != 0 {
		t.Errorf("the record of who came took a flight whose counter never opened: %d departures", got)
	}
	if n, ok := tn.Carried(g, day); ok {
		t.Errorf("a flight whose counter never opened is counted as flown with %d passengers", n)
	}
	// Nobody flew, so nobody bought anything.
	if sold, _ := tn.Extras(); len(sold) != 0 {
		t.Errorf("a flight that nobody boarded sold extras: %v", sold)
	}

	// A flight whose list went before most of its bookings were written.
	// 4 passengers are on the list and 8 more book after it. The 4 are
	// checked in and fly. The door's count of 4 is not what the flight
	// carried, and 4 of 4 is not the flight's show-up rate.
	var h world.Flight
	for _, x := range s.Flights[f.Carrier] {
		if x.Number != f.Number && x.Number != g.Number && x.Marketing == "" && len(host.Cabins(x, s.capacity)) == 1 {
			h = x
			break
		}
	}
	if h.Number == "" {
		t.Fatal("no third flight")
	}
	part := s.rm.Cabin(rm.Key(h.Carrier, h.Number, h.From, "Y"), classes)
	book := func(n int, name string) {
		t.Helper()
		var locs []string
		for i := 0; i < n; i++ {
			res, err := s.Book(ctx, h, "Y", 0, fmt.Sprintf("%s%c", name, 'A'+i))
			if err != nil {
				t.Fatal(err)
			}
			locs = append(locs, res.PNR.RecordLocator)
		}
		for _, l := range locs {
			for wait := time.Now().Add(30 * time.Second); ; {
				if ok, _ := s.Settled(ctx, l); ok {
					break
				}
				if time.Now().After(wait) {
					t.Fatalf("%s never settled", l)
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
	}
	book(4, "EARLY")
	hkey := dcs.Key{Flight: h.Carrier + h.Number, Date: wire, Board: h.From}
	if err := tn.SendPNL(ctx, h, day); err != nil {
		t.Fatal(err)
	}
	for wait := time.Now().Add(30 * time.Second); ; {
		if fl, err := tn.DCS.Flight(hkey); err == nil && fl.Counts().Listed == 4 {
			break
		}
		if time.Now().After(wait) {
			t.Fatalf("departure control never listed the 4 on %s", hkey)
		}
		time.Sleep(50 * time.Millisecond)
	}
	book(8, "AFTER")
	hl, _ := tn.DCS.Flight(hkey)
	for _, p := range hl.Passengers {
		if _, err := tn.DCS.Accept(ctx, hkey, dcs.AcceptRequest{PassengerID: p.ID}); err != nil {
			t.Fatal(err)
		}
		if _, err := tn.DCS.Board(ctx, hkey, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := tn.Close(ctx, h, day); err != nil {
		t.Fatalf("Close: %v", err)
	}
	hl, _ = tn.DCS.Flight(hkey)
	if hc := hl.Counts(); hc.Boarded != 4 {
		t.Fatalf("the 4 on the list: %+v", hc)
	}
	if n, ok := tn.Carried(h, day); ok {
		t.Errorf("a flight closed on a third of its bookings is counted as flown with %d passengers", n)
	}
	if got := part.Shown(); got != 0 {
		t.Errorf("the record of who came took a flight closed on a third of its bookings: %d departures", got)
	}
}

// A passenger denied boarding is paid by the fare of one direction, also
// in a class that is sold only as a round trip. 14 CFR 250.5 counts by
// "the fare to the passenger's destination or first stopover". For a
// round trip that is the half that was being flown, which is what a
// round-trip fare's amount is here: 1 fare component.
//
// The fare was once looked up among the one-way fares alone. A class
// with none gave $0, so a volunteer in it was offered nothing, and a
// passenger turned away was owed a meal and no compensation.
func TestADeniedBoardingOnARoundTripFareIsPaidByItsHalf(t *testing.T) {
	m := &world.Manifest{
		Airports: []world.Airport{
			{IATA: "JFK", Country: "United States", Lat: 40.64, Lon: -73.78, TZ: "America/New_York"},
			{IATA: "LHR", Country: "United Kingdom", Lat: 51.47, Lon: -0.45, TZ: "Europe/London"},
		},
		Carriers: []world.Carrier{{Designator: "AA", Country: "United States", Hub: "JFK"}},
		Flights: []world.Flight{
			{Carrier: "AA", Number: "0100", From: "JFK", To: "LHR", DepMin: 1080, ArrMin: 1500, KM: 5540},
			{Carrier: "AA", Number: "0102", From: "JFK", To: "LHR", DepMin: 1260, ArrMin: 1680, KM: 5540},
		},
	}
	s := &Sim{tariff: tariff.FromManifest(m), Flights: map[string][]world.Flight{"AA": m.Flights},
		carriers: map[string]world.Carrier{"AA": m.Carriers[0]},
		airports: map[string]world.Airport{"JFK": m.Airports[0], "LHR": m.Airports[1]}}
	f := m.Flights[0]

	// A class with a round-trip fare and no one-way fare, and one sold
	// one way.
	oneWay, half := map[string]int64{}, map[string]int64{}
	for _, x := range s.tariff.Fares("AA", "JFK", "LHR") {
		if len(x.Rule.Sale.Accounts) > 0 {
			continue
		}
		if x.RoundTrip {
			half[x.Class] = max(half[x.Class], x.OneWay.Amount)
		} else {
			oneWay[x.Class] = max(oneWay[x.Class], x.OneWay.Amount)
		}
	}
	var class string
	for c, amount := range half {
		if oneWay[c] == 0 && amount > 0 && 2*amount < 107500 && (class == "" || c < class) {
			class = c
		}
	}
	if class == "" || oneWay["Y"] == 0 {
		t.Fatalf("no class of AA JFK-LHR is sold only as a round trip under the limit, or Y has no one-way fare: round trip %v, one way %v", half, oneWay)
	}

	// A volunteer has the carrier's offer: twice the fare of the half.
	if got, want := s.owedDenied(host.Denied{Flight: f, Class: class, Voluntary: true}), 2*half[class]; got != want {
		t.Errorf("a volunteer in class %s, sold only as a round trip at %d a direction, is offered %d; want %d", class, half[class], got, want)
	}
	// A passenger turned away, with the next flight 3 hours later: twice
	// the fare, and a meal.
	law := rights.DeniedBoarding(rights.Bumped{Regime: rights.US, KM: f.KM, Outbound: true, Fare: half[class], Wait: 180})
	if law.Compensation != 2*half[class] || law.Care == 0 {
		t.Fatalf("the law gives %+v for a fare of %d and 3 hours; the test wants twice the fare and a meal", law, half[class])
	}
	if got := s.owedDenied(host.Denied{Flight: f, Class: class, Cause: dcs.OversaleSold}); got != law.Compensation+law.Care {
		t.Errorf("a passenger turned away in class %s is owed %d; want %d of compensation and %d of care", class, got, law.Compensation, law.Care)
	}
	// A class sold one way is paid by its one-way fare, as before.
	if got, want := s.owedDenied(host.Denied{Flight: f, Class: "Y", Voluntary: true}), min(2*oneWay["Y"], 107500); got != want {
		t.Errorf("a volunteer in Y at %d is offered %d; want %d", oneWay["Y"], got, want)
	}
}

// Families with an infant on a lap, from the booking to the count of
// who flew. 6 parties of 2 adults and an infant book 2 seats each. The
// airport lists 18 names, 6 of them infants, and seats the 12 adults.
// The door counts everyone on board. The carrier counts the passengers
// it carried in seats, and revenue management's count of the cabin has
// the 12 bookings: an infant is in neither. Nobody is denied boarding.
func TestAnInfantOnALapFromBookingToTheCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	var f world.Flight
	for _, c := range m.Carriers {
		for _, g := range s.Flights[c.Designator] {
			if cabins := host.Cabins(g, s.capacity); len(cabins) == 1 && g.Marketing == "" && g.DepMin > 8*60 && g.DepMin < 20*60 && cabins["Y"] >= 60 && f.Number == "" {
				f = g
			}
		}
	}
	if f.Number == "" {
		t.Fatal("no flight with one cabin")
	}
	tn, day := s.Tenants[f.Carrier], s.BookingDate
	wire := strings.ToUpper(day.Format("02Jan"))
	s.clock.Sync(time.Now(), float64(f.DepMin-200), s.clock.Warp())
	// The records are in the carrier's book as the day's fill writes
	// them: 3 names, 2 seats, and the infant named in a service request.
	for i := 0; i < 6; i++ {
		surname := fmt.Sprintf("FAMILY%c", 'A'+i)
		rec := &pnr.PNR{RecordLocator: fmt.Sprintf("FAM%c%02d", 'A'+i, i), Status: pnr.StatusTicketed,
			Passengers: []pnr.Passenger{
				{Ref: 1, Surname: surname, Given: "PA", Title: "MR", Type: pnr.PaxAdult},
				{Ref: 2, Surname: surname, Given: "MA", Title: "MRS", Type: pnr.PaxAdult},
				{Ref: 3, Surname: surname, Given: "BABY", Title: "INF", Type: pnr.PaxInfant, Infant: true}},
			Segments: []pnr.Segment{{Ref: 1, Type: pnr.SegmentAir, Carrier: f.Carrier, FlightNum: f.Number, Class: "Y", Depart: day, WireDate: wire,
				DepartTime: hhmmDigits(f.DepMin), Board: f.From, Off: f.To, Status: "HK", Seats: 2}},
			SSRs: []pnr.SSR{{Code: "INFT", Carrier: f.Carrier, Status: "HK", Count: 1, PaxRef: 3, Text: surname + "/BABY"}},
		}
		if err := tn.Store.CreatePNR(ctx, rec, nil); err != nil {
			t.Fatal(err)
		}
		tn.Inventory.Seed(rec.Segments[0])
	}
	sold := 0
	for _, n := range tn.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, "Y") {
		sold += n
	}
	if sold != 12 {
		t.Fatalf("6 parties of 2 adults and an infant hold %d seats; want 12", sold)
	}

	key := dcs.Key{Flight: f.Carrier + f.Number, Date: wire, Board: f.From}
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	for wait := time.Now().Add(30 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if fl, err := tn.DCS.Flight(key); err == nil && fl.Counts().Listed == 18 {
			break
		}
		if time.Now().After(wait) {
			fl, err := tn.DCS.Flight(key)
			if err != nil {
				t.Fatalf("departure control never opened the flight: %v", err)
			}
			t.Fatalf("departure control lists %d names; want 18", fl.Counts().Listed)
		}
	}
	for _, out := range []int{150, 120, 90, 60, 46} {
		if err := tn.CheckIn(ctx, f, day, out); err != nil {
			t.Fatalf("CheckIn at %d: %v", out, err)
		}
	}
	if err := tn.CloseCheckIn(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	for _, out := range []int{30, 20, 14} {
		if err := tn.Board(ctx, f, day, out); err != nil {
			t.Fatal(err)
		}
	}
	if err := tn.Close(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	fl, err := tn.DCS.Flight(key)
	if err != nil {
		t.Fatal(err)
	}
	infants, onLaps, seated := 0, 0, 0
	for _, p := range fl.Passengers {
		if p.Type == dcs.PaxInfant {
			infants++
			if p.Seat != "" {
				t.Errorf("the infant %s has seat %s", p.Display(), p.Seat)
			}
			if p.Status == dcs.StatusBoarded {
				onLaps++
			}
		} else if p.Status == dcs.StatusBoarded {
			seated++
		}
	}
	c := fl.Counts()
	if infants != 6 || onLaps == 0 || c.Boarded != seated+onLaps {
		t.Fatalf("the airport has %d infants, %d of them on board, and the door counts %d on board with %d in seats; want 6 infants, some on board", infants, onLaps, c.Boarded, seated)
	}
	if n, ok := tn.Carried(f, day); !ok || n != seated {
		t.Errorf("the carrier counts %d carried (%v); want the %d passengers in seats, and not the %d infants on laps", n, ok, seated, onLaps)
	}
	y := fl.Tally()["Y"]
	if y.Booked != 12 || y.Boarded != seated {
		t.Errorf("revenue management's count of the cabin: %+v; want 12 booked and %d boarded", y, seated)
	}
	if c.Denied != 0 || len(tn.Denied()) != 0 {
		t.Errorf("%d denied boarding at the door and %d on the carrier's list; want none", c.Denied, len(tn.Denied()))
	}
	t.Logf("%s%s: 18 names, %d infants; %d on board, %d of them in seats", f.Carrier, f.Number, infants, c.Boarded, seated)
}

// hhmmDigits is a minute of the day as a record's segment has it: 0730.
func hhmmDigits(minute int) string { return fmt.Sprintf("%02d%02d", minute/60%24, minute%60) }
