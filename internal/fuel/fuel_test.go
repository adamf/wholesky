package fuel

import (
	"math"
	"sort"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/refdata"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// The continuation's constants are the record's. Fit a first-order
// autoregression to the daily log price from 2005 on, turn it into the
// speed of reversion, the long-run level and the volatility, and hold
// the package's constants against the fit. If the embedded series is
// replaced and the fit moves, this fails and the constants are stale.
func TestTheProcessIsFittedToTheRecord(t *testing.T) {
	var x []float64
	for _, p := range refdata.JetFuel() {
		if p.Date.Year() >= 2005 && p.Value > 0 {
			x = append(x, math.Log(p.Value))
		}
	}
	n := float64(len(x) - 1)
	var sx, sy, sxx, sxy float64
	for i := 1; i < len(x); i++ {
		sx += x[i-1]
		sy += x[i]
		sxx += x[i-1] * x[i-1]
		sxy += x[i-1] * x[i]
	}
	b := (n*sxy - sx*sy) / (n*sxx - sx*sx)
	a := (sy - b*sx) / n
	kappa := -math.Log(b) * tradingDays
	level := math.Exp(a / (1 - b))
	var resid []float64
	var ss float64
	for i := 1; i < len(x); i++ {
		e := x[i] - a - b*x[i-1]
		resid = append(resid, e)
		ss += e * e
	}
	sd := math.Sqrt(ss / n)
	total := sd * math.Sqrt(tradingDays)
	// Days beyond three standard deviations are the jumps.
	var clipped float64
	jumps := 0
	for _, e := range resid {
		if math.Abs(e) > 3*sd {
			jumps++
			continue
		}
		clipped += e * e
	}
	diffusion := math.Sqrt(clipped/float64(len(resid)-jumps)) * math.Sqrt(tradingDays)
	perYear := float64(jumps) / (n / tradingDays)
	t.Logf("fit 2005-2026: reversion %.2f a year, long-run level $%.2f, volatility %.0f%% (%.0f%% without %.1f jumps a year)", kappa, level, total*100, diffusion*100, perYear)
	if math.Abs(kappa-Reversion) > 0.05 {
		t.Errorf("reversion: the record gives %.2f, the package uses %.2f", kappa, Reversion)
	}
	if math.Abs(level-LongRun) > 0.05 {
		t.Errorf("long-run level: the record gives %.2f, the package uses %.2f", level, LongRun)
	}
	if math.Abs(diffusion-Volatility) > 0.02 {
		t.Errorf("volatility without jumps: the record gives %.3f, the package uses %.3f", diffusion, Volatility)
	}
	if math.Abs(perYear-JumpsPerYear) > 1.5 {
		t.Errorf("jumps: the record has %.1f a year, the package draws %.1f", perYear, JumpsPerYear)
	}
	// The process's whole volatility, steps and jumps, against the record's.
	model := math.Sqrt(Volatility*Volatility + JumpsPerYear*JumpSize*JumpSize)
	if math.Abs(model-total) > 0.02 {
		t.Errorf("whole volatility: the record gives %.2f, the process %.2f", total, model)
	}
}

// A world that starts inside the record lives it: fuel in July 2008 is
// near four dollars and by December is under a dollar and a half.
func TestAWorldInsideTheRecordLivesIt(t *testing.T) {
	m := New(date(2008, 1, 2), 1)
	day := func(d time.Time) int { return int(d.Sub(m.Start).Hours() / 24) }
	jul, dec := m.Reference(day(date(2008, 7, 3))), m.Reference(day(date(2008, 12, 24)))
	if jul < 3.8 || jul > 4.5 || dec > 1.5 || !m.Recorded(day(date(2008, 7, 3))) {
		t.Errorf("July 2008 at %.2f, December at %.2f", jul, dec)
	}
	// What a carrier paid lagged the fall: the index is an average of the
	// two months before.
	if paid := m.Index(day(date(2008, 10, 15))); paid < m.Reference(day(date(2008, 10, 15)))+0.4 {
		t.Errorf("in a falling market the averaged index stays above spot: %.2f against %.2f", paid, m.Reference(day(date(2008, 10, 15))))
	}
	// March 2026: spot rose by more than half in a month, and the index a
	// carrier pays on rose by much less.
	n := New(date(2026, 2, 2), 1)
	spot0, spot1 := n.Reference(0), n.Reference(57)
	idx0, idx1 := n.Index(0), n.Index(57)
	if spot1/spot0 < 1.4 || idx1/idx0 > spot1/spot0-0.15 {
		t.Errorf("February to end of March 2026: spot x%.2f, index x%.2f", spot1/spot0, idx1/idx0)
	}
}

// Past the record the price is the world's own: the same seed gives the
// same path on every machine, another seed another path, and over many
// worlds the paths move the way the record did.
func TestTheContinuation(t *testing.T) {
	end := refdata.JetFuel()[len(refdata.JetFuel())-1].Date
	a, b, c := New(end, 7), New(end, 7), New(end, 8)
	if a.Recorded(5) {
		t.Fatal("five days past the record is not the record")
	}
	same, differs := true, false
	for d := 1; d < 400; d += 13 {
		if a.Reference(d) != b.Reference(d) {
			same = false
		}
		if a.Reference(d) != c.Reference(d) {
			differs = true
		}
	}
	if !same || !differs {
		t.Fatalf("same seed same path %v, other seed other path %v", same, differs)
	}
	// Three hundred worlds, a year on: the 12-month change in the record
	// had its 5th and 95th percentiles at -44 and +84 per cent. The start
	// is well above the long-run level in November 2026, so the paths
	// drift down; compare changes after taking the expected one out.
	start := a.Reference(0)
	var change []float64
	for seed := int64(1); seed <= 300; seed++ {
		w := New(end, seed)
		change = append(change, math.Log(w.Reference(365)/start))
	}
	sort.Float64s(change)
	mean := 0.0
	for _, v := range change {
		mean += v
	}
	mean /= float64(len(change))
	lo, hi := math.Exp(change[15]-mean)-1, math.Exp(change[285]-mean)-1
	t.Logf("a year on: expected change %+.0f%%, 5th and 95th percentiles around it %+.0f%% and %+.0f%%", (math.Exp(mean)-1)*100, lo*100, hi*100)
	if lo > -0.25 || lo < -0.60 || hi < 0.35 || hi > 1.30 {
		t.Errorf("the spread of the 12-month change is not the record's: %+.2f to %+.2f", lo, hi)
	}
	if want := a.Forward(0, 365); math.Abs(math.Exp(mean)*start-want)/want > 0.12 {
		t.Errorf("the forward price %.2f is not where the paths go on average, %.2f", want, math.Exp(mean)*start)
	}
	// Every price is a price.
	for d := 0; d < 3000; d += 7 {
		if p := a.Reference(d); p < 0.3 || p > 15 || math.IsNaN(p) {
			t.Fatalf("day %d: %.2f", d, p)
		}
	}
}

// A station pays its region's differential and its own into-plane charge,
// and a shock moves every station.
func TestStationsRegionsAndShocks(t *testing.T) {
	m := New(date(2019, 6, 3), 1)
	eu, me := m.Paid(0, refdata.Europe, 200), m.Paid(0, refdata.MiddleEast, 200)
	if d := eu - me; d < 0.25 || d > 0.40 {
		t.Errorf("Europe over the Middle East in a calm week: %.2f a gallon", d)
	}
	if big, small := m.Paid(0, refdata.Europe, 600), m.Paid(0, refdata.Europe, 5); small-big < 0.4 {
		t.Errorf("a small station pays more to fuel: %.2f against %.2f", small, big)
	}
	// US carriers paid 12 cents over Gulf Coast spot in 2019.
	if over := m.Paid(0, refdata.NorthAmerica, 300) - m.Index(0); over < 0.10 || over > 0.20 {
		t.Errorf("a large US station pays %.2f over the index", over)
	}
	before := m.Reference(100)
	m.AddShock(Shock{Day: 90, Step: 0.4, HalfLife: 60, Name: "strait closed"})
	if after := m.Reference(100); after/before < 1.35 || after/before > 1.50 {
		t.Errorf("ten days into a shock of 0.4: x%.2f", after/before)
	}
	if late := m.Reference(450); math.Abs(late/New(date(2019, 6, 3), 1).Reference(450)-1) > 0.02 {
		t.Error("a shock with a 60-day half-life is still there a year on")
	}
	// In a crisis Europe's differential widens.
	crisis := New(date(2026, 9, 1), 1)
	if calm, hot := m.Basis(0, refdata.Europe), crisis.Basis(0, refdata.Europe); hot <= calm+0.05 {
		t.Errorf("Europe's differential in September 2026 (%.2f) against June 2019 (%.2f)", hot, calm)
	}
}

// What a flight burns, against the source's own trips.
func TestBurn(t *testing.T) {
	for _, c := range []struct {
		code   string
		nm     int
		source float64
	}{{"320", 1000, 6192}, {"73H", 1000, 6382}, {"AT7", 250, 915}, {"789", 4000, 43812}, {"77W", 4000, 70988}} {
		got := Burn(c.code, 0, int(float64(c.nm)*1.852))
		// Within the five per cent the route adds to the great circle.
		if got < c.source*0.97 || got > c.source*1.08 {
			t.Errorf("%s over %d nm burns %.0f kg, the source says %.0f", c.code, c.nm, got, c.source)
		}
	}
	if dense, sparse := Burn("320", 186, 1500), Burn("320", 150, 1500); dense <= sparse {
		t.Error("a denser cabin burns no more")
	}
	if g := Gallons(3040); g != 1000 {
		t.Errorf("3,040 kg is %v gallons", g)
	}
	if kg := Burn("ZZZ", 150, 1000); kg < 3000 || kg > 5000 {
		t.Errorf("an unknown 150-seat type over 1,000 km burns %.0f kg", kg)
	}
}

// A swap for the gallons a carrier burns fixes what it pays; a cap pays
// only above its strike and costs a premium; a collar costs nothing to
// enter.
func TestHedges(t *testing.T) {
	m := New(date(2008, 1, 2), 1)
	gallons := 100000.0
	swap := m.Quote(Swap, 0, 30, 330, gallons, 0)
	if swap.Strike < 2.2 || swap.Strike > 2.9 || swap.Premium != 0 {
		t.Fatalf("a swap bought in January 2008: %+v", swap)
	}
	// July 2008: the swap pays the difference, and the net price is the
	// fixed one.
	day := 185
	net := m.Index(day)*gallons - swap.Settle(m, day)
	if math.Abs(net-swap.Strike*gallons) > 1 {
		t.Errorf("fuel %.0f less the swap %.0f is %.0f, the fixed price is %.0f", m.Index(day)*gallons, swap.Settle(m, day), net, swap.Strike*gallons)
	}
	if swap.Settle(m, 10) != 0 || swap.Settle(m, 400) != 0 {
		t.Error("a swap settled outside its term")
	}
	// The same swap bought in July is a liability by December.
	late := m.Quote(Swap, 185, 30, 330, gallons, 0)
	if v := late.Value(m, 350); v > -10e6 {
		t.Errorf("a swap fixed at %.2f in July 2008, marked in December at %.2f: %.0f", late.Strike, m.Reference(350), v)
	}
	cap := m.Quote(Cap, 0, 30, 330, gallons, 1.1)
	notional := cap.Strike / 1.1 * gallons * 330
	if share := cap.Premium / notional; share < 0.02 || share > 0.10 {
		t.Errorf("a cap ten per cent out of the money for eleven months costs %.1f%% of what it covers", share*100)
	}
	if cap.Settle(m, day) <= 0 || cap.Settle(m, 345) != 0 {
		t.Errorf("a cap pays above its strike only: July %.0f, December %.0f", cap.Settle(m, day), cap.Settle(m, 345))
	}
	collar := m.Quote(Collar, 0, 30, 330, gallons, 1.1)
	if v := collar.Value(m, 0); math.Abs(v) > 0.01*notional || collar.Floor >= collar.Strike || collar.Floor <= 0 {
		t.Errorf("a collar worth %.0f at the start, floor %.2f, cap %.2f", v, collar.Floor, collar.Strike)
	}
	if collar.Settle(m, 345) >= 0 {
		t.Errorf("with fuel at %.2f a collar with a floor at %.2f costs the carrier", m.Index(345), collar.Floor)
	}
}
