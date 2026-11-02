package noc

import (
	"fmt"
	"math"
	"sort"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// Policy is how the operations centre meets a disruption.
type Policy struct {
	Name string `json:"name"`
	// Thin is the share of the carrier's flights into the disruption, in
	// its window, cancelled ahead of time with the aircraft's turn back
	// out: the slots left go to the flights that still operate, and the
	// crews and aircraft stay where tomorrow needs them.
	Thin float64 `json:"thin,omitempty"`
	// Standby is how many standby crews to bring in at each crew base the
	// disruption touches, or at the hub when it touches none.
	Standby int `json:"standby,omitempty"`
	// Target, when set, names the one event the policy acts on: the
	// weather system a desk is deciding about. The other events are played
	// and left to their own decisions.
	Target string `json:"target,omitempty"`
}

// Policies are the desk's usual choices.
func Policies() []Policy {
	return []Policy{
		{Name: "absorb"},
		{Name: "thin 15%", Thin: 0.15},
		{Name: "thin 30%", Thin: 0.30},
		{Name: "standby crews", Standby: 2},
	}
}

// PolicyByName finds one of the usual policies.
func PolicyByName(name string) (Policy, bool) {
	for _, p := range Policies() {
		if p.Name == name {
			return p, true
		}
	}
	return Policy{}, false
}

// Cancellations are the flights the policy cancels ahead of the day,
// chosen once against the forecast's median: the desk thins against what
// it expects, not what will happen. An event less likely than not is not
// thinned for. The cheapest inbound legs go first, each with the same
// aircraft's next departure out of the airport within three hours.
func (p Policy) Cancellations(m *Model, sc Scenario) map[string]string {
	out := map[string]string{}
	if p.Thin <= 0 {
		return out
	}
	if err := m.Prepare(); err != nil {
		return out
	}
	if p.Target != "" {
		// Only this decision's event is thinned for.
		only := Scenario{Name: sc.Name}
		for _, e := range sc.Events {
			if e.Name == p.Target {
				only.Events = append(only.Events, e)
			}
		}
		sc = only
	}
	u := make([]float64, sc.Dims())
	for i := range u {
		u[i] = 0.5
	}
	r := m.Realise(sc, u, 0)
	type window struct {
		airport    string
		start, end int
	}
	var ws []window
	for _, c := range r.Cells {
		for _, a := range placed(m, c) {
			ws = append(ws, window{a, c.Start, c.End})
		}
	}
	for _, s := range r.Stops {
		ws = append(ws, window{s.Airport, s.Start, s.End})
	}
	revenue := func(f world.Flight) int64 {
		if m.Revenue != nil {
			return m.Revenue(f)
		}
		return int64(f.Seats)
	}
	byTail := map[string][]world.Flight{}
	for _, f := range m.flights {
		if f.Tail != "" {
			byTail[f.Tail] = append(byTail[f.Tail], f)
		}
	}
	for _, legs := range byTail {
		sort.Slice(legs, func(i, j int) bool { return legs[i].DepMin < legs[j].DepMin })
	}
	for _, w := range ws {
		var in []world.Flight
		for _, f := range m.flights {
			if f.To == w.airport && f.ArrMin >= w.start && f.ArrMin < w.end {
				in = append(in, f)
			}
		}
		sort.Slice(in, func(i, j int) bool {
			ri, rj := revenue(in[i]), revenue(in[j])
			if ri != rj {
				return ri < rj
			}
			return dayplan.Key(in[i]) < dayplan.Key(in[j])
		})
		n := int(math.Ceil(float64(len(in)) * p.Thin))
		reason := fmt.Sprintf("noc: thinned ahead of the disruption at %s", w.airport)
		for _, f := range in[:min(n, len(in))] {
			out[dayplan.Key(f)] = reason
			for _, g := range byTail[f.Tail] {
				if g.From == w.airport && g.DepMin > f.ArrMin && g.DepMin <= f.ArrMin+180 {
					out[dayplan.Key(g)] = reason
					break
				}
			}
		}
	}
	return out
}

// placed is the airports a cell covers.
func placed(m *Model, c dayplan.Cell) []string {
	if len(c.Airports) > 0 {
		return c.Airports
	}
	var out []string
	for _, a := range m.World.Airports {
		if km(c.Lat, c.Lon, a.Lat, a.Lon) <= c.RadiusKM {
			out = append(out, a.IATA)
		}
	}
	return out
}

// Decided is a policy turned into acts against a scenario's forecast:
// the flights it cancels and the standby crews it brings in, where.
type Decided struct {
	Policy  Policy            `json:"policy"`
	Cancel  map[string]string `json:"-"`
	Standby map[string]int    `json:"standby,omitempty"`
}

// Decide turns the policy into acts once, before the day: an operations
// centre buys standby for every event it is told of, likely or not,
// because a crew it does not need costs less than a flight it cancels.
func (p Policy) Decide(m *Model, sc Scenario) Decided {
	d := Decided{Policy: p, Cancel: p.Cancellations(m, sc), Standby: map[string]int{}}
	if p.Standby <= 0 || m.Roster == nil {
		return d
	}
	roster := m.Roster.Roster(m.Carrier)
	if roster == nil {
		return d
	}
	for _, e := range sc.Events {
		if p.Target != "" && e.Name != p.Target {
			continue
		}
		if e.Airport != "" && roster.IsBase(e.Airport) {
			d.Standby[e.Airport] = p.Standby
		}
	}
	if len(d.Standby) == 0 {
		d.Standby[roster.Hub] = p.Standby
	}
	return d
}
