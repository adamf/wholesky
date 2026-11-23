package host

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/inventory"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// The programme over one connecting point: a wide-open feeder into a
// nearly-full trunk. The trunk's capacity binds, so its last seat carries
// a price; the feeder has room for everyone and prices at nothing.
func TestNetworkProgrammePricesTheBindingLeg(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	m := &world.Manifest{
		Carriers: []world.Carrier{{Designator: "BA", Hub: "LHR", TTYAddress: "LONRMBA"}},
		Flights: []world.Flight{
			{Carrier: "BA", Number: "1390", From: "MAN", To: "LHR", DepMin: 700, ArrMin: 760, KM: 250},
			{Carrier: "BA", Number: "0117", From: "LHR", To: "JFK", DepMin: 800, ArrMin: 1300, KM: 5550},
		},
	}
	cap := func(carrier, flightNum, wireDate, board string) (map[string]int, bool) {
		switch board {
		case "MAN":
			return map[string]int{"Y": 150}, true
		case "LHR":
			return map[string]int{"Y": 6}, true
		}
		return nil, false
	}
	st := store.NewMem()
	for i := 0; i < 30; i++ {
		rec := &pnr.PNR{RecordLocator: "CNX" + string(rune('A'+i%26)) + string(rune('A'+i/26)), Status: pnr.StatusTicketed,
			Passengers: []pnr.Passenger{{Ref: 1, Surname: "THRU", Given: "A"}},
			Segments: []pnr.Segment{
				{Ref: 1, Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "1390", WireDate: "26NOV", Board: "MAN", Off: "LHR", Class: "M", Status: "HK", Seats: 1},
				{Ref: 2, Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "0117", WireDate: "26NOV", Board: "LHR", Off: "JFK", Class: "M", Status: "HK", Seats: 1}},
			Pricing: &pnr.Pricing{Currency: "USD", Base: 20000, Total: 20000, Passengers: []pnr.PassengerPricing{{Ref: 1, Segments: []int64{5000, 15000}}}}}
		if err := st.CreatePNR(ctx, rec, nil); err != nil {
			t.Fatal(err)
		}
	}
	tn := &Tenant{Carrier: m.Carriers[0], Store: st, Inventory: inventory.New("BA", cap), flights: m.Flights, capacity: cap,
		tariff: tariff.FromManifest(m), dayPos: func() float64 { return 600 }, bookingDate: day, net: &networkRM{},
		log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := tn.solveNetwork(ctx); err != nil {
		t.Fatal(err)
	}
	ns := tn.Network()
	if ns.Hubs != 1 || ns.Legs != 2 || ns.Products < 3 {
		t.Fatalf("status %+v", ns)
	}
	trunk, ok := tn.BidPrice("BA", "0117", "26NOV", "LHR", "Y")
	if !ok || trunk <= 0 {
		t.Errorf("the six-seat trunk's last seat is worth something: %v %v", trunk, ok)
	}
	feeder, ok := tn.BidPrice("BA", "1390", "26NOV", "MAN", "Y")
	if !ok || feeder != 0 {
		t.Errorf("the empty feeder prices at nothing: %v %v", feeder, ok)
	}
	if _, ok := tn.BidPrice("BA", "0117", "26NOV", "LHR", "C"); ok {
		t.Error("a cabin the programme did not price falls back to the ladder")
	}
	// A leg outside the window is left to its ladder.
	tn.dayPos = func() float64 { return 100 }
	if err := tn.solveNetwork(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := tn.BidPrice("BA", "0117", "26NOV", "LHR", "Y"); ok {
		t.Error("a leg departing outside the window is not priced")
	}
}

// The programme counts the bookings a cabin may hold, and not its seats.
// A trunk of 6 seats has sold 6, and the carrier overbooks it to 8. It
// has room for 2 more bookings, so its last seat is worth what the
// second-best passenger still to come would pay. Counted by its seats
// the cabin was full: the programme then priced it at the dearest fare
// on the leg, and a connecting passenger had to beat that fare to get a
// seat the carrier wanted to sell. With all 8 sold, the cabin is full
// and the price is the dearest fare again.
func TestNetworkProgrammeCountsBookingsACabinMayHold(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	m := &world.Manifest{
		Carriers: []world.Carrier{{Designator: "BA", Hub: "LHR", TTYAddress: "LONRMBA"}},
		Flights: []world.Flight{
			{Carrier: "BA", Number: "1390", From: "MAN", To: "LHR", DepMin: 700, ArrMin: 760, KM: 250},
			{Carrier: "BA", Number: "0117", From: "LHR", To: "JFK", DepMin: 800, ArrMin: 1300, KM: 5550},
		},
	}
	cap := func(carrier, flightNum, wireDate, board string) (map[string]int, bool) {
		switch board {
		case "MAN":
			return map[string]int{"Y": 150}, true
		case "LHR":
			return map[string]int{"Y": 6}, true
		}
		return nil, false
	}
	// bid is the trunk's price with a booking limit and a number sold.
	bid := func(limit, sold int) float64 {
		t.Helper()
		st := store.NewMem()
		for i := 0; i < 30; i++ {
			rec := &pnr.PNR{RecordLocator: "CNX" + string(rune('A'+i%26)) + string(rune('A'+i/26)), Status: pnr.StatusTicketed,
				Passengers: []pnr.Passenger{{Ref: 1, Surname: "THRU", Given: "A"}},
				Segments: []pnr.Segment{
					{Ref: 1, Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "1390", WireDate: "26NOV", Board: "MAN", Off: "LHR", Class: "M", Status: "HK", Seats: 1},
					{Ref: 2, Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "0117", WireDate: "26NOV", Board: "LHR", Off: "JFK", Class: "M", Status: "HK", Seats: 1}},
				Pricing: &pnr.Pricing{Currency: "USD", Base: 20000, Total: 20000, Passengers: []pnr.PassengerPricing{{Ref: 1, Segments: []int64{5000, 15000}}}}}
			if err := st.CreatePNR(ctx, rec, nil); err != nil {
				t.Fatal(err)
			}
		}
		inv := inventory.New("BA", cap)
		if limit > 0 {
			inv.Authorized = func(carrier, flightNum, wireDate, board, compartment string, seats int) int {
				if board == "LHR" && compartment == "Y" {
					return limit
				}
				return seats
			}
		}
		inv.Seed(pnr.Segment{Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "0117", WireDate: "26NOV", Board: "LHR", Class: "M", Status: "HK", Seats: sold})
		tn := &Tenant{Carrier: m.Carriers[0], Store: st, Inventory: inv, flights: m.Flights, capacity: cap,
			tariff: tariff.FromManifest(m), dayPos: func() float64 { return 600 }, bookingDate: day, net: &networkRM{},
			log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		if err := tn.solveNetwork(ctx); err != nil {
			t.Fatal(err)
		}
		b, ok := tn.BidPrice("BA", "0117", "26NOV", "LHR", "Y")
		if !ok {
			t.Fatalf("the trunk is not priced with a limit of %d and %d sold", limit, sold)
		}
		return b
	}
	dearest := float64(tariff.FullFare(tariff.FromManifest(m), "BA", "LHR", "JFK"))
	full := bid(0, 6)
	if full < dearest {
		t.Fatalf("a cabin of 6 seats with 6 sold is priced at %.0f; want the dearest fare on the leg, %.0f, or more", full, dearest)
	}
	room := bid(8, 6)
	if room >= dearest {
		t.Errorf("a cabin that may hold 8 bookings and has sold 6 is priced at %.0f, the dearest fare or more (%.0f): the programme counted its 6 seats", room, dearest)
	}
	if room <= 0 {
		t.Errorf("a cabin with room for 2 more bookings and more passengers to come is priced at %.0f; want something", room)
	}
	if sold := bid(8, 8); sold < dearest {
		t.Errorf("a cabin that may hold 8 bookings and has sold 8 is priced at %.0f; want the dearest fare, %.0f, or more", sold, dearest)
	}
	t.Logf("the dearest fare is %.0f; full by its seats %.0f; overbooked to 8 with 6 sold %.0f", dearest, full, room)
}
