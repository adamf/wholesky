package equipment

import (
	"math"
	"testing"

	"github.com/adamf/wholesky/internal/refdata"
)

// The turnaround model against the manufacturers' own charts and the
// carriers' own standards.
func TestTurnaroundsAgainstTheCharts(t *testing.T) {
	cases := []struct {
		what      string
		g         Ground
		low, high float64
		path      string
	}{
		// Airbus: A320, 150 passengers, one bridge, full servicing: 44 minutes.
		{"A320 full service", Ground{Type: "320", Seats: 150, Off: 150, On: 150, NextKM: 1800, Model: Network}, 36, 46, "passengers"},
		// Airbus: A320, 180 passengers, two stairs, no fuel: 22 minutes. Ryanair's standard is 25.
		{"A320 low-cost", Ground{Type: "320", Seats: 180, Off: 180, On: 180, NextKM: 600, Model: LowCost}, 20, 27, ""},
		// Boeing: 737-800, 162 passengers, one door: 37.5 minutes.
		{"737-800 full service", Ground{Type: "73H", Seats: 162, Off: 162, On: 162, NextKM: 1800, Model: Network}, 35, 44, "passengers"},
		// Boeing: 787-9, 360 passengers: about 51 minutes, set by the forward hold.
		{"787-9", Ground{Type: "789", Seats: 290, Off: 290, On: 290, NextKM: 3000, Model: Network}, 40, 60, ""},
		// Embraer: E190 at a full load: 18 minutes; a regional jet schedules 25 to 30.
		{"E175", Ground{Type: "E75", Seats: 76, Off: 76, On: 76, NextKM: 700, Model: Network}, 15, 24, ""},
		// An ATR: 20 to 25 minutes.
		{"ATR 72", Ground{Type: "AT7", Seats: 70, Off: 70, On: 70, NextKM: 350, Model: Network}, 14, 25, ""},
		// Boeing: 777-300ER, two doors, full servicing: about 62 minutes.
		{"777-300ER long haul", Ground{Type: "77W", Seats: 360, Off: 330, On: 330, NextKM: 9000, Model: Network, LongHaul: true}, 55, 75, ""},
	}
	for _, c := range cases {
		turn := TurnFor(c.g)
		if turn.Minimum < c.low || turn.Minimum > c.high || (c.path != "" && turn.Path != c.path) {
			t.Errorf("%s: %.1f minutes by the %s path, want %.0f to %.0f (%s)", c.what, turn.Minimum, turn.Path, c.low, c.high, c.path)
		}
		if turn.Scheduled < turn.Minimum || len(turn.Tasks) < 7 {
			t.Errorf("%s: scheduled %.0f for a minimum of %.1f, %d tasks", c.what, turn.Scheduled, turn.Minimum, len(turn.Tasks))
		}
		for _, k := range turn.Tasks {
			if k.End < k.Start || k.End > turn.Minimum+0.06 {
				t.Errorf("%s: task %q runs %.1f to %.1f in a turn of %.1f", c.what, k.Name, k.Start, k.End, turn.Minimum)
			}
		}
	}
	// Long haul is scheduled with half an hour over the minimum: 90
	// minutes and more for a 777.
	if long := TurnFor(Ground{Type: "77W", Seats: 360, Off: 330, On: 330, NextKM: 9000, Model: Network, LongHaul: true}); long.Scheduled < 90 || long.Scheduled > 135 {
		t.Errorf("a 777-300ER going long haul is scheduled %.0f minutes on the ground", long.Scheduled)
	}
	// A half-empty aircraft turns faster than a full one, and a wide-body
	// going long haul takes longer than the same aircraft on a short leg.
	full := TurnFor(Ground{Type: "320", Seats: 180, Off: 180, On: 180, NextKM: 1500, Model: Network})
	half := TurnFor(Ground{Type: "320", Seats: 180, Off: 90, On: 90, NextKM: 1500, Model: Network})
	if half.Minimum >= full.Minimum {
		t.Errorf("half a load turns in %.1f, a full one in %.1f", half.Minimum, full.Minimum)
	}
	if lcc, net := MinimumTurn("320", 180, UltraLowCost, 900), MinimumTurn("320", 180, Network, 900); lcc >= net || lcc < 22 {
		t.Errorf("minimum turn: low-cost %d, network %d", lcc, net)
	}
	if long, short := MinimumTurn("789", 290, Network, 9000), MinimumTurn("789", 290, Network, 1500); long <= short {
		t.Errorf("a 787 going long haul needs %d, on a short leg %d", long, short)
	}
}

