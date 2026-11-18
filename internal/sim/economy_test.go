package sim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/fuel"
)

// What a carrier's day costs, flight by flight, comes out where real
// carriers' costs are: a few cents a seat-kilometre, with fuel a fifth to
// a third of it. Before, a flight cost a constant per block minute and
// fuel, rent, maintenance and airports were not there.
func TestAFlightCostsWhatItsPartsCost(t *testing.T) {
	s := bootWorld(t, Options{EconomyStart: time.Date(2019, 6, 3, 0, 0, 0, 0, time.UTC)})
	if s.econ == nil {
		t.Fatal("the world has no economy")
	}
	day, date := s.Day()
	if day != 0 || date.Format("2006-01-02") != "2019-06-03" || !s.econ.fuel.Recorded(0) {
		t.Fatalf("day %d on %v", day, date)
	}
	var cost, fuelCost, ask float64
	flights := 0
	for code, fs := range s.Flights {
		for _, f := range fs {
			fc := s.econ.flightCost(f, f.Seats*8/10)
			if fc.Fuel <= 0 || fc.Maintenance <= 0 || fc.Airports <= 0 || fc.Handling <= 0 || fc.Gallons <= 0 {
				t.Fatalf("%s%s: a cost line is missing: %+v", f.Carrier, f.Number, fc)
			}
			cost += float64(fc.total())
			fuelCost += float64(fc.Fuel)
			ask += float64(f.Seats) * float64(f.KM)
			flights++
		}
		rent, own := s.econ.fixedCosts(code, 1)
		if rent+own <= 0 {
			t.Fatalf("%s's fleet costs nothing to hold", code)
		}
		cost += float64(rent + own)
	}
	cask := cost / ask // cents per seat-km, before crew
	share := fuelCost / cost
	t.Logf("%d flights: %.1f cents a seat-km before crew and overheads, fuel %.0f%% of it at $%.2f a gallon", flights, cask, share*100, s.econ.fuel.Reference(0))
	// A world of short sectors is dear by the seat-kilometre: easyJet's
	// costs before crew and overheads were about 6 US cents in 2019 on
	// sectors of 1,100 km, and these average under 700.
	if cask < 4 || cask > 10 {
		t.Errorf("cost of %.1f cents a seat-km before crew; short-haul carriers run 5 to 9", cask)
	}
	if share < 0.15 || share > 0.45 {
		t.Errorf("fuel is %.0f%% of cost; it was 24%% of airline costs in 2019", share*100)
	}

	// The scorecard carries the lines.
	code := s.Manifest.Carriers[0].Designator
	sc := seatWorld{s}.Score(code)
	if _, flat := sc.Costs["block hours"]; flat {
		t.Error("the scorecard still prices a flight by the block minute")
	}
	if sc.Costs["aircraft rent"]+sc.Costs["aircraft ownership"] <= 0 {
		t.Errorf("no fleet cost on the scorecard: %v", sc.Costs)
	}
}

