package noc

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/workforce"
	"github.com/adamf/wholesky/internal/world"
)

var day = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

// hubWorld is B6 at JFK: sixteen tails out and back to BOS twice, then to
// ORD and back, with another carrier's arrivals into JFK competing for
// the slots.
func hubWorld() *world.Manifest {
	m := &world.Manifest{
		Airports: []world.Airport{
			{IATA: "JFK", ICAO: "KJFK", Country: "United States", Lat: 40.64, Lon: -73.78},
			{IATA: "BOS", ICAO: "KBOS", Country: "United States", Lat: 42.36, Lon: -71.01},
			{IATA: "ORD", ICAO: "KORD", Country: "United States", Lat: 41.98, Lon: -87.90},
		},
		Carriers: []world.Carrier{{Designator: "B6", Hub: "JFK", Country: "United States"}, {Designator: "AA", Hub: "ORD", Country: "United States"}},
	}
	n := 100
	add := func(c, tail, from, to string, dep, block, km int) {
		m.Flights = append(m.Flights, world.Flight{Carrier: c, Number: fmt.Sprintf("%04d", n), From: from, To: to, DepMin: dep, ArrMin: dep + block,
			BlockMin: block, Seats: 150, Equipment: "320", Tail: tail, KM: km})
		n++
	}
	for t := 0; t < 16; t++ {
		tail := fmt.Sprintf("N%02dJB", t)
		dep := 6*60 + t*15
		for _, l := range []struct {
			from, to  string
			block, km int
		}{{"JFK", "BOS", 75, 300}, {"BOS", "JFK", 80, 300}, {"JFK", "ORD", 160, 1180}, {"ORD", "JFK", 140, 1180}, {"JFK", "BOS", 75, 300}, {"BOS", "JFK", 80, 300}} {
			add("B6", tail, l.from, l.to, dep, l.block, l.km)
			dep += l.block + 40
		}
	}
	for i := 0; i < 30; i++ {
		add("AA", fmt.Sprintf("N%02dAA", i), "ORD", "JFK", 10*60+i*15, 140, 1180)
	}
	return m
}

func model(m *world.Manifest) *Model {
	return &Model{World: m, Carrier: "B6", Day: day, Roster: workforce.New(m, day, nil)}
}

func storm() Scenario {
	return Scenario{Name: "storm at JFK", Events: []Event{{Kind: KindWeather, Airport: "JFK", Onset: Tri(13*60, 14*60, 15*60),
		Duration: Tri(120, 240, 360), Factor: Tri(0.3, 0.45, 0.6)}}}
}

func TestQuantiles(t *testing.T) {
	if v := (Dist{Kind: "uniform", A: 10, B: 20}).Quantile(0.25); math.Abs(v-12.5) > 1e-9 {
		t.Errorf("uniform %v", v)
	}
	tri := Tri(0, 2, 10)
	// The CDF at the mode is (mode-lo)/(hi-lo) = 0.2.
	if v := tri.Quantile(0.2); math.Abs(v-2) > 1e-9 {
		t.Errorf("triangular at the mode's quantile %v", v)
	}
	if v := Normal(100, 15).Quantile(0.5); math.Abs(v-100) > 1e-9 {
		t.Errorf("normal median %v", v)
	}
	// The normal's 97.5th percentile is 1.95996 deviations up.
	if v := Normal(0, 1).Quantile(0.975); math.Abs(v-1.959964) > 1e-5 {
		t.Errorf("normal 97.5%% %v", v)
	}
	if v := (Dist{Kind: "lognormal", A: 50, B: 0.4}).Quantile(0.5); math.Abs(v-50) > 1e-9 {
		t.Errorf("lognormal median %v", v)
	}
}

func TestSummarise(t *testing.T) {
	var xs []float64
	for i := 1; i <= 101; i++ {
		xs = append(xs, float64(i))
	}
	s := Summarise(xs)
	if s.Mean != 51 || s.P50 != 51 || s.P10 != 11 || s.P90 != 91 || s.Min != 1 || s.Max != 101 {
		t.Errorf("%+v", s)
	}
	// SD of 1..101 is sqrt(101*102/12) = 29.30.
	if math.Abs(s.SD-29.3002) > 1e-3 {
		t.Errorf("sd %v", s.SD)
	}
}

