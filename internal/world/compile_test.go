package world

import (
	"github.com/adamf/wholesky/internal/equipment"
	"sort"
	"testing"
)

// Every leg belongs to a rotation, and rotations are physically coherent: a
// tail departs where it last arrived, after its turnaround. This is what
// makes the movement stream read like a fleet instead of teleporting
// registrations.
func TestRotationsAreCoherent(t *testing.T) {
	m, err := Compile(CompileOptions{
		DataDir: "../../data", Seed: 1,
		Countries: []string{"United Kingdom", "France"}, MaxCarriers: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	byTail := map[string][]Flight{}
	for _, f := range m.Flights {
		if f.Tail == "" {
			t.Fatalf("flight %s%s has no tail", f.Carrier, f.Number)
		}
		byTail[f.Tail] = append(byTail[f.Tail], f)
	}
	if len(byTail) == 0 {
		t.Fatal("no rotations at all")
	}
	multi := 0
	turns := map[string][]int{}
	for tail, legs := range byTail {
		sort.Slice(legs, func(a, b int) bool { return legs[a].DepMin < legs[b].DepMin })
		if len(legs) > 1 {
			multi++
		}
		for i := 1; i < len(legs); i++ {
			prev, next := legs[i-1], legs[i]
			if next.From != prev.To {
				t.Errorf("tail %s teleports: %s%s lands %s, %s%s departs %s",
					tail, prev.Carrier, prev.Number, prev.To, next.Carrier, next.Number, next.From)
			}
			// An aircraft is one type all day, and it is given the ground
			// time its type needs for the way its carrier works.
			if next.Equipment != prev.Equipment {
				t.Errorf("tail %s is a %s on %s%s and a %s on %s%s", tail, prev.Equipment, prev.Carrier, prev.Number, next.Equipment, next.Carrier, next.Number)
			}
			floor := ScheduledTurn(next.Equipment, next.Seats, equipment.ModelOf(next.Carrier), max(prev.KM, next.KM))
			if next.DepMin < prev.ArrMin+floor {
				t.Errorf("tail %s turns in %d minutes between %s%s and %s%s; a %s of %s is scheduled %d",
					tail, next.DepMin-prev.ArrMin, prev.Carrier, prev.Number, next.Carrier, next.Number, next.Equipment, next.Carrier, floor)
			}
			turns[next.Carrier] = append(turns[next.Carrier], floor)
		}
	}
	if multi == 0 {
		t.Error("every tail flies exactly one leg; the chaining never chained anything")
	}
	// easyJet's standard turn is 25 minutes and the schedule gives it 30;
	// a network carrier's narrow-body is given 45 and more.
	low, net := 1000, 0
	for carrier, ts := range turns {
		for _, v := range ts {
			if equipment.ModelOf(carrier) != equipment.Network {
				low = min(low, v)
			} else {
				net = max(net, v)
			}
		}
	}
	if low > 30 || net < 45 {
		t.Errorf("shortest low-cost turn %d, longest network turn %d", low, net)
	}
	t.Logf("%d flights on %d tails, %d rotations of 2+ legs", len(m.Flights), len(byTail), multi)
}

// The full world flies about what the real one does: the commercial fleet
// runs roughly a hundred thousand legs a day, and the frequency tiers are
// calibrated to land there. Drift outside the band means somebody retuned
// the tiers without meaning to.
func TestFullWorldFliesARealDay(t *testing.T) {
	m, err := Compile(CompileOptions{DataDir: "../../data", Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(m.Flights); n < 95000 || n > 115000 {
		t.Errorf("the full world flies %d legs a day; the real one flies about 105000", n)
	}
	if len(m.Carriers) < 400 {
		t.Errorf("only %d carriers survived compilation", len(m.Carriers))
	}
}
