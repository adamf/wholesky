package fill

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// A carrier's revenue management sets each flight's class limits from the
// flight's own earlier departures. How it reads those departures decides
// whether that works.
//
// One network carrier's 900 flights are sold for 38 days at a peak: 1.4
// times the travellers of an ordinary day, so that cabins fill and the
// limits matter. The first 7 days run on the tariff's fixed limits. The
// carrier then reads its record in one of 3 ways.
//
// Read as demand, sales feed on themselves. A class that closed sold
// nothing after it closed, and a traveller who would have paid more
// bought the cheapest class open. The forecast for the dear classes
// falls, fewer seats are held for them, the cheap classes take the
// seats, and the dear classes sell less again. Revenue falls from one
// day to the next and does not stop. This is the spiral-down of Cooper,
// Homem-de-Mello and Kleywegt (2006).
//
// Divided by the time each class was on sale, sales are unconstrained,
// and the fall stops. Buy-down is still there: of two classes with the
// same rules, the dearer one sells only while the cheaper one is closed,
// so its forecast falls to nothing.
//
// Read by the lowest class on sale, with the marginal revenue
// transformation of Fiig, Isler, Hopperstad and Belobaba (2010), the
// record says what opening each class adds and what it takes from the
// classes above it. Revenue holds and is the highest of the 3.
func TestAForecastThatReadsItsOwnSalesSpiralsDown(t *testing.T) {
	if raceEnabled {
		t.Skip("38 days of a carrier's booking windows, read three ways: CI runs it without the race detector")
	}
	m := usWorld(t)
	one := &world.Manifest{Carriers: m.Carriers, Airports: m.Airports}
	for _, f := range m.Flights {
		if f.Carrier == "UA" {
			one.Flights = append(one.Flights, f)
		}
	}
	if len(one.Flights) < 500 {
		t.Fatalf("%d flights", len(one.Flights))
	}
	tf := tariff.FromManifest(m)
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	const days = 38
	type result struct {
		fares []float64 // dollars a day
		full  []float64 // full fare's share of the seats sold
		upper []float64 // K's share: the dearer of the two 14-day classes
	}
	run := func(method inventory.Method) result {
		mem := rm.New()
		mem.Method = method
		var r result
		for d := 0; d < days; d++ {
			plan, err := Day(context.Background(), one, Options{LoadFactor: 0.82, Seed: 3 + int64(d)*7919, Day: day, Tariff: tf, Base: tf.BaseFare,
				Cabins: func(f world.Flight) map[string]int { return host.Cabins(f, 0) },
				Share:  func(world.Flight) float64 { return 1.4 },
				RM:     mem, Dry: true}, nil)
			if err != nil {
				t.Error(err)
				return r
			}
			r.fares = append(r.fares, float64(plan.Fares)/100)
			r.full = append(r.full, float64(plan.Classes["Y"])/float64(plan.Sold))
			r.upper = append(r.upper, float64(plan.Classes["K"])/float64(plan.Sold))
		}
		return r
	}
	methods := []inventory.Method{inventory.SalesAsDemand, inventory.Unconstrained, inventory.FareAdjusted}
	results := make([]result, len(methods))
	var wg sync.WaitGroup
	for i, method := range methods {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = run(method)
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	mean := func(v []float64, from, to int) float64 {
		var sum float64
		for _, x := range v[from:to] {
			sum += x
		}
		return sum / float64(to-from)
	}
	sales, unconstrained, adjusted := results[0], results[1], results[2]
	// The second to the sixth day on the carrier's own limits, and the
	// last 5.
	early := func(r result) float64 { return mean(r.fares, rm.Warm+1, rm.Warm+6) }
	late := func(r result) float64 { return mean(r.fares, days-5, days) }
	fixed := mean(sales.fares, 0, rm.Warm)
	t.Logf("fixed limits: %.0f a day", fixed)
	t.Logf("sales as demand: %.0f early, %.0f late (%+.1f%%); full fare %.3f to %.3f of seats", early(sales), late(sales), (late(sales)/early(sales)-1)*100, sales.full[rm.Warm+1], sales.full[days-1])
	t.Logf("unconstrained: %.0f early, %.0f late (%+.1f%%); class K %.3f of seats", early(unconstrained), late(unconstrained), (late(unconstrained)/early(unconstrained)-1)*100, unconstrained.upper[days-1])
	t.Logf("fare adjusted: %.0f early, %.0f late (%+.1f%%); class K %.3f of seats", early(adjusted), late(adjusted), (late(adjusted)/early(adjusted)-1)*100, adjusted.upper[days-1])

	if late(sales) > 0.97*early(sales) {
		t.Errorf("reading sales as demand, revenue went from %.0f to %.0f a day; it should have fallen by more than 3%%", early(sales), late(sales))
	}
	if sales.full[days-1] > 0.7*sales.full[rm.Warm+1] {
		t.Errorf("reading sales as demand, full fare went from %.3f to %.3f of seats; it should have lost a third", sales.full[rm.Warm+1], sales.full[days-1])
	}
	if late(sales) > 1.02*fixed {
		t.Errorf("reading sales as demand, the carrier ends at %.0f a day; the fixed limits it began on earned %.0f", late(sales), fixed)
	}
	if late(unconstrained) < 0.99*early(unconstrained) {
		t.Errorf("unconstrained, revenue went from %.0f to %.0f a day; it should hold", early(unconstrained), late(unconstrained))
	}
	if late(adjusted) < early(adjusted) {
		t.Errorf("fare adjusted, revenue went from %.0f to %.0f a day; it should hold", early(adjusted), late(adjusted))
	}
	if late(adjusted) < 1.02*late(unconstrained) || late(adjusted) < 1.10*late(sales) || late(adjusted) < 1.10*fixed {
		t.Errorf("fare adjusted earns %.0f a day against %.0f unconstrained, %.0f reading sales and %.0f on fixed limits", late(adjusted), late(unconstrained), late(sales), fixed)
	}
	// Buy-down. K and L have the same rules and K costs more. Read class
	// by class, K's demand is what it sold while L was closed.
	before := mean(sales.upper, 0, rm.Warm)
	if u := unconstrained.upper[days-1]; u > before/4 {
		t.Errorf("unconstrained, class K is %.3f of seats against %.3f on the fixed limits; buy-down should have taken it", u, before)
	}
	if a, u := adjusted.upper[days-1], unconstrained.upper[days-1]; a < 2*u {
		t.Errorf("fare adjusted, class K is %.3f of seats against %.3f unconstrained", a, u)
	}
}
