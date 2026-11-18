// Command skynoc projects what a disruption will cost a carrier, offline,
// against a compiled world: the scenario is played through the day plan
// and the carrier's crew roster many times, by Latin hypercube or plain
// Monte Carlo sampling, under each recovery policy, and the spread of the
// cost is printed with each policy's difference from absorbing the
// disruption. The scenario comes from the flags, a JSON file, the world's
// own weather for the day, or the real terminal forecasts of the
// carrier's busiest stations.
//
//	skynoc -world world.json -carrier UA -wx ORD:1400:240:0.45
//	skynoc -world world.json -carrier BA -taf -trials 1000 -sensitivity
//	skynoc -world world.json -carrier DL -sickout 0.08 -groundstop ATL:1600:90
//	skynoc -world world.json -carrier AA -wx DFW:1300:300:0.5:0.4 -history replay.json
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/noc"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/workforce"
	"github.com/adamf/wholesky/internal/world"
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, " ") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	var wx, stops, closures multi
	var (
		worldPath = flag.String("world", "world.json", "compiled world")
		dataDir   = flag.String("data", "data", "the OpenFlights data, for the ICAO codes of a world compiled without them")
		carrier   = flag.String("carrier", "", "the carrier's designator")
		scenario  = flag.String("scenario", "", "a scenario as JSON (see docs/noc.md)")
		taf       = flag.Bool("taf", false, "the real terminal forecasts of the carrier's ten busiest stations, from aviationweather.gov")
		dayWx     = flag.Bool("day-weather", false, "the world's own weather for the day, as a forecast")
		sickout   = flag.Float64("sickout", 0, "a share of the carrier's crews calling in sick")
		aog       = flag.Int("aog", 0, "aircraft going technical through the day")
		certain   = flag.Bool("certain", false, "play the events exactly as given, without a forecast's uncertainty")
		trials    = flag.Int("trials", 500, "plays of the day per policy")
		method    = flag.String("method", "lhs", "sampling: lhs (Latin hypercube) or mc (Monte Carlo)")
		seed      = flag.Uint64("seed", 1, "seed")
		workers   = flag.Int("workers", 0, "plays at once; 0 is the CPUs less one")
		policies  = flag.String("policies", "absorb,thin 15%,thin 30%,standby crews", "policies to compare, the first the reference")
		history   = flag.String("history", "", "a compiled replay world: own delays are drawn from its recorded day")
		noCrew    = flag.Bool("no-crew", false, "play without a roster: any crew, reserves at the hub, no crew costs")
		sens      = flag.Bool("sensitivity", false, "swing each uncertainty alone and print the tornado and the stress cases")
		asJSON    = flag.Bool("json", false, "print the full results as JSON")
	)
	flag.Var(&wx, "wx", "weather AIRPORT:HHMM:MINUTES:RATE[:PROBABILITY], repeatable")
	flag.Var(&stops, "groundstop", "a ground stop AIRPORT:HHMM:MINUTES, repeatable")
	flag.Var(&closures, "closure", "a closure both ways AIRPORT:HHMM:MINUTES, repeatable")
	flag.Parse()
	if *carrier == "" {
		log.Fatal("name a carrier with -carrier")
	}
	m, err := load(*worldPath)
	if err != nil {
		log.Fatal(err)
	}
	world.FillICAO(m, *dataDir) //nolint:errcheck // an old world without the data only loses -taf
	code := strings.ToUpper(*carrier)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	if m.Replay != nil {
		log.Fatal("project against a synthetic world; pass a replay world as -history")
	}
	md := &noc.Model{World: m, Carrier: code, Day: day, Own: synthOwn}
	t := tariff.FromManifest(m)
	md.Fare = func(f world.Flight) int64 { return tariff.AverageFare(t, f.Carrier, f.From, f.To) }
	if !*noCrew {
		md.Roster = workforce.New(m, day, func(c string) bool { return c == code })
	}
	if *history != "" {
		h, err := load(*history)
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range h.Flights {
			if f.Actual != nil && !f.Actual.Cancelled {
				md.History = append(md.History, f.Actual.DepDelay)
			}
		}
		if len(md.History) == 0 {
			log.Fatal("the history world records no delays")
		}
	}
	if err := md.Prepare(); err != nil {
		log.Fatal(err)
	}

	sc := noc.Scenario{Name: "scenario"}
	if *scenario != "" {
		b, err := os.ReadFile(*scenario)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(b, &sc); err != nil {
			log.Fatal(err)
		}
	}
	for _, s := range wx {
		e, err := parseEvent(noc.KindWeather, s, *certain)
		if err != nil {
			log.Fatal(err)
		}
		sc.Events = append(sc.Events, e)
	}
	for _, s := range stops {
		e, err := parseEvent(noc.KindGroundStop, s, *certain)
		if err != nil {
			log.Fatal(err)
		}
		sc.Events = append(sc.Events, e)
	}
	for _, s := range closures {
		e, err := parseEvent(noc.KindClosure, s, *certain)
		if err != nil {
			log.Fatal(err)
		}
		sc.Events = append(sc.Events, e)
	}
	if *sickout > 0 {
		r := noc.Tri(*sickout*0.6, *sickout, *sickout*1.5)
		if *certain {
			r = noc.Fixed(*sickout)
		}
		sc.Events = append(sc.Events, noc.Event{Kind: noc.KindSickout, Onset: noc.Fixed(0), Duration: noc.Fixed(1440), Rate: r})
	}
	if *aog > 0 {
		r := noc.Tri(0, float64(*aog), float64(*aog)*2)
		if *certain {
			r = noc.Fixed(float64(*aog))
		}
		sc.Events = append(sc.Events, noc.Event{Kind: noc.KindAOG, Onset: noc.Fixed(300), Duration: noc.Fixed(1080), Rate: r})
	}
	if *dayWx {
		plan := dayplan.Build(m, day, synthOwn)
		cells, _, _ := plan.Picture()
		for _, c := range cells {
			d := float64(c.End - c.Start)
			sc.Events = append(sc.Events, noc.Event{Kind: noc.KindWeather, Name: c.Name, Airport: c.Name[:3], RadiusKM: c.RadiusKM,
				Onset: noc.Normal(float64(c.Start), 45), Duration: noc.Tri(0.7*d, d, 1.3*d), Factor: noc.Tri(c.Factor-0.1, c.Factor, min(1, c.Factor+0.1))})
		}
	}
	if *taf {
		icao, byICAO := busiest(m, code, 10)
		if len(icao) == 0 {
			log.Fatal("the world names no ICAO codes for the carrier's stations; pass -data or recompile it")
		}
		tafs, err := noc.FetchTAFs(context.Background(), nil, icao)
		if err != nil {
			log.Fatal(err)
		}
		real := noc.FromTAFs(tafs, time.Now().UTC(), func(c string) string { return byICAO[c] })
		sc.Events = append(sc.Events, real.Events...)
		fmt.Fprintf(os.Stderr, "read %d forecasts for %s: %d groups cut a rate\n", len(tafs), strings.Join(icao, " "), len(real.Events))
	}

	var pols []noc.Policy
	for _, name := range strings.Split(*policies, ",") {
		p, ok := noc.PolicyByName(strings.TrimSpace(name))
		if !ok {
			log.Fatalf("no policy %q", name)
		}
		pols = append(pols, p)
	}
	o := noc.Options{Trials: *trials, Seed: *seed, Method: *method, Workers: *workers}
	start := time.Now()
	rs, err := md.Compare(sc, pols, o)
	if err != nil {
		log.Fatal(err)
	}
	out := map[string]any{"carrier": code, "scenario": sc, "results": rs}
	var swings []noc.Swing
	stress := map[string]noc.Impact{}
	if *sens && len(sc.Events) > 0 {
		swings, _ = md.Sensitivity(sc, pols[0])
		for name, q := range map[string]float64{"expected (P50)": 0.5, "bad (P90)": 0.9, "severe (P99)": 0.99} {
			stress[name], _ = md.Stress(sc, pols[0], q)
		}
		out["swings"], out["stress"] = swings, stress
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out) //nolint:errcheck
		return
	}
	fmt.Printf("%s, %d flights. %s\n", code, rs[0].Baseline.Flights, describe(sc))
	fmt.Printf("%d plays per policy, %s, %s. A calm day costs %s in crews, ground staff and delay.\n\n", o.Trials, methodName(*method), time.Since(start).Round(time.Millisecond), money(float64(rs[0].Baseline.Total)))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "policy\tcancelled ahead\texpected\tP50\tP95\tvs "+pols[0].Name+" (95%)\tcancelled\tdelay min\tmissed conx\tcrew\trevenue lost\t")
	for i, r := range rs {
		vs := "-"
		if i > 0 {
			vs = fmt.Sprintf("%s ± %s", money(r.Delta.Mean), money(1.96*r.Delta.SE))
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%.1f\t%.0f\t%.0f\t%s\t%s\t\n", r.Policy, r.Cancels, money(r.Cost.Mean), money(r.Cost.P50), money(r.Cost.P95), vs,
			r.Metrics["cancelled"].Mean, r.Metrics["delay_min"].Mean, r.Metrics["pax_misconnected"].Mean,
			money(r.Metrics["crew"].Mean+r.Metrics["crew_premium"].Mean), money(r.Metrics["revenue_lost"].Mean))
	}
	w.Flush()
	best := rs[0]
	for _, r := range rs {
		if r.Cost.Mean < best.Cost.Mean {
			best = r
		}
	}
	fmt.Printf("\nLowest expected cost: %s. Standard error of its mean: %s.\n", best.Policy, money(best.Cost.SE))
	if len(swings) > 0 {
		fmt.Printf("\nWhat the cost turns on (%s, each uncertainty alone at its 10th and 90th percentiles):\n", pols[0].Name)
		for _, s := range swings {
			fmt.Printf("  %-28s %-10s %10s to %10s\n", s.Event, s.Factor, money(s.Low), money(s.High))
		}
		names := make([]string, 0, len(stress))
		for k := range stress {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Println("\nStress cases, every uncertainty at the same adverse quantile:")
		for _, k := range names {
			im := stress[k]
			fmt.Printf("  %-16s cost %10s, %d cancelled, %d delay minutes, %d crew timeouts\n", k, money(float64(im.Total-rs[0].Baseline.Total)), im.Cancelled, im.DelayMin, im.Timeouts)
		}
	}
}

