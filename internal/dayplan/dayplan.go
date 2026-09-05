// Package dayplan decides how the day goes wrong before it begins, and for
// the right reasons. The synthetic delay model drew each flight's delay on
// its own; a real day's delays have causes that chain: weather closes half
// an airport's arrival rate and the Network Manager slots every arrival
// into what is left (a SAM with a calculated take-off time, a regulation
// cause of WA 84); the aircraft arrives late and its next departure waits
// for it (late aircraft, code 93); the crew that has flown it all day runs
// out of duty time under 14 CFR 117 and the last leg is cancelled unless a
// reserve crew is at the base. The plan computes all of that once from the
// schedule, deterministically from the day, so every machine in a
// federated world agrees on it and the simulation reads the fate of a
// flight instead of inventing it as it goes. On a recorded day the record
// is the plan: its delays stand, its NAS and weather minutes become the
// slots that explain them, and crew legality is reported, not enforced.
package dayplan

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/atfm"
	"github.com/adamf/jetway/pkg/crew"
	"github.com/adamf/wholesky/internal/world"
)

// Flight is one flight's fate.
type Flight struct {
	// DepDelay and ArrDelay are the minutes the flight will run late; the
	// three parts of the departure delay are the flight's own (the
	// carrier's), the slot's (ATFM) and the inbound aircraft's.
	DepDelay, ArrDelay      int
	Own, ATFM, LateAircraft int
	// CTOT is the calculated take-off time the slot gave, minutes into the
	// day, when the flight is regulated; Regulation and Cause name why.
	CTOT       int
	Regulation string
	Cause      *atfm.Cause
	// Cancelled says the plan cancels the flight; Reason says why and Code
	// is the record's letter (A carrier, B weather, C airspace).
	Cancelled bool
	Reason    string
	Code      string
	// Duty is which of the tail's crew duties flies the leg; Extended says
	// the crew needed the two-hour extension; Reserve says a reserve crew
	// was called at the base, which is what the extra delay is.
	Duty     int
	Extended bool
	Reserve  bool
}

// Cell is one weather system: a centre, a radius and a window during
// which the airports under it accept arrivals at a fraction of their rate.
type Cell struct {
	Name     string   `json:"name"`
	Lat      float64  `json:"lat"`
	Lon      float64  `json:"lon"`
	RadiusKM float64  `json:"radius_km"`
	Start    int      `json:"start"`
	End      int      `json:"end"`
	Factor   float64  `json:"factor"`
	Airports []string `json:"airports"`
}

// Regulation is one ATFM regulation: an airport's arrivals in a window
// held to a rate, and what it cost.
type Regulation struct {
	Name       string      `json:"name"`
	Airport    string      `json:"airport"`
	Start      int         `json:"start"`
	End        int         `json:"end"`
	Rate       int         `json:"rate"`
	NormalRate int         `json:"normal_rate"`
	Cause      *atfm.Cause `json:"-"`
	CauseText  string      `json:"cause"`
	Flights    int         `json:"flights"`
	DelayMin   int         `json:"delay_min"`
}

// Summary is the day's plan in numbers, for the instruments.
type Summary struct {
	Flights      int `json:"flights"`
	Cells        int `json:"cells"`
	Regulations  int `json:"regulations"`
	Slotted      int `json:"slotted"`
	SlotDelayMin int `json:"slot_delay_min"`
	LateAircraft int `json:"late_aircraft"`
	Duties       int `json:"duties"`
	Extended     int `json:"extended"`
	Reserves     int `json:"reserves"`
	TimedOut     int `json:"timed_out"`
	Cancelled    int `json:"cancelled"`
}

// Plan is the day.
type Plan struct {
	Day         time.Time
	Replay      bool
	Flights     map[string]*Flight
	Weather     []Cell
	Regulations []Regulation
	Summary     Summary
}

// Key names a flight the way the plan does: carrier, number and boarding
// point, because one number flies several legs a day.
func Key(f world.Flight) string { return f.Carrier + strings.TrimLeft(f.Number, "0") + "/" + f.From }

// Of is a flight's fate; the zero fate for one the plan never saw.
func (p *Plan) Of(f world.Flight) Flight {
	if p == nil {
		return Flight{}
	}
	if pf, ok := p.Flights[Key(f)]; ok {
		return *pf
	}
	return Flight{}
}

// Own is the flight's own delay model: what the carrier does to itself.
type Own func(f world.Flight, day time.Time) (dep, arr int)

// The plan's constants: the taxi time a CTOT stands off from off-block, the
// minimum turnaround a tail needs, the report lead and release a crew has,
// the reserve callout, and how far a slot may push a flight before the
// regulation would suspend it instead.
const (
	TaxiMin      = 15
	MinTurnMin   = 30
	ReportLead   = 60 * time.Minute
	Release      = 15 * time.Minute
	ReserveCall  = 90
	MaxSlotDelay = 180
)

