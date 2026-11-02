package noc

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Options set how a projection samples.
type Options struct {
	Trials int    `json:"trials"`
	Seed   uint64 `json:"seed"`
	// Method is "mc" for plain Monte Carlo or "lhs" for a Latin hypercube:
	// every dimension's range cut into as many strata as there are trials
	// and each stratum drawn once, which spreads few trials evenly and
	// narrows the error of the mean for the same cost.
	Method  string `json:"method"`
	Workers int    `json:"workers,omitempty"`
}

func (o Options) norm() Options {
	if o.Trials <= 0 {
		o.Trials = 200
	}
	if o.Method == "" {
		o.Method = "lhs"
	}
	if o.Workers <= 0 {
		o.Workers = max(1, runtime.GOMAXPROCS(0)-1)
	}
	return o
}

// Stat summarises a sample.
type Stat struct {
	Mean float64 `json:"mean"`
	SD   float64 `json:"sd"`
	// SE is the standard error of the mean; Lo95 and Hi95 its 95 per cent
	// interval.
	SE   float64 `json:"se"`
	Lo95 float64 `json:"lo95"`
	Hi95 float64 `json:"hi95"`
	P5   float64 `json:"p5"`
	P10  float64 `json:"p10"`
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	Min  float64 `json:"min"`
	Max  float64 `json:"max"`
}

// Summarise is the statistics of a sample.
func Summarise(xs []float64) Stat {
	n := len(xs)
	if n == 0 {
		return Stat{}
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	var sum float64
	for _, x := range s {
		sum += x
	}
	st := Stat{Mean: sum / float64(n), Min: s[0], Max: s[n-1]}
	if n > 1 {
		var ss float64
		for _, x := range s {
			ss += (x - st.Mean) * (x - st.Mean)
		}
		st.SD = math.Sqrt(ss / float64(n-1))
		st.SE = st.SD / math.Sqrt(float64(n))
	}
	st.Lo95, st.Hi95 = st.Mean-1.96*st.SE, st.Mean+1.96*st.SE
	q := func(p float64) float64 {
		// Linear between order statistics.
		x := p * float64(n-1)
		i := int(x)
		if i >= n-1 {
			return s[n-1]
		}
		return s[i] + (x-float64(i))*(s[i+1]-s[i])
	}
	st.P5, st.P10, st.P50, st.P90, st.P95 = q(0.05), q(0.10), q(0.5), q(0.90), q(0.95)
	return st
}

// Bin is one bar of a histogram.
type Bin struct {
	Lo float64 `json:"lo"`
	Hi float64 `json:"hi"`
	N  int     `json:"n"`
}

func histogram(xs []float64, bins int) []Bin {
	if len(xs) == 0 {
		return nil
	}
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	if hi == lo {
		return []Bin{{Lo: lo, Hi: hi, N: len(xs)}}
	}
	w := (hi - lo) / float64(bins)
	out := make([]Bin, bins)
	for i := range out {
		out[i] = Bin{Lo: lo + float64(i)*w, Hi: lo + float64(i+1)*w}
	}
	for _, x := range xs {
		i := min(bins-1, int((x-lo)/w))
		out[i].N++
	}
	return out
}

// Result is a projection: a scenario played under a policy many times.
type Result struct {
	Carrier  string `json:"carrier"`
	Scenario string `json:"scenario"`
	Policy   string `json:"policy"`
	Method   string `json:"method"`
	Trials   int    `json:"trials"`
	// Baseline is the calm day under the same policy's absence: no events.
	Baseline Impact `json:"baseline"`
	// Cost is what the disruption costs: each play's total less the calm
	// day's, in cents. Revenue is the revenue lost alone.
	Cost    Stat `json:"cost"`
	Revenue Stat `json:"revenue_lost"`
	// Delta is the cost against the first policy compared, play by play
	// on the same draws; zero for the first.
	Delta     Stat            `json:"delta"`
	Metrics   map[string]Stat `json:"metrics"`
	Histogram []Bin           `json:"histogram"`
	// Cancels are the flights the policy cancels before the day.
	Cancels   int            `json:"cancels"`
	Standby   map[string]int `json:"standby,omitempty"`
	ElapsedMS int64          `json:"elapsed_ms"`
	costs     []float64
}

// draws are the uniforms of every trial: independent for Monte Carlo,
// stratified per dimension for a Latin hypercube.
func draws(o Options, d int) [][]float64 {
	out := make([][]float64, o.Trials)
	if o.Method == "lhs" {
		perms := make([][]int, d)
		for j := 0; j < d; j++ {
			r := rand.New(rand.NewPCG(o.Seed, uint64(j)+0x1000))
			perms[j] = r.Perm(o.Trials)
		}
		for t := range out {
			r := rand.New(rand.NewPCG(o.Seed, uint64(t)))
			u := make([]float64, d)
			for j := 0; j < d; j++ {
				u[j] = (float64(perms[j][t]) + r.Float64()) / float64(o.Trials)
			}
			out[t] = u
		}
		return out
	}
	for t := range out {
		r := rand.New(rand.NewPCG(o.Seed, uint64(t)))
		u := make([]float64, d)
		for j := range u {
			u[j] = r.Float64()
		}
		out[t] = u
	}
	return out
}

// Compare plays a scenario under each policy on the same draws -- common
// random numbers -- so the differences between policies are the
// policies', not the dice's. The first policy is the reference for Delta.
func (m *Model) Compare(sc Scenario, pols []Policy, o Options) ([]Result, error) {
	if err := m.Prepare(); err != nil {
		return nil, err
	}
	o = o.norm()
	us := draws(o, sc.Dims())
	baselines := m.baselines(o)
	out := make([]Result, len(pols))
	for i, p := range pols {
		start := time.Now()
		d := p.Decide(m, sc)
		impacts := make([]Impact, o.Trials)
		var wg sync.WaitGroup
		next := make(chan int)
		for w := 0; w < o.Workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for t := range next {
					impacts[t] = m.Play(m.Realise(sc, us[t], uint64(t)), d)
				}
			}()
		}
		for t := 0; t < o.Trials; t++ {
			next <- t
		}
		close(next)
		wg.Wait()
		res := Result{Carrier: m.Carrier, Scenario: sc.Name, Policy: p.Name, Method: o.Method, Trials: o.Trials,
			Baseline: baselines(0), Cancels: len(d.Cancel), Standby: d.Standby, Metrics: map[string]Stat{}}
		cols := map[string][]float64{}
		res.costs = make([]float64, o.Trials)
		revenue := make([]float64, o.Trials)
		for t, im := range impacts {
			res.costs[t] = float64(im.Total - baselines(t).Total)
			revenue[t] = float64(im.RevenueLost - baselines(t).RevenueLost)
			for k, v := range im.Metrics() {
				cols[k] = append(cols[k], v)
			}
		}
		for k, v := range cols {
			res.Metrics[k] = Summarise(v)
		}
		res.Cost, res.Revenue = Summarise(res.costs), Summarise(revenue)
		res.Histogram = histogram(res.costs, 20)
		if i > 0 {
			delta := make([]float64, o.Trials)
			for t := range delta {
				delta[t] = res.costs[t] - out[0].costs[t]
			}
			res.Delta = Summarise(delta)
		}
		res.ElapsedMS = time.Since(start).Milliseconds()
		out[i] = res
	}
	return out, nil
}