// A Latin hypercube puts exactly one draw in each of N strata of every
// dimension.
func TestLatinHypercubeStratifies(t *testing.T) {
	o := Options{Trials: 50, Seed: 7, Method: "lhs"}
	us := draws(o, 5)
	for d := 0; d < 5; d++ {
		seen := make([]bool, 50)
		for _, u := range us {
			k := int(u[d] * 50)
			if seen[k] {
				t.Fatalf("dimension %d stratum %d drawn twice", d, k)
			}
			seen[k] = true
		}
	}
	mc := draws(Options{Trials: 50, Seed: 7, Method: "mc"}, 5)
	twice := false
	for d := 0; d < 5 && !twice; d++ {
		seen := make([]bool, 50)
		for _, u := range mc {
			k := int(u[d] * 50)
			twice = twice || seen[k]
			seen[k] = true
		}
	}
	if !twice {
		t.Error("plain Monte Carlo stratified by chance in every dimension")
	}
}

// A storm over the hub costs money in every play; the spread is real, the
// projection reproduces itself from its seed, and the thinner schedule
// cancels flights before the day.
func TestProjectionOfAStorm(t *testing.T) {
	m := model(hubWorld())
	o := Options{Trials: 40, Seed: 1, Method: "lhs", Workers: 4}
	rs, err := m.Compare(storm(), []Policy{{Name: "absorb"}, {Name: "thin 30%", Thin: 0.3}, {Name: "standby crews", Standby: 2}}, o)
	if err != nil {
		t.Fatal(err)
	}
	absorb := rs[0]
	if absorb.Cost.Mean <= 0 || absorb.Cost.P95 < absorb.Cost.P50 || absorb.Cost.SD == 0 {
		t.Fatalf("absorb %+v", absorb.Cost)
	}
	if absorb.Metrics["delay_min"].Mean <= float64(absorb.Baseline.DelayMin) {
		t.Errorf("the storm delays nothing: %+v vs baseline %d", absorb.Metrics["delay_min"], absorb.Baseline.DelayMin)
	}
	thin := rs[1]
	if thin.Cancels == 0 || thin.Metrics["cancelled"].Mean < float64(thin.Cancels) {
		t.Errorf("thinning cancelled %d planned, %v played", thin.Cancels, thin.Metrics["cancelled"])
	}
	if thin.Metrics["delay_min"].Mean >= absorb.Metrics["delay_min"].Mean {
		t.Errorf("thinning the schedule did not cut the delays: %v vs %v", thin.Metrics["delay_min"].Mean, absorb.Metrics["delay_min"].Mean)
	}
	if thin.Delta.Mean != thin.Cost.Mean-absorb.Cost.Mean && math.Abs(thin.Delta.Mean-(thin.Cost.Mean-absorb.Cost.Mean)) > 1 {
		t.Errorf("delta %v against %v - %v", thin.Delta.Mean, thin.Cost.Mean, absorb.Cost.Mean)
	}
	if rs[2].Standby["JFK"] != 2 {
		t.Errorf("standby at the hub under the storm: %v", rs[2].Standby)
	}
	again, _ := m.Compare(storm(), []Policy{{Name: "absorb"}}, o)
	if again[0].Cost != absorb.Cost {
		t.Error("the same seed gave a different projection")
	}
}

