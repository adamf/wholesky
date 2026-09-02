package fill

import (
	"context"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/world"
)

func smallDay() *world.Manifest {
	fl := func(c, n, from, to string, dep, arr, seats int) world.Flight {
		return world.Flight{Carrier: c, Number: n, From: from, To: to, DepMin: dep, ArrMin: arr, BlockMin: arr - dep, Seats: seats, Equipment: "73H"}
	}
	return &world.Manifest{Flights: []world.Flight{
		fl("WN", "0100", "BNA", "MDW", 6*60, 8*60, 174),
		fl("WN", "0200", "MDW", "DEN", 9*60+30, 12*60, 174), // connects from 0100
		fl("WN", "0300", "MDW", "BWI", 8*60+15, 10*60, 174), // too tight to connect from 0100
		fl("WN", "0400", "DEN", "BNA", 13*60, 16*60, 143),
		fl("DL", "0500", "ATL", "JFK", 7*60, 9*60+30, 160),
		fl("OO", "3991", "SFO", "SEA", 10*60, 12*60, 76),
	}}
}

// A fill puts about the load factor on every flight, connects onto legs that
// actually connect, and writes records a store can read back by flight and
// by the channel's locator; the same seed fills the same day.
func TestFillLoadsTheDay(t *testing.T) {
	ctx := context.Background()
	m := smallDay()
	m.Flights[5].Marketing, m.Flights[5].MarketingNumber = "DL", "3991"
	day := time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)
	stores := map[string]*store.Mem{}
	sink := func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
		st, ok := stores[carrier]
		if !ok {
			st = store.NewMem()
			stores[carrier] = st
		}
		return st.LoadPNRs(ctx, recs, "fill")
	}
	plan, err := Day(ctx, m, Options{LoadFactor: 0.85, Seed: 7, Day: day}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Carriers != 3 || plan.Flights != 6 || plan.Records == 0 {
		t.Fatalf("plan: %+v", plan)
	}
	// Seats per flight near the target, never over the aircraft.
	for _, f := range m.Flights {
		recs, err := stores[f.Carrier].FindPNRsByFlight(ctx, f.Carrier+f.Number, "26NOV", 10000)
		if err != nil {
			t.Fatal(err)
		}
		seats := 0
		for _, r := range recs {
			for _, sg := range r.Segments {
				if sg.FlightNum == f.Number && sg.Board == f.From {
					seats += sg.Seats
				}
			}
		}
		lf := float64(seats) / float64(f.Seats)
		if lf < 0.55 || lf > 0.98 {
			t.Errorf("%s%s %s: %d of %d seats (%.2f) is not a holiday load", f.Carrier, f.Number, f.From, seats, f.Seats, lf)
		}
	}
	// Connections ride legs that connect: 0100 into MDW then 0200 out, never 0300.
	conn, bad := 0, 0
	all, _ := stores["WN"].FindPNRsByFlight(ctx, "WN0100", "26NOV", 10000)
	for _, r := range all {
		if len(r.Segments) == 2 {
			conn++
			if r.Segments[1].FlightNum != "0200" {
				bad++
			}
		}
	}
	if conn == 0 || bad > 0 {
		t.Errorf("connections off WN0100: %d, of which %d onto a leg that does not connect", conn, bad)
	}
	// A record reads as one: names, class, ticket per name, a channel locator.
	r := all[0]
	if len(r.Tickets) != len(r.Passengers) || r.Status != pnr.StatusTicketed || r.Segments[0].WireDate != "26NOV" {
		t.Errorf("record shape: %+v", r)
	}
	channelled := 0
	for _, r := range all {
		if len(r.Locators) == 1 {
			channelled++
			if got, err := stores["WN"].FindPNRByExternalLocator(ctx, r.Locators[0].Owner, r.Locators[0].Value); err != nil || got.RecordLocator != r.RecordLocator {
				t.Errorf("channel locator %v does not find its record: %v", r.Locators[0], err)
			}
		}
	}
	if channelled == 0 || channelled == len(all) {
		t.Errorf("%d of %d records sold through a channel; expected a mix", channelled, len(all))
	}
	// The codeshare is on the record as a remark, under the operating flight.
	oo, _ := stores["OO"].FindPNRsByFlight(ctx, "OO3991", "26NOV", 10000)
	remarked := 0
	for _, r := range oo {
		if len(r.Remarks) > 0 {
			remarked++
		}
	}
	if len(oo) == 0 || remarked == 0 {
		t.Errorf("codeshare remarks on OO3991: %d of %d", remarked, len(oo))
	}
	// Deterministic: fill again, same locators in the same order.
	var first, second []string
	collect := func(out *[]string) Sink {
		return func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
			for _, r := range recs {
				*out = append(*out, carrier+":"+r.RecordLocator)
			}
			return nil
		}
	}
	if _, err := Day(ctx, m, Options{LoadFactor: 0.85, Seed: 7, Day: day}, collect(&first)); err != nil {
		t.Fatal(err)
	}
	if _, err := Day(ctx, m, Options{LoadFactor: 0.85, Seed: 7, Day: day}, collect(&second)); err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) || first[3] != second[3] || first[len(first)-1] != second[len(second)-1] {
		t.Errorf("the same seed should fill the same day: %d vs %d records", len(first), len(second))
	}
}

// One party in a few thousand travels with a cello, and the cello has a
// seat: a name of its own on the record, the CBBG request that tells the
// airport what is in it, and a seat counted against the aircraft like any
// other. Asked for on LinkedIn, and yes, it does.
func TestACelloGetsASeat(t *testing.T) {
	ctx := context.Background()
	m := smallDay()
	day := time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)
	var withCello []*pnr.PNR
	sink := func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
		for _, r := range recs {
			for _, p := range r.Passengers {
				if p.Given == "CBBG" {
					withCello = append(withCello, r)
				}
			}
		}
		return nil
	}
	if _, err := Day(ctx, m, Options{LoadFactor: 0.8, Seed: 11, Day: day, Cellos: 0.5}, sink); err != nil {
		t.Fatal(err)
	}
	if len(withCello) == 0 {
		t.Fatal("half the parties were told to bring a cello and none did")
	}
	r := withCello[0]
	cbbg := 0
	for _, s := range r.SSRs {
		if s.Code == "CBBG" && s.Text == "CELLO" {
			cbbg++
		}
	}
	if cbbg != 1 {
		t.Errorf("a cello wants exactly one CBBG: %+v", r.SSRs)
	}
	seated := 0
	for _, p := range r.Passengers {
		if !p.Infant {
			seated++
		}
	}
	if r.Segments[0].Seats != seated || seated < 2 {
		t.Errorf("the cello's seat should be counted: %d seats for %d seated names", r.Segments[0].Seats, seated)
	}
	if len(r.Tickets) != len(r.Passengers) {
		t.Errorf("the cello is ticketed like everyone else: %d tickets, %d names", len(r.Tickets), len(r.Passengers))
	}
}
