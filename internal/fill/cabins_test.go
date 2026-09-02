package fill

import (
	"context"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/inventory"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/wholesky/internal/world"
)

// A party is put in a cabin with room for it. Before this the class was
// drawn without looking at the aircraft, and the recorded day's Hawaii
// widebodies held fifty business passengers in thirty-two seats, which the
// live invariant check caught.
func TestFillRespectsEachCabin(t *testing.T) {
	m := &world.Manifest{Flights: []world.Flight{
		{Carrier: "HA", Number: "0011", From: "HNL", To: "LAX", DepMin: 8 * 60, ArrMin: 16 * 60, BlockMin: 8 * 60, Seats: 100, Equipment: "332"},
		{Carrier: "HA", Number: "0012", From: "LAX", To: "HNL", DepMin: 18 * 60, ArrMin: 24 * 60, BlockMin: 6 * 60, Seats: 100, Equipment: "332"},
	}}
	cabins := map[string]int{"C": 4, "Y": 96}
	sold := map[string]map[string]int{}
	sink := func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
		for _, r := range recs {
			for _, s := range r.Segments {
				key := s.FlightNum + "/" + s.Board
				if sold[key] == nil {
					sold[key] = map[string]int{}
				}
				sold[key][inventory.CompartmentFor(s.Class, cabins)] += s.Seats
			}
		}
		return nil
	}
	for seed := int64(1); seed <= 5; seed++ {
		for k := range sold {
			delete(sold, k)
		}
		_, err := Day(context.Background(), m, Options{LoadFactor: 0.98, Seed: seed, Day: time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC),
			Cabins: func(world.Flight) map[string]int { return cabins }}, sink)
		if err != nil {
			t.Fatal(err)
		}
		for key, comps := range sold {
			for comp, n := range comps {
				if n > cabins[comp] {
					t.Errorf("seed %d: %s cabin %s holds %d of %d", seed, key, comp, n, cabins[comp])
				}
			}
			if comps["Y"] < 80 {
				t.Errorf("seed %d: %s economy should still be nearly full at 0.98, holds %d", seed, key, comps["Y"])
			}
		}
	}
}