func load(path string) (*world.Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m world.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// synthOwn is no own delay: the offline tool plays the disruption against
// a punctual day unless a recorded day's delays are drawn with -history.
func synthOwn(world.Flight, time.Time) (int, int) { return 0, 0 }

// parseEvent reads AIRPORT:HHMM:MINUTES[:RATE[:PROBABILITY]].
func parseEvent(kind, s string, certain bool) (noc.Event, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 3 {
		return noc.Event{}, fmt.Errorf("%q: want AIRPORT:HHMM:MINUTES", s)
	}
	hhmm, err := strconv.Atoi(parts[1])
	if err != nil {
		return noc.Event{}, fmt.Errorf("%q: start %q", s, parts[1])
	}
	start := float64(hhmm/100*60 + hhmm%100)
	dur, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return noc.Event{}, fmt.Errorf("%q: minutes %q", s, parts[2])
	}
	e := noc.Event{Kind: kind, Airport: strings.ToUpper(parts[0])}
	if certain {
		e.Onset, e.Duration = noc.Fixed(start), noc.Fixed(dur)
	} else {
		e.Onset, e.Duration = noc.Tri(start-45, start, start+45), noc.Tri(0.7*dur, dur, 1.3*dur)
	}
	if kind == noc.KindWeather {
		f := 0.5
		if len(parts) > 3 {
			if f, err = strconv.ParseFloat(parts[3], 64); err != nil {
				return noc.Event{}, fmt.Errorf("%q: rate %q", s, parts[3])
			}
		}
		e.Factor = noc.Tri(max(0.05, f-0.1), f, min(1, f+0.1))
		if certain {
			e.Factor = noc.Fixed(f)
		}
		if len(parts) > 4 {
			if e.Probability, err = strconv.ParseFloat(parts[4], 64); err != nil {
				return noc.Event{}, fmt.Errorf("%q: probability %q", s, parts[4])
			}
		}
	}
	return e, nil
}