// Build computes the day.
func Build(m *world.Manifest, day time.Time, own Own) *Plan {
	p := &Plan{Day: day, Replay: m.Replay != nil, Flights: map[string]*Flight{}}
	for _, f := range m.Flights {
		pf := &Flight{}
		if f.Actual != nil {
			pf.DepDelay, pf.ArrDelay = f.Actual.DepDelay, f.Actual.ArrDelay
			pf.Own = pf.DepDelay
			if f.Actual.Cancelled {
				pf.Cancelled, pf.Code, pf.Reason = true, f.Actual.CancelCode, "cancelled as recorded, code "+f.Actual.CancelCode
			}
		} else if own != nil {
			pf.Own, pf.ArrDelay = own(f, day)
			pf.DepDelay = pf.Own
		}
		p.Flights[Key(f)] = pf
	}
	p.Summary.Flights = len(m.Flights)
	if p.Replay {
		p.recordedSlots(m)
	} else {
		p.weather(m)
		p.regulate(m)
		p.rotate(m)
	}
	p.crews(m)
	for _, pf := range p.Flights {
		if pf.Cancelled {
			p.Summary.Cancelled++
		}
	}
	return p
}

// hash is the plan's deterministic draw.
func hash(parts ...string) uint64 {
	var h uint64 = 1469598103934665603
	for _, s := range parts {
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= 1099511628211
		}
		h ^= '/'
		h *= 1099511628211
	}
	return h
}

// weather draws the day's cells: two to four systems centred on busy
// airports, each a few hundred kilometres across for three to six hours,
// cutting arrival rates to between half and three quarters.
func (p *Plan) weather(m *world.Manifest) {
	arrivals := map[string]int{}
	for _, f := range m.Flights {
		arrivals[f.To]++
	}
	byIATA := map[string]world.Airport{}
	var busy []world.Airport
	for _, a := range m.Airports {
		byIATA[a.IATA] = a
		if arrivals[a.IATA] >= 20 {
			busy = append(busy, a)
		}
	}
	if len(busy) == 0 {
		return
	}
	sort.Slice(busy, func(i, j int) bool {
		if arrivals[busy[i].IATA] != arrivals[busy[j].IATA] {
			return arrivals[busy[i].IATA] > arrivals[busy[j].IATA]
		}
		return busy[i].IATA < busy[j].IATA
	})
	if len(busy) > 60 {
		busy = busy[:60]
	}
	dayKey := p.Day.Format("20060102")
	n := 2 + int(hash(dayKey, "cells")%3)
	used := map[string]bool{}
	for i := 0; i < n; i++ {
		h := hash(dayKey, "cell", fmt.Sprint(i))
		centre := busy[int(h%uint64(len(busy)))]
		if used[centre.IATA] {
			continue
		}
		used[centre.IATA] = true
		c := Cell{
			Name: fmt.Sprintf("%s%s", centre.IATA, string(rune('A'+i))), Lat: centre.Lat, Lon: centre.Lon,
			RadiusKM: 200 + float64(h>>8%300), Start: 6*60 + int(h>>16%(14*60)),
			Factor: 0.5 + float64(h>>24%26)/100,
		}
		c.End = c.Start + 180 + int(h>>32%180)
		for _, a := range m.Airports {
			if arrivals[a.IATA] > 0 && distKM(c.Lat, c.Lon, a.Lat, a.Lon) <= c.RadiusKM {
				c.Airports = append(c.Airports, a.IATA)
			}
		}
		sort.Strings(c.Airports)
		p.Weather = append(p.Weather, c)
	}
	p.Summary.Cells = len(p.Weather)
}