// Fuel moves, and a hedge fixes what it costs. A shock raises the price a
// carrier pays; a carrier that swapped its fuel beforehand is paid the
// difference by its hedge.
func TestFuelShocksAndHedges(t *testing.T) {
	s := bootWorld(t, Options{EconomyStart: time.Date(2019, 6, 3, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	code := s.Manifest.Carriers[0].Designator
	hub := s.carriers[code].Hub
	w := seatWorld{s}
	// Start from no hedge at all, then fix 60 per cent for a year.
	s.econ.mu.Lock()
	s.econ.hedges[code] = nil
	s.econ.mu.Unlock()
	res, err := w.Act(ctx, code, airline.Action{Kind: "hedge", Policy: "swap", Count: 60, Minutes: 365})
	if err != nil || !strings.Contains(res, "60% of the fuel fixed") {
		t.Fatalf("hedge: %q %v", res, err)
	}
	if _, err := w.Act(ctx, code, airline.Action{Kind: "hedge", Policy: "swap", Count: 60}); err == nil {
		t.Error("a carrier hedged more fuel than it burns")
	}
	if _, err := w.Act(ctx, code, airline.Action{Kind: "hedge", Policy: "futures", Count: 10}); err == nil {
		t.Error("a hedge that is not a swap, a cap or a collar was taken")
	}
	// The hedge starts tomorrow: move the world on sixty days, so the
	// index the carrier pays on has caught up with the price.
	s.econ.day.Store(60)
	before := s.econ.fuelPrice(hub)
	out, err := s.applyInject(ctx, nocEvent{Kind: "fuel", Factor: 0.5, Duration: 400, Source: "test"})
	if err != nil || !strings.Contains(out, "fuel shock") {
		t.Fatalf("inject: %q %v", out, err)
	}
	s.econ.day.Store(130)
	after := s.econ.fuelPrice(hub)
	if after < before*1.3 {
		t.Fatalf("fuel at the hub was $%.2f, and $%.2f seventy days into a shock of a half", before, after)
	}
	paid := s.econ.hedgeResult(code, 1)
	extra := (after - before) * s.econ.burn[code] * 100 // cents a day more for all the fuel
	if float64(paid) < 0.4*extra || float64(paid) > 0.8*extra {
		t.Errorf("a swap on 60%% of the fuel pays %d cents a day against a rise of %.0f on all of it", paid, extra)
	}

	// The view a seat and an agent read.
	mux := http.NewServeMux()
	(&airline.Server{Reg: s.Airline, World: seatWorld{s}}).Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/carrier/"+code+"/economy", nil))
	var v economyView
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &v) != nil {
		t.Fatalf("economy %d %s", rec.Code, rec.Body.String())
	}
	if v.Fuel.Hedged != 0.6 || len(v.Fuel.Hedges) != 1 || v.Fuel.Hedges[0].Kind != fuel.Swap || v.Fuel.Hedges[0].Value <= 0 || len(v.Fuel.History) != 60 || len(v.Fuel.Shocks) != 1 {
		t.Errorf("fuel view: %+v", v.Fuel)
	}
	if v.Fleet.Aircraft == 0 || len(v.Fleet.ByType) == 0 || len(v.Fleet.Offers) == 0 || v.Fleet.Value <= 0 || v.Date != "2019-10-11" {
		t.Errorf("fleet view: %d aircraft, %d types, %d offers, date %s", v.Fleet.Aircraft, len(v.Fleet.ByType), len(v.Fleet.Offers), v.Date)
	}

	// The calendar and the hedge survive a save and a restore.
	st := s.snapshot()
	if st.Economy == nil || st.Economy.Day != 130 || len(st.Economy.Hedges[code]) != 1 || len(st.Economy.Shocks) != 1 {
		t.Errorf("saved economy: %+v", st.Economy)
	}
}

// The turnaround before a departure, task by task: what the aircraft
// does on the ground between landing and leaving.
func TestTheTurnaroundOfADeparture(t *testing.T) {
	s := bootWorld(t, Options{})
	w := seatWorld{s}
	// A leg that is not the first of its aircraft's day.
	for code, fs := range s.Flights {
		for _, f := range fs {
			v, ok := w.Turn(code, f.Carrier+f.Number, f.From)
			if !ok {
				t.Fatalf("no turnaround for %s%s", f.Carrier, f.Number)
			}
			b, _ := json.Marshal(v)
			var got struct {
				Inbound    string  `json:"inbound"`
				GroundTime int     `json:"ground_time"`
				Minimum    float64 `json:"minimum"`
				Path       string  `json:"path"`
				Slack      int     `json:"slack"`
				Tasks      []struct {
					Name       string
					Start, End float64
				} `json:"tasks"`
			}
			json.Unmarshal(b, &got) //nolint:errcheck
			if got.Minimum <= 0 || len(got.Tasks) < 7 || got.Path == "" {
				t.Fatalf("%s%s: %s", f.Carrier, f.Number, b)
			}
			if got.Inbound == "" {
				continue
			}
			// The schedule gave the aircraft at least the time its tasks
			// take.
			if got.GroundTime <= 0 || got.Slack < -12 {
				t.Errorf("%s%s is given %d minutes on the ground for a turnaround of %.0f: %s", f.Carrier, f.Number, got.GroundTime, got.Minimum, b)
			}
			return
		}
	}
	t.Skip("no aircraft flies two legs in this world")
}