// busiest are the carrier's busiest stations by ICAO, and the map back.
func busiest(m *world.Manifest, code string, n int) ([]string, map[string]string) {
	count := map[string]int{}
	for _, f := range m.Flights {
		if f.Carrier == code {
			count[f.From]++
			count[f.To]++
		}
	}
	type st struct {
		iata string
		n    int
	}
	var list []st
	for k, v := range count {
		list = append(list, st{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	icaoOf := map[string]string{}
	for _, a := range m.Airports {
		icaoOf[a.IATA] = a.ICAO
	}
	var out []string
	back := map[string]string{}
	for _, s := range list {
		if c := icaoOf[s.iata]; c != "" {
			out = append(out, c)
			back[c] = s.iata
		}
		if len(out) == n {
			break
		}
	}
	return out, back
}

func describe(sc noc.Scenario) string {
	if len(sc.Events) == 0 {
		return "No events: the calm day against itself."
	}
	return sc.Describe() + "."
}

func methodName(m string) string {
	if m == "mc" {
		return "Monte Carlo"
	}
	return "Latin hypercube"
}

func money(cents float64) string {
	d := cents / 100
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	switch {
	case d >= 1e6:
		return fmt.Sprintf("%s$%.2fM", sign, d/1e6)
	case d >= 1e4:
		return fmt.Sprintf("%s$%.0fk", sign, d/1e3)
	}
	return fmt.Sprintf("%s$%.0f", sign, d)
}