// Rents, values and maintenance by age, against the appraisers' figures
// in the reference table.
func TestRentsValuesAndMaintenanceByAge(t *testing.T) {
	neo, _ := refdata.AircraftType("32N")
	if r := Rent(neo, 0); r != 400000 {
		t.Errorf("a new A320neo rents for %.0f", r)
	}
	a320, _ := refdata.AircraftType("320")
	if r := Rent(a320, 11); r != 240000 {
		t.Errorf("an eleven-year-old A320 rents for %.0f", r)
	}
	if old, mid := Rent(a320, 20), Rent(a320, 11); old >= mid || old < 0.5*mid {
		t.Errorf("a twenty-year-old A320 rents for %.0f against %.0f at eleven", old, mid)
	}
	// Lease rate factors: 0.6 to 0.8 per cent of value a month new, about
	// 0.9 at ten years, 1.4 to 1.5 at twenty.
	for _, c := range []struct{ age, low, high float64 }{{0, 0.006, 0.0085}, {10, 0.008, 0.013}, {20, 0.011, 0.022}} {
		if f := Rent(a320, c.age) / Value(a320, c.age); f < c.low || f > c.high {
			t.Errorf("lease rate factor of an A320 at %.0f years: %.4f", c.age, f)
		}
	}
	if young, mature, old := MaintenancePerBlockHour(a320, 1), MaintenancePerBlockHour(a320, 10), MaintenancePerBlockHour(a320, 22); young >= mature || mature >= old || mature != a320.Maintenance {
		t.Errorf("maintenance by age: %.0f, %.0f, %.0f", young, mature, old)
	}
	// A type out of production still has a value.
	b757, _ := refdata.AircraftType("752")
	if v := Value(b757, 20); v < 3e6 || v > 12e6 {
		t.Errorf("a twenty-year-old 757 is worth %.0f", v)
	}
}

// A fleet drawn from its tails is the same every time it is drawn, a
// low-cost carrier leases more of it and flies it younger, and the
// market has an offer for every type still built.
func TestFleetAndMarket(t *testing.T) {
	a, b := New("LH", "D-AIAB", "320", 180), New("LH", "D-AIAB", "320", 180)
	if a != b || a.Rent <= 0 || a.Value <= 0 || a.Maintenance <= 0 {
		t.Fatalf("an aircraft drawn twice: %+v %+v", a, b)
	}
	share := func(carrier string) (leased, age float64) {
		n := 400
		for i := 0; i < n; i++ {
			ac := New(carrier, string(rune('A'+i%26))+string(rune('A'+i/26)), "320", 180)
			if ac.Leased {
				leased++
				if ac.TermLeft <= 0 || ac.Lessor == "" || ac.Reserves <= 0 {
					t.Fatalf("a leased aircraft without its terms: %+v", ac)
				}
			}
			age += ac.Age
		}
		return leased / float64(n), age / float64(n)
	}
	netLeased, netAge := share("LH")
	lccLeased, lccAge := share("FR")
	if math.Abs(netLeased-0.40) > 0.08 || math.Abs(lccLeased-0.70) > 0.08 || lccAge >= netAge {
		t.Errorf("network: %.0f%% leased, %.1f years; low-cost: %.0f%% leased, %.1f years", netLeased*100, netAge, lccLeased*100, lccAge)
	}
	if ModelOf("FR") != UltraLowCost || ModelOf("WN") != LowCost || ModelOf("LH") != Network {
		t.Error("carrier models")
	}
	offers := Market(1)
	seen := map[string]bool{}
	for _, o := range offers {
		seen[o.Type] = true
		if o.Rent <= 0 || o.Deposit != 2*o.Rent || o.ACMI < 1000 || o.Term < 60 {
			t.Errorf("offer %+v", o)
		}
	}
	if !seen["32N"] || !seen["789"] || !seen["AT7"] || len(offers) < 40 {
		t.Errorf("%d offers", len(offers))
	}
	if glut := Market(0.7); glut[0].Rent >= offers[0].Rent {
		t.Error("a glut does not lower the rents")
	}
	// A wet-leased narrow-body: 2,500 to 4,500 dollars a block hour.
	for _, o := range offers {
		if o.Type == "320" && o.Age == 8 && (o.ACMI < 2500 || o.ACMI > 4500) {
			t.Errorf("an A320 on a wet lease: %.0f a block hour", o.ACMI)
		}
	}
}
