package dayplan

import (
	"fmt"
	"sort"

	"github.com/adamf/jetway/pkg/atfm"
	"github.com/adamf/jetway/pkg/crew"
	"github.com/adamf/wholesky/internal/world"
)

// Options change how a day is built: a scenario's weather and ground stops
// in place of the drawn ones, flights cancelled before the day runs, and
// the crew roster that staffs the duties.
type Options struct {
	// Weather, when not nil, is the day's weather. A cell with no airports
	// listed covers every airport with arrivals inside its radius.
	Weather []Cell
	// Calm builds the day with no weather at all: the baseline a
	// scenario's cost is measured against.
	Calm bool
	// GroundStops hold departures to an airport for a window.
	GroundStops []GroundStop
	// Cancel are flights cancelled before the day runs, by Key, with the
	// reason: an operations centre thinning the schedule ahead of a
	// storm. A flight cancelled before the day takes no slot, so the
	// flights that still fly wait less.
	Cancel map[string]string
	// Staff is the crew roster.
	Staff Staff
}

// GroundStop holds every departure to an airport from Start to End:
// nothing leaves for it until the stop lifts, then departures resume at
// the airport's normal arrival rate. Departures also holds the flights
// leaving the airport, which is a closure. Cause is the regulation's
// reason, weather unless "ATC".
type GroundStop struct {
	Airport    string `json:"airport"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Departures bool   `json:"departures,omitempty"`
	Cause      string `json:"cause,omitempty"`
	Regulation string `json:"regulation,omitempty"`
}

// Staff is a carrier's crew roster as the plan sees it: who flies each
// duty, whether they may fly it as the day runs, and whether a reserve
// answers when they may not.
type Staff interface {
	// Assign names a crew for a duty as scheduled. False says no legal
	// crew could be found, with the reason; the duty's legs are then
	// cancelled for crew.
	Assign(d DutyRef) (ok bool, reason string)
	// Check is the duty at its times on the day, for the crew that flies
	// it: Table B, and the limits of the crew's own history.
	Check(d DutyRef) crew.Verdict
	// Reserve asks for a reserve crew at the station to fly the duty's
	// remaining legs, which d carries with the callout's delay. True when
	// one answers.
	Reserve(d DutyRef, station string) bool
	// Release forgets every assignment and callout on the tail, before the
	// plan works the tail's day again.
	Release(carrier, tail string)
}

// DutyRef is one duty as the plan hands it to the roster.
type DutyRef struct {
	Carrier, Tail string
	// N is the duty's number in the tail's day; Relief counts the reserve
	// crews that have taken it over, 0 for the crew scheduled to it.
	N, Relief int
	// Duty is the legs with their times: scheduled for Assign, as the day
	// has them for Check and Reserve. Cancelled legs are left out of the
	// day's version.
	Duty      crew.Duty
	LongHaul  bool
	Equipment string
	Seats     int
	// From is where the duty starts and To where its last leg lands.
	From, To string
	// Keys are the plan's keys of every leg of the duty, cancelled or not.
	Keys []string
}

// ID names the duty on the tail, whoever flies it.
func (d DutyRef) ID() string { return fmt.Sprintf("%s/%s/%d", d.Carrier, d.Tail, d.N) }

// placeCells gives each cell that lists no airports the airports with
// arrivals inside its radius.
func placeCells(m *world.Manifest, cells []Cell) []Cell {
	arrivals := map[string]bool{}
	for _, f := range m.Flights {
		arrivals[f.To] = true
	}
	out := make([]Cell, len(cells))
	for i, c := range cells {
		if len(c.Airports) == 0 {
			for _, a := range m.Airports {
				if arrivals[a.IATA] && distKM(c.Lat, c.Lon, a.Lat, a.Lon) <= c.RadiusKM {
					c.Airports = append(c.Airports, a.IATA)
				}
			}
			sort.Strings(c.Airports)
		}
		out[i] = c
	}
	return out
}

// groundStops holds the departures to each stopped airport: a flight due
// to leave inside the window waits for the end, and the flights released
// together leave at the airport's normal rate. A flight that has left by
// `now` keeps its fate.
func (p *Plan) groundStops(m *world.Manifest, stops []GroundStop, now int) {
	if len(stops) == 0 {
		return
	}
	icao, country := map[string]string{}, map[string]string{}
	for _, a := range m.Airports {
		icao[a.IATA] = a.ICAO
		country[a.IATA] = a.Country
	}
	if p.released == nil {
		p.released = map[string]bool{}
	}
	for _, gs := range stops {
		var held []world.Flight
		perHour := map[int]int{}
		for _, f := range m.Flights {
			if f.To != gs.Airport && !(gs.Departures && f.From == gs.Airport) {
				continue
			}
			perHour[(f.ArrMin/60)%24]++
			pf := p.Flights[Key(f)]
			if pf == nil || pf.Cancelled || f.Actual != nil {
				continue
			}
			if f.DepMin >= gs.Start && f.DepMin < gs.End {
				held = append(held, f)
			}
		}
		normal := 1
		for _, n := range perHour {
			normal = max(normal, n)
		}
		sort.Slice(held, func(i, j int) bool {
			if held[i].DepMin != held[j].DepMin {
				return held[i].DepMin < held[j].DepMin
			}
			return Key(held[i]) < Key(held[j])
		})
		cause := atfm.NewCause(atfm.CauseWeather, 'A')
		if gs.Cause == "ATC" {
			cause = atfm.NewCause(atfm.CauseATCCapacity, 'A')
		}
		loc := icao[gs.Airport]
		if loc == "" {
			loc = gs.Airport
		}
		name := gs.Regulation
		if name == "" {
			name = fmt.Sprintf("%sGS%02d", loc, p.Day.Day())
		}
		reg := Regulation{Name: name, Airport: gs.Airport, Start: gs.Start, End: gs.End, Rate: 0, NormalRate: normal, Cause: cause, CauseText: cause.String(), Mode: modeFor(country[gs.Airport])}
		// The stop lifts at its end and the held departures leave in turn,
		// one slot each at the airport's normal rate. The list is kept so
		// that a held flight cancelled lets the ones behind it leave
		// sooner.
		book := newSlotBook(float64(gs.End), 60.0/float64(normal), true)
		var placed []world.Flight
		for _, f := range held {
			key := Key(f)
			pf := p.Flights[key]
			if p.released[key] || (now >= 0 && f.DepMin+pf.DepDelay <= now) {
				continue
			}
			at := book.take(f, float64(f.DepMin))
			if d := int(at) - f.DepMin; d > 0 {
				book.delay[key] = d
			}
			placed = append(placed, f)
		}
		reg.book = book
		reg.Flights, reg.DelayMin = book.counts()
		if reg.Flights == 0 {
			continue
		}
		p.Regulations = append(p.Regulations, reg)
		p.Summary.Regulations++
		for _, f := range placed {
			if !p.settle(f) {
				continue
			}
			pf := p.Flights[Key(f)]
			pf.DepDelay = max(pf.DepDelay, pf.ATFM)
			pf.ArrDelay = max(pf.ArrDelay, pf.ATFM)
		}
	}
	p.recount()
}

// Picture is the day's weather, regulations and summary, copied under the
// lock: cells and stops can be added while the day runs.
func (p *Plan) Picture() ([]Cell, []Regulation, Summary) {
	if p == nil {
		return nil, nil, Summary{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]Cell(nil), p.Weather...), append([]Regulation(nil), p.Regulations...), p.Summary
}

// Inject adds weather and ground stops to a day already running: the
// arrivals still on the ground are slotted, the stopped departures held,
// and every tail they touch worked again down the day -- late aircraft,
// crews, reserves. now is the day's position in minutes; nothing that has
// left changes. It returns the flights whose fate changed.
func (p *Plan) Inject(m *world.Manifest, cells []Cell, stops []GroundStop, now int) []world.Flight {
	if p == nil || p.Replay {
		return nil
	}
	cells = placeCells(m, cells)
	p.mu.Lock()
	before := map[string]Flight{}
	for k, pf := range p.Flights {
		before[k] = *pf
	}
	p.Weather = append(p.Weather, cells...)
	p.Summary.Cells = len(p.Weather)
	p.regulate(m, cells, now)
	p.groundStops(m, stops, now)
	touched := map[string]world.Flight{}
	for _, f := range m.Flights {
		if b, ok := before[Key(f)]; ok && b.ATFM != p.Flights[Key(f)].ATFM && f.Tail != "" {
			touched[f.Carrier+"/"+f.Tail] = f
		}
	}
	p.mu.Unlock()
	keys := make([]string, 0, len(touched))
	for k := range touched {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f := touched[k]
		p.Replan(m, f.Carrier, f.Tail)
	}
	var changed []world.Flight
	p.mu.RLock()
	for _, f := range m.Flights {
		b, ok := before[Key(f)]
		pf := p.Flights[Key(f)]
		if ok && pf != nil && (b.DepDelay != pf.DepDelay || b.Cancelled != pf.Cancelled || b.Reserve != pf.Reserve) {
			changed = append(changed, f)
		}
	}
	p.mu.RUnlock()
	return changed
}

// Rebuild works the day out again from the schedule, as it was first
// built, for a new day of the same schedule: the weather and slots drawn
// for the date, and crews staffed afresh from the roster, whose records
// have moved on a day. What was added to yesterday -- a seat's
// cancellation, weather injected, a reserve called by hand -- is gone
// with it.
func (p *Plan) Rebuild(m *world.Manifest, own Own) {
	if p == nil {
		return
	}
	fresh := BuildWith(m, p.Day, own, Options{Staff: p.Staff})
	p.mu.Lock()
	p.Flights, p.Weather, p.Regulations, p.Summary = fresh.Flights, fresh.Weather, fresh.Regulations, fresh.Summary
	p.released, p.ready = fresh.released, fresh.ready
	p.mu.Unlock()
}