func distKM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	toRad := math.Pi / 180
	dLat := (lat2 - lat1) * toRad
	dLon := (lon2 - lon1) * toRad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*toRad)*math.Cos(lat2*toRad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// regulate runs the flow management over every airport under weather: its
// arrivals in the window are held to the reduced rate, first come first
// served by scheduled arrival, and each one held gets a slot -- a CTOT that
// is its off-block plus the wait plus the taxi -- with the weather cause.
func (p *Plan) regulate(m *world.Manifest) {
	icao := map[string]string{}
	for _, a := range m.Airports {
		icao[a.IATA] = a.ICAO
	}
	byFlight := map[string]world.Flight{}
	arrivalsAt := map[string][]world.Flight{}
	for _, f := range m.Flights {
		byFlight[Key(f)] = f
		arrivalsAt[f.To] = append(arrivalsAt[f.To], f)
	}
	for _, c := range p.Weather {
		for _, ap := range c.Airports {
			arr := arrivalsAt[ap]
			if len(arr) == 0 {
				continue
			}
			// The airport's normal rate is its busiest hour of arrivals.
			perHour := map[int]int{}
			for _, f := range arr {
				perHour[(f.ArrMin/60)%24]++
			}
			normal := 0
			for _, n := range perHour {
				normal = max(normal, n)
			}
			if normal < 4 {
				continue
			}
			rate := max(1, int(math.Floor(float64(normal)*c.Factor)))
			period := "M"
			switch {
			case c.Start >= 21*60 || c.Start < 5*60:
				period = "N"
			case c.Start >= 12*60:
				period = "A"
			case c.Start < 7*60:
				period = "E"
			}
			loc := icao[ap]
			if loc == "" {
				loc = ap
			}
			reg := Regulation{Name: fmt.Sprintf("%sA%02d%s", loc, p.Day.Day(), period), Airport: ap, Start: c.Start, End: c.End,
				Rate: rate, NormalRate: normal, Cause: atfm.NewCause(atfm.CauseWeather, 'A')}
			reg.CauseText = reg.Cause.String()
			var held []world.Flight
			for _, f := range arr {
				if f.Actual != nil {
					continue
				}
				if f.ArrMin >= c.Start && f.ArrMin < c.End {
					held = append(held, f)
				}
			}
			sort.Slice(held, func(i, j int) bool {
				if held[i].ArrMin != held[j].ArrMin {
					return held[i].ArrMin < held[j].ArrMin
				}
				return Key(held[i]) < Key(held[j])
			})
			interval := 60.0 / float64(rate)
			next := float64(c.Start)
			for _, f := range held {
				slot := math.Max(float64(f.ArrMin), next)
				next = slot + interval
				delay := int(math.Round(slot - float64(f.ArrMin)))
				if delay > MaxSlotDelay {
					delay = MaxSlotDelay
				}
				pf := p.Flights[Key(f)]
				if delay <= 0 || pf == nil {
					continue
				}
				if pf.ATFM >= delay {
					continue
				}
				pf.ATFM = delay
				pf.Regulation, pf.Cause = reg.Name, reg.Cause
				pf.CTOT = f.DepMin + delay + TaxiMin
				pf.DepDelay = max(pf.DepDelay, delay)
				pf.ArrDelay = max(pf.ArrDelay, delay)
				reg.Flights++
				reg.DelayMin += delay
			}
			if reg.Flights > 0 {
				p.Regulations = append(p.Regulations, reg)
				p.Summary.Regulations++
				p.Summary.Slotted += reg.Flights
				p.Summary.SlotDelayMin += reg.DelayMin
			}
		}
	}
}

// recordedSlots reads the record's causes back into slots: a flight the
// record delays for the national airspace or for weather was held by a
// regulation, and the plan says which kind.
func (p *Plan) recordedSlots(m *world.Manifest) {
	icao := map[string]string{}
	for _, a := range m.Airports {
		icao[a.IATA] = a.ICAO
	}
	regs := map[string]*Regulation{}
	for _, f := range m.Flights {
		if f.Actual == nil || f.Actual.Cancelled {
			continue
		}
		var cause *atfm.Cause
		mins := 0
		switch {
		case f.Actual.Weather >= 15 && f.Actual.Weather >= f.Actual.NAS:
			cause, mins = atfm.NewCause(atfm.CauseWeather, 'A'), f.Actual.Weather
		case f.Actual.NAS >= 15:
			cause, mins = atfm.NewCause(atfm.CauseATCCapacity, 'A'), f.Actual.NAS
		default:
			continue
		}
		loc := icao[f.To]
		if loc == "" {
			loc = f.To
		}
		period := "M"
		if f.ArrMin >= 12*60 {
			period = "A"
		}
		if f.ArrMin >= 21*60 {
			period = "N"
		}
		name := fmt.Sprintf("%sA%02d%s", loc, p.Day.Day(), period)
		reg := regs[name+cause.String()]
		if reg == nil {
			reg = &Regulation{Name: name, Airport: f.To, Start: (f.ArrMin / 360) * 360, End: (f.ArrMin/360)*360 + 360, Cause: cause, CauseText: cause.String()}
			regs[name+cause.String()] = reg
		}
		pf := p.Flights[Key(f)]
		pf.ATFM, pf.Regulation, pf.Cause = mins, name, cause
		pf.CTOT = f.DepMin + pf.DepDelay + TaxiMin
		reg.Flights++
		reg.DelayMin += mins
	}
	names := make([]string, 0, len(regs))
	for k := range regs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		p.Regulations = append(p.Regulations, *regs[k])
		p.Summary.Slotted += regs[k].Flights
		p.Summary.SlotDelayMin += regs[k].DelayMin
	}
	p.Summary.Regulations = len(p.Regulations)
}