// Run plays a scenario under one policy.
func (m *Model) Run(sc Scenario, p Policy, o Options) (Result, error) {
	rs, err := m.Compare(sc, []Policy{p}, o)
	if err != nil {
		return Result{}, err
	}
	return rs[0], nil
}

// baselines is the calm day each trial is measured against: one day when
// own delays are the world's model, a day per trial when they are drawn
// from history, on the trial's own draws.
func (m *Model) baselines(o Options) func(t int) Impact {
	if len(m.History) == 0 {
		b := m.Play(Realisation{}, Decided{})
		return func(int) Impact { return b }
	}
	bs := make([]Impact, o.Trials)
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < o.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range next {
				bs[t] = m.Play(Realisation{Trial: uint64(t)}, Decided{})
			}
		}()
	}
	for t := 0; t < o.Trials; t++ {
		next <- t
	}
	close(next)
	wg.Wait()
	return func(t int) Impact { return bs[t] }
}

// Swing is one uncertainty's reach: the cost with it at its tenth
// percentile and at its ninetieth, everything else at the median.
type Swing struct {
	Event  string  `json:"event"`
	Factor string  `json:"factor"`
	Low    float64 `json:"low"`
	High   float64 `json:"high"`
	Range  float64 `json:"range"`
}

// Sensitivity swings each uncertain dimension alone and sorts the swings
// by their reach: the tornado, which says what the answer turns on.
func (m *Model) Sensitivity(sc Scenario, p Policy) ([]Swing, error) {
	if err := m.Prepare(); err != nil {
		return nil, err
	}
	d := p.Decide(m, sc)
	base := m.Play(Realisation{}, Decided{}).Total
	mid := make([]float64, sc.Dims())
	for i := range mid {
		mid[i] = 0.5
	}
	var out []Swing
	names := []string{"occurrence", "onset", "duration", "size"}
	for i, e := range sc.Events {
		for k := 0; k < dims; k++ {
			if !e.uncertain(k) {
				continue
			}
			at := func(q float64) float64 {
				u := append([]float64(nil), mid...)
				u[i*dims+k] = q
				return float64(m.Play(m.Realise(sc, u, 0), d).Total - base)
			}
			lo, hi := at(0.1), at(0.9)
			if math.Abs(hi-lo) < 100 {
				continue // under a dollar: this uncertainty does not matter
			}
			label := e.Kind + " " + e.Airport
			if e.Name != "" {
				label = e.Name
			}
			out = append(out, Swing{Event: label, Factor: names[k], Low: lo, High: hi, Range: math.Abs(hi - lo)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Range > out[j].Range })
	return out, nil
}

// uncertain says a dimension of the event has spread.
func (e Event) uncertain(k int) bool {
	switch k {
	case 0:
		return e.Probability > 0 && e.Probability < 1
	case 1:
		return e.Onset.Uncertain()
	case 2:
		return e.Duration.Uncertain()
	}
	if e.Kind == KindWeather {
		return e.Factor.Uncertain()
	}
	return e.Rate.Uncertain()
}

// Stress is a deterministic case: every uncertainty at the same quantile
// of its adverse direction -- the event happens, runs long, cuts deep --
// with the onset at its median. Stress(0.5) is the expected case; 0.9
// and 0.99 are the bad and the severe day a planner names.
func (m *Model) Stress(sc Scenario, p Policy, q float64) (Impact, error) {
	if err := m.Prepare(); err != nil {
		return Impact{}, err
	}
	u := make([]float64, sc.Dims())
	for i, e := range sc.Events {
		v := u[i*dims : (i+1)*dims]
		v[0] = 1 - q // a low draw is the event happening
		v[1] = 0.5
		v[2] = q
		if e.Kind == KindWeather {
			v[3] = 1 - q // a low factor is the worse weather
		} else {
			v[3] = q
		}
	}
	u[len(u)-1] = 0.5
	return m.Play(m.Realise(sc, u, 0), p.Decide(m, sc)), nil
}
