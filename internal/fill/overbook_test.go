package fill

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// About 1 booking in 10 does not come for the flight. A carrier that
// sells each cabin to its seat count flies those seats empty on its full
// flights. A carrier that overbooks keeps a record of who came for each
// flight and lets the main cabin hold more bookings than seats, by the
// limit that costs least in denied boardings and empty seats together.
//
// One network carrier's 900 flights are sold for 38 days of busy days, 3
// ways: with no overbooking, with a denied boarding counted at 2 average
// fares (what the carrier offers a volunteer), and with it counted at 20
// (a carrier that will not turn a passenger away). The travellers, and
// who of them comes, are drawn the same way in all 3. The last 8 days
// are compared.
//
// The rate of denied boardings is held against the airlines' own over
// the whole world, in TestFillSellsTheReportsFares. A network carrier is
// under that rate: it seats an oversold main cabin in the empty seats in
// front.
func TestOverbookingFillsTheSeatsNoShowsLeave(t *testing.T) {
	if raceEnabled {
		t.Skip("38 days of a carrier's booking windows, sold three ways: CI runs it without the race detector")
	}
	m := usWorld(t)
	one := &world.Manifest{Carriers: m.Carriers, Airports: m.Airports}
	for _, f := range m.Flights {
		if f.Carrier == "UA" {
			one.Flights = append(one.Flights, f)
		}
	}
	tf := tariff.FromManifest(m)
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	const days, last = 38, 8
	// A busy day has a quarter more travellers than an ordinary one.
	const busy = 1.25
	run := func(overbooks bool, deniedCost float64) (sum Plan, first Plan) {
		mem := rm.New()
		for d := 0; d < days; d++ {
			plan, err := Day(context.Background(), one, Options{LoadFactor: 0.82, Seed: 3 + int64(d)*7919, Day: day, Tariff: tf, Base: tf.BaseFare,
				Cabins:    func(f world.Flight) map[string]int { return host.Cabins(f, 0) },
				Overbooks: func(string) bool { return overbooks }, DeniedCost: deniedCost,
				Share: func(world.Flight) float64 { return busy },
				RM:    mem, Dry: true}, nil)
			if err != nil {
				t.Error(err)
				return
			}
			if d == rm.Warm-1 {
				first = plan
			}
			if d >= days-last {
				sum.Seats += plan.Seats
				sum.Sold += plan.Sold
				sum.Fares += plan.Fares
				sum.NoShows += plan.NoShows
				sum.Boarded += plan.Boarded
				sum.Denied += plan.Denied
				sum.Spoiled += plan.Spoiled
				sum.Over += plan.Over
			}
		}
		return sum, first
	}
	var none, usual, wary, early Plan
	var wg sync.WaitGroup
	for _, arm := range []func(){
		func() { none, _ = run(false, 0) },
		func() { usual, early = run(true, 0) },
		func() { wary, _ = run(true, 20) },
	} {
		wg.Add(1)
		go func() { defer wg.Done(); arm() }()
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	in10k := func(p Plan) float64 { return 1e4 * float64(p.Denied) / float64(p.Boarded) }
	line := func(name string, p Plan) {
		t.Logf("%s: booked %.3f of seats, flew %.3f, %.3f of bookings did not come, %.2f in 10,000 denied boarding, %.4f of seats spoiled, limits %.3f over the seats, $%.0f a day",
			name, float64(p.Sold)/float64(p.Seats), float64(p.Boarded)/float64(p.Seats), float64(p.NoShows)/float64(p.Sold), in10k(p),
			float64(p.Spoiled)/float64(p.Seats), float64(p.Over)/float64(p.Seats), float64(p.Fares)/100/last)
	}
	line("no overbooking", none)
	line("a denied boarding at 2 fares", usual)
	line("a denied boarding at 20 fares", wary)

	// Until a flight has its record, no cabin is sold past its seats.
	if early.Over != 0 || early.Denied != 0 {
		t.Errorf("on day %d, before any record, the limits were %d over the seats and %d passengers were denied boarding", rm.Warm, early.Over, early.Denied)
	}
	if none.Over != 0 || none.Denied != 0 {
		t.Errorf("with no overbooking the limits were %d over the seats and %d passengers were denied boarding", none.Over, none.Denied)
	}
	// Every booking is one of: flew, did not come, was denied boarding.
	for name, p := range map[string]Plan{"no overbooking": none, "2 fares": usual, "20 fares": wary} {
		if p.Boarded+p.NoShows+p.Denied != p.Sold {
			t.Errorf("%s: %d booked, and %d flew, %d did not come and %d were denied", name, p.Sold, p.Boarded, p.NoShows, p.Denied)
		}
	}
	// The carrier that overbooks flies more passengers in the same seats.
	if got, was := float64(usual.Boarded), float64(none.Boarded); got < 1.05*was {
		t.Errorf("overbooking flew %.0f passengers against %.0f without: at least 5%% more is expected", got, was)
	}
	// It spoils far fewer seats: a no-show's seat on a full flight was
	// sold twice.
	if usual.Spoiled*2 > none.Spoiled {
		t.Errorf("overbooking spoiled %d seats against %d without: under half is expected", usual.Spoiled, none.Spoiled)
	}
	// It turns some passengers away, and no more than airlines do: 4.4
	// in 10,000 in 2018 and 10 in 2012 (GAO-20-191).
	if r := in10k(usual); usual.Denied == 0 || r > 12 {
		t.Errorf("%.2f passengers in 10,000 were denied boarding: some are expected, and airlines report 4.4 to 10", r)
	}
	// It earns more, after it has paid each passenger it turned away
	// twice the average fare.
	paid := 2 * float64(usual.Fares) / float64(usual.Sold) * float64(usual.Denied)
	if net, was := float64(usual.Fares)-paid, float64(none.Fares); net < 1.01*was {
		t.Errorf("overbooking earned %.0f after %.0f of compensation, against %.0f without", net/100/last, paid/100/last, was/100/last)
	}
	// A carrier that counts a denied boarding dearer overbooks less,
	// turns fewer away, and spoils more seats.
	if !(wary.Over < usual.Over && wary.Over > 0) {
		t.Errorf("limits over the seats: %d at 20 fares, %d at 2", wary.Over, usual.Over)
	}
	if wary.Denied*4 > usual.Denied {
		t.Errorf("denied boardings: %d at 20 fares, %d at 2: under a quarter is expected", wary.Denied, usual.Denied)
	}
	if !(wary.Spoiled > usual.Spoiled && wary.Spoiled < none.Spoiled) {
		t.Errorf("spoiled seats: %d at 20 fares, %d at 2, %d with no overbooking", wary.Spoiled, usual.Spoiled, none.Spoiled)
	}
}

// A flight overbooks only when its own booking record sets its class
// limits. Every cabin of a carrier is given 7 departures of who came, and
// no booking record. The first day is then sold under the tariff's fixed
// limits, and no cabin is authorised past its seats.
func TestACabinWithNoBookingRecordIsNotOverbooked(t *testing.T) {
	m := usWorld(t)
	one := &world.Manifest{Carriers: m.Carriers, Airports: m.Airports}
	for _, f := range m.Flights {
		if f.Carrier == "UA" && len(one.Flights) < 200 {
			one.Flights = append(one.Flights, f)
		}
	}
	tf := tariff.FromManifest(m)
	mem := rm.New()
	for _, f := range one.Flights {
		c := mem.Cabin(rm.Key(f.Carrier, f.Number, f.From, "Y"), []string{"Y"})
		for d := 0; d < rm.Warm; d++ {
			c.Show(100, 10)
		}
	}
	plan, err := Day(context.Background(), one, Options{LoadFactor: 0.82, Seed: 3, Day: time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC), Tariff: tf, Base: tf.BaseFare,
		Cabins: func(f world.Flight) map[string]int { return host.Cabins(f, 0) },
		RM:     mem, Dry: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Sold == 0 {
		t.Fatal("nothing was sold")
	}
	if plan.Over != 0 || plan.Denied != 0 {
		t.Errorf("with no booking record the cabins were authorised %d over their seats and %d passengers were denied boarding", plan.Over, plan.Denied)
	}
	// The record of who came was kept when the booking record began.
	f := one.Flights[0]
	if c := mem.Peek(rm.Key(f.Carrier, f.Number, f.From, "Y")); c == nil || c.Shown() != rm.Warm+1 || c.Departures() != 1 {
		t.Errorf("after the day the first flight's cabin has %d departures of who came and %d of bookings; want %d and 1", c.Shown(), c.Departures(), rm.Warm+1)
	}
}