// rotations groups a carrier's flights by tail in departure order.
func rotations(m *world.Manifest) map[string][]world.Flight {
	tails := map[string][]world.Flight{}
	for _, f := range m.Flights {
		if f.Tail == "" {
			continue
		}
		tails[f.Carrier+"/"+f.Tail] = append(tails[f.Carrier+"/"+f.Tail], f)
	}
	for k := range tails {
		legs := tails[k]
		sort.Slice(legs, func(i, j int) bool { return legs[i].DepMin < legs[j].DepMin })
		tails[k] = legs
	}
	return tails
}

// rotate passes a late aircraft's delay to its next leg: a departure
// cannot leave before the aircraft has arrived and turned.
func (p *Plan) rotate(m *world.Manifest) {
	for _, legs := range rotations(m) {
		prevIn := -1
		for _, f := range legs {
			pf := p.Flights[Key(f)]
			if pf.Cancelled {
				prevIn = -1
				continue
			}
			dep := f.DepMin + pf.DepDelay
			if prevIn >= 0 && prevIn+MinTurnMin > dep {
				late := prevIn + MinTurnMin - dep
				pf.LateAircraft = late
				pf.DepDelay += late
				pf.ArrDelay += late
				p.Summary.LateAircraft++
			}
			prevIn = f.ArrMin + pf.ArrDelay
		}
	}
}

// crews gives each tail its crews: a duty runs as many legs as Part 117
// allows as scheduled, then a fresh crew takes the aircraft. With the day's
// delays each duty is checked again; one that only fits on the two-hour
// extension is noted, and one that does not fit at all is either rescued
// by a reserve crew, when the leg leaves the carrier's base, or cancelled
// for crew -- a carrier-caused cancellation, code A. On a recorded day the
// duties are reported and nothing is cancelled: the record already says
// what did not fly.
func (p *Plan) crews(m *world.Manifest) {
	hubs := map[string]string{}
	for _, c := range m.Carriers {
		hubs[c.Designator] = c.Hub
	}
	at := func(min int) time.Time { return p.Day.Add(time.Duration(min) * time.Minute) }
	for _, legs := range rotations(m) {
		var duty crew.Duty
		dutyN := 0
		planned := 0
		start := func(f world.Flight) {
			dutyN++
			planned = 0
			duty = crew.Duty{Rules: crew.Part117, Report: at(f.DepMin).Add(-ReportLead), Release: Release}
			p.Summary.Duties++
		}
		for i, f := range legs {
			pf := p.Flights[Key(f)]
			// As scheduled: does this leg fit the current duty?
			trial := duty
			trial.Legs = append(append([]crew.Leg(nil), duty.Legs...), crew.Leg{Flight: f.Carrier + f.Number, Depart: at(f.DepMin), Arrive: at(f.ArrMin)})
			if i == 0 || dutyN == 0 || !trial.Check().Legal || trial.Check().Extended {
				start(f)
			}
			duty.Legs = append(duty.Legs, crew.Leg{Flight: f.Carrier + f.Number, Depart: at(f.DepMin), Arrive: at(f.ArrMin)})
			planned++
			pf.Duty = dutyN
			if pf.Cancelled {
				continue
			}
			// As the day runs: the duty with every leg at its real times.
			actual := duty
			actual.Legs = make([]crew.Leg, len(duty.Legs))
			for j := range duty.Legs {
				g := legs[i-len(duty.Legs)+1+j]
				gf := p.Flights[Key(g)]
				actual.Legs[j] = crew.Leg{Flight: g.Carrier + g.Number, Depart: at(g.DepMin + gf.DepDelay), Arrive: at(g.ArrMin + gf.ArrDelay)}
			}
			v := actual.Check()
			switch {
			case v.Legal && v.Extended:
				pf.Extended = true
				p.Summary.Extended++
			case !v.Legal && !p.Replay:
				if hubs[f.Carrier] == f.From {
					// A reserve crew reports at the base; the flight waits for them.
					pf.Reserve = true
					pf.DepDelay += ReserveCall
					pf.ArrDelay += ReserveCall
					p.Summary.Reserves++
					start(f)
					duty.Legs = []crew.Leg{{Flight: f.Carrier + f.Number, Depart: at(f.DepMin), Arrive: at(f.ArrMin)}}
					pf.Duty = dutyN
				} else {
					pf.Cancelled, pf.Code = true, "A"
					pf.Reason = "crew duty limit: " + v.Reason
					p.Summary.TimedOut++
				}
			case !v.Legal:
				pf.Extended = true
				p.Summary.TimedOut++
			}
		}
	}
}