// An event less likely than not is played in about its share of trials.
func TestProbabilityOfAnEvent(t *testing.T) {
	m := model(hubWorld())
	sc := storm()
	sc.Events[0].Probability = 0.3
	r, err := m.Run(sc, Policy{Name: "absorb"}, Options{Trials: 100, Seed: 3, Method: "lhs", Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	quiet := 0
	for _, c := range r.costs {
		if math.Abs(c) < 1 {
			quiet++
		}
	}
	if quiet < 60 || quiet > 80 {
		t.Errorf("%d of 100 plays had no storm at p=0.3", quiet)
	}
}

func TestSensitivityAndStress(t *testing.T) {
	m := model(hubWorld())
	sw, err := m.Sensitivity(storm(), Policy{Name: "absorb"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sw) != 3 {
		t.Fatalf("three uncertainties, %d swings", len(sw))
	}
	if !sort.SliceIsSorted(sw, func(i, j int) bool { return sw[i].Range > sw[j].Range }) {
		t.Error("the tornado is not sorted")
	}
	mid, _ := m.Stress(storm(), Policy{Name: "absorb"}, 0.5)
	bad, _ := m.Stress(storm(), Policy{Name: "absorb"}, 0.9)
	if bad.Total <= mid.Total || bad.DelayMin <= mid.DelayMin {
		t.Errorf("the bad day %+v is no worse than the expected %+v", bad, mid)
	}
}

// Own delays drawn from a recorded day vary from play to play.
func TestHistoricalBootstrap(t *testing.T) {
	m := model(hubWorld())
	m.History = []int{0, 0, 0, 5, 10, 20, 45, 90, 180}
	r, err := m.Run(Scenario{Name: "a normal day"}, Policy{Name: "absorb"}, Options{Trials: 20, Seed: 2, Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	if r.Metrics["delay_min"].SD == 0 {
		t.Error("every bootstrapped day had the same delays")
	}
	if r.Cost.Max != 0 || r.Cost.Min != 0 {
		t.Errorf("a day with no events costs nothing over its own baseline: %+v", r.Cost)
	}
}

// The real forecasts the data service returned: KORD's base group is
// overcast at 800 feet (IFR), its PROB30 group drizzle and mist under 700
// feet; EGLL's PROB40 TEMPO is broken at 900 feet.
func TestTAFsFromTheService(t *testing.T) {
	f, err := os.Open("testdata/taf.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tafs, err := ParseTAFs(f)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]TAF{}
	for _, t := range tafs {
		byID[t.ICAO] = t
	}
	ord := byID["KORD"]
	if ord.Groups[0].Category() != "IFR" || ord.Groups[0].Kind() != "base" {
		t.Errorf("KORD base: %s %s", ord.Groups[0].Kind(), ord.Groups[0].Category())
	}
	if g := ord.Groups[3]; g.Kind() != "PROB" || *g.Probability != 30 || g.Category() != "IFR" {
		t.Errorf("KORD PROB30: %s %d %s", g.Kind(), *g.Probability, g.Category())
	}
	if byID["KJFK"].Groups[0].Category() != "VFR" {
		t.Error("KJFK is clear")
	}
	issued := time.Unix(ord.From, 0).UTC()
	sc := FromTAFs(tafs, issued, func(icao string) string { return map[string]string{"KORD": "ORD", "EGLL": "LHR"}[icao] })
	var prob30, prob40 bool
	for _, e := range sc.Events {
		if e.Airport == "JFK" {
			t.Error("a clear forecast made an event")
		}
		prob30 = prob30 || e.Airport == "ORD" && e.Probability == 0.3
		prob40 = prob40 || e.Airport == "LHR" && e.Probability == 0.4 && e.Duration.Kind == "triangular"
	}
	if !prob30 || !prob40 {
		t.Errorf("events %+v", sc.Events)
	}
}

var _ = dayplan.Key

// A policy aimed at one event thins for that event alone.
func TestPolicyTarget(t *testing.T) {
	m := model(hubWorld())
	sc := storm()
	sc.Events[0].Name = "JFKA"
	sc.Events = append(sc.Events, Event{Kind: KindWeather, Name: "ORDB", Airport: "ORD", Onset: Fixed(16 * 60), Duration: Fixed(240), Factor: Fixed(0.4)})
	all := Policy{Name: "thin 30%", Thin: 0.3}.Decide(m, sc)
	one := Policy{Name: "thin 30%", Thin: 0.3, Target: "JFKA"}.Decide(m, sc)
	if len(one.Cancel) == 0 || len(one.Cancel) >= len(all.Cancel) {
		t.Errorf("targeted %d cancellations, untargeted %d", len(one.Cancel), len(all.Cancel))
	}
	for k, reason := range one.Cancel {
		if !strings.Contains(reason, "JFK") {
			t.Errorf("%s thinned for %q", k, reason)
		}
	}
}
