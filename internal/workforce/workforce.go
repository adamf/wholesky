// Package workforce is the people who fly the world's carriers and the
// agreements they work under. Every carrier has a roster sized from its
// schedule: captains, first officers and flight attendants at its crew
// bases, by fleet, each with a seniority, years of service, a line of days
// on and off, and a record of the last 28 days. The day plan cuts each
// tail's day into duties; the roster staffs each duty by name with
// members who are legal for it under jetway's crew rules -- Part 117's
// tables, its cumulative limits and its rest for pilots, 121.467 for
// cabin crew -- reaching for a reserve when the line runs out and for a
// member on a day off, at the agreement's premium, when the reserves do.
// As the day runs late the named crews are checked again against their
// own records, and a pilot pushed over a limit, or a member who calls in
// sick, needs a reserve at a crew base or the leg does not fly.
//
// The agreements set what all of it costs: credit by block, rig and
// minimum day, pay protection when a duty is cancelled, premiums for
// extensions and day-off assignments, per diem away from base, hotels on
// layovers, and the ground staff's overtime when the day runs late. An
// agreement's standing -- in force, amendable, in talks, at an impasse --
// moves how many members call in sick and how often the union grieves.
//
// Everything is drawn deterministically from the carrier and the day, so
// every machine that staffs a carrier staffs it the same way.
package workforce

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/crew"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// Group is a crew position.
type Group string

const (
	Captain         Group = "captain"
	FirstOfficer    Group = "first officer"
	FlightAttendant Group = "flight attendant"
)

func (g Group) unit() Unit {
	if g == FlightAttendant {
		return FlightAttendants
	}
	return Pilots
}

func (g Group) code() string {
	switch g {
	case Captain:
		return "CA"
	case FirstOfficer:
		return "FO"
	}
	return "FA"
}

// Fleet is the pay and qualification class of an aircraft.
type Fleet string

const (
	Regional   Fleet = "regional"
	Narrowbody Fleet = "narrowbody"
	Widebody   Fleet = "widebody"
)

// FleetOf classes an aircraft by its seats, and any long-haul flight as a
// widebody's.
func FleetOf(seats int, longHaul bool) Fleet {
	switch {
	case longHaul || seats >= 230:
		return Widebody
	case seats > 0 && seats < 100:
		return Regional
	}
	return Narrowbody
}

// Member is one crew member.
type Member struct {
	ID        string `json:"id"`
	Group     Group  `json:"group"`
	Base      string `json:"base"`
	Fleet     Fleet  `json:"fleet"`
	Seniority int    `json:"seniority"`
	Years     int    `json:"years"`
	// Reserve says the member holds a reserve line this month.
	Reserve bool `json:"reserve"`
	// Standby says the member was brought in for the day on top of the
	// roster: a volunteer on a day off, paid a premium to sit reserve.
	Standby bool `json:"standby,omitempty"`

	line pattern
	hist history

	// The ops day: whether the member works it, the duty they fly, and
	// what happened.
	duty    string
	sick    bool
	short   bool // the sick call comes after the roster is made
	called  bool // a reserve called out
	junior  bool // assigned on a day off
	relieve bool // taken off the duty part way
}

// poolKey names one pool of interchangeable members.
type poolKey struct {
	base    string
	fleet   Fleet
	group   Group
	reserve bool
}

// dutyState is one duty as the roster staffed it.
type dutyState struct {
	sched   dayplan.DutyRef
	actual  crew.Duty
	crew    []int32
	off     []int32 // members relieved part way, sick or timed out
	called  []int32 // reserves called out to it
	failing []int32
	layover bool
	// short counts the flight attendants the duty flies without, seats
	// blocked.
	short   int
	fleet   Fleet
	base    string
	pay     int64
	premium int64
	perDiem int64
	hotel   int64
	dirty   bool
}

// Roster is one carrier's crew.
type Roster struct {
	Carrier   string
	Country   string
	Hub       string
	Bases     []string
	Contracts map[Unit]*Contract
	Members   []Member

	pools  map[poolKey][]int32
	cursor map[poolKey]int
	duties map[string]*dutyState
	byTail map[string][]string
	// Grievances are the day-off assignments the union grieved.
	Grievances int
	// Junior counts the day-off assignments made.
	Junior int
	// Standby counts members brought in for the day.
	Standby int
	byID    map[string]int32
	planned map[poolKey]int // members working today the schedule needs
}

// Workforce is every carrier's roster that this machine staffs.
type Workforce struct {
	mu   sync.Mutex
	m    *world.Manifest
	day  time.Time
	dayN int
	// Owns says which carriers this machine staffs; the plan falls back to
	// any crew and hub reserves for the rest.
	owns     func(string) bool
	rosters  map[string]*Roster
	carriers map[string]world.Carrier
	flights  map[string][]world.Flight
}

// New is the workforce for a world's day. owns, when set, limits the
// rosters to the carriers this machine runs.
func New(m *world.Manifest, day time.Time, owns func(string) bool) *Workforce {
	w := &Workforce{m: m, day: day, owns: owns, rosters: map[string]*Roster{}, carriers: map[string]world.Carrier{}, flights: map[string][]world.Flight{}}
	for _, c := range m.Carriers {
		w.carriers[c.Designator] = c
	}
	for _, f := range m.Flights {
		w.flights[f.Carrier] = append(w.flights[f.Carrier], f)
	}
	return w
}

// Day is the ops day and how many days the workforce has worked.
func (w *Workforce) Day() (time.Time, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.day, w.dayN
}

// roster is a carrier's roster, built on first use; nil for a carrier
// this machine does not staff. The caller holds w.mu.
func (w *Workforce) roster(code string) *Roster {
	if r, ok := w.rosters[code]; ok {
		return r
	}
	if w.owns != nil && !w.owns(code) {
		w.rosters[code] = nil
		return nil
	}
	c, ok := w.carriers[code]
	if !ok {
		w.rosters[code] = nil
		return nil
	}
	r := w.build(c)
	w.rosters[code] = r
	return r
}

// Roster is a carrier's roster for reading; nil when not staffed here.
func (w *Workforce) Roster(code string) *Roster {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.roster(strings.ToUpper(code))
}

// bases are where the carrier keeps crews: its hub, and up to two more of
// its busiest stations when each has a tenth of its departures.
func bases(c world.Carrier, fs []world.Flight) []string {
	out := []string{c.Hub}
	count := map[string]int{}
	for _, f := range fs {
		count[f.From]++
	}
	type st struct {
		code string
		n    int
	}
	var list []st
	for code, n := range count {
		if code != c.Hub {
			list = append(list, st{code, n})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].code < list[j].code
	})
	for _, s := range list {
		if len(out) == 3 || s.n*10 < len(fs) || s.n < 8 {
			break
		}
		out = append(out, s.code)
	}
	return out
}

// complement is the crew a duty needs: two pilots, or three or four on an
// augmented long haul, and the cabin crew 121.391 requires for the seats
// plus what 121.467 needs for a scheduled duty over fourteen hours.
func complement(seats int, longHaul bool, block, duty time.Duration) map[Group]int {
	need := map[Group]int{Captain: 1, FirstOfficer: 1}
	if longHaul {
		need[FirstOfficer] = 2
		if block > 12*time.Hour {
			need[Captain] = 2
		}
	}
	fa := crew.MinFlightAttendants(max(seats, 1))
	switch {
	case duty > 18*time.Hour:
		fa += 3
	case duty > 16*time.Hour:
		fa += 2
	case duty > 14*time.Hour:
		fa++
	}
	need[FlightAttendant] = fa
	return need
}

// cabinExtra is how many flight attendants above the minimum a duty of
// that length carries.
func cabinExtra(duty time.Duration) int {
	switch {
	case duty > 18*time.Hour:
		return 3
	case duty > 16*time.Hour:
		return 2
	case duty > 14*time.Hour:
		return 1
	}
	return 0
}

func dutySpan(d crew.Duty) (block, length time.Duration) {
	if len(d.Legs) == 0 {
		return 0, 0
	}
	last := d.Legs[0].Arrive
	for _, l := range d.Legs {
		block += l.Arrive.Sub(l.Depart)
		if l.Arrive.After(last) {
			last = l.Arrive
		}
	}
	return block, last.Add(d.Release).Sub(d.Report)
}

// build sizes a carrier's roster from its schedule: for each base and
// fleet, the members a day's duties need, a sixth more for the ones who
// cannot fly a given duty, divided by the share of a line that is on, and
// the agreement's share of reserves on top.
func (w *Workforce) build(c world.Carrier) *Roster {
	fs := w.flights[c.Designator]
	r := &Roster{Carrier: c.Designator, Country: c.Country, Hub: c.Hub, Bases: bases(c, fs),
		Contracts: map[Unit]*Contract{}, pools: map[poolKey][]int32{}, cursor: map[poolKey]int{},
		duties: map[string]*dutyState{}, byTail: map[string][]string{}, byID: map[string]int32{}, planned: map[poolKey]int{}}
	for _, u := range []Unit{Pilots, FlightAttendants, Ground} {
		r.Contracts[u] = NewContract(c.Designator, c.Country, u, w.day)
	}
	// The day's duties as scheduled, the way the plan cuts them, give the
	// demand: a duty starting away from a base is flown by a crew of the
	// hub on a layover.
	need := map[poolKey]int{}
	for _, d := range scheduledDuties(fs, w.day) {
		base := r.baseFor(d.from)
		block, length := dutySpan(d.duty)
		fl := FleetOf(d.seats, d.long)
		for g, n := range complement(d.seats, d.long, block, length) {
			need[poolKey{base, fl, g, false}] += n
		}
	}
	keys := make([]poolKey, 0, len(need))
	for k := range need {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fmt.Sprint(keys[i]) < fmt.Sprint(keys[j]) })
	seq := map[Group]int{}
	for _, k := range keys {
		r.planned[k] = need[k]
		c := r.Contracts[k.group.unit()]
		// A line is on four days in seven; a fifth more members than the
		// duties need work each day, for the ones who cannot fly a given
		// duty under the rules.
		line := (need[k]*6*7 + 5*4 - 1) / (5 * 4)
		reserves := max(1, int(float64(line)*c.ReserveShare+0.5))
		for i := 0; i < line+reserves; i++ {
			seq[k.group]++
			id := fmt.Sprintf("%s-%s-%05d", r.Carrier, k.group.code(), seq[k.group])
			m := Member{ID: id, Group: k.group, Base: k.base, Fleet: k.fleet, Reserve: i >= line,
				Years: 1 + int(draw(id, "years")*24)}
			n := i
			if m.Reserve {
				n = i - line
			}
			m.line = linePattern(n)
			m.hist = synthesise(id, m.line, w.dayN, m.Reserve)
			r.add(m)
		}
	}
	r.rank()
	r.today(w.dayN)
	return r
}

func (r *Roster) add(m Member) int32 {
	idx := int32(len(r.Members))
	r.Members = append(r.Members, m)
	r.byID[m.ID] = idx
	k := poolKey{m.Base, m.Fleet, m.Group, m.Reserve}
	r.pools[k] = append(r.pools[k], idx)
	return idx
}

// rank numbers each position's seniority by years of service.
func (r *Roster) rank() {
	byGroup := map[Group][]int32{}
	for i := range r.Members {
		byGroup[r.Members[i].Group] = append(byGroup[r.Members[i].Group], int32(i))
	}
	for _, idx := range byGroup {
		sort.Slice(idx, func(a, b int) bool {
			ma, mb := r.Members[idx[a]], r.Members[idx[b]]
			if ma.Years != mb.Years {
				return ma.Years > mb.Years
			}
			return ma.ID < mb.ID
		})
		for n, i := range idx {
			r.Members[i].Seniority = n + 1
		}
	}
}

// shortNotice is the share of sick calls that come at report, after the
// roster is made, rather than the evening before.
const shortNotice = 0.08

// today draws the ops day's sick calls: most come the evening before and
// keep the member off the roster; a few come at report.
func (r *Roster) today(dayN int) {
	d := itoaN(dayN)
	for i := range r.Members {
		m := &r.Members[i]
		m.duty, m.called, m.junior, m.relieve = "", false, false, false
		rate := r.Contracts[m.Group.unit()].SickToday()
		m.sick = draw(m.ID, d, "sick") < rate
		m.short = m.sick && draw(m.ID, d, "short") < shortNotice
	}
}

// baseFor is the crew base that flies a duty starting at the station: the
// station itself when it is one, else the hub, on a layover.
func (r *Roster) baseFor(station string) string {
	for _, b := range r.Bases {
		if b == station {
			return b
		}
	}
	return r.Hub
}

// IsBase says the station is a crew base.
func (r *Roster) IsBase(station string) bool {
	for _, b := range r.Bases {
		if b == station {
			return true
		}
	}
	return false
}

// schedDuty is a duty as the schedule cuts it, for sizing.
type schedDuty struct {
	from  string
	seats int
	long  bool
	duty  crew.Duty
}

// scheduledDuties cuts each tail's day the way the plan does, as
// scheduled, so the roster is sized for the duties it will be asked for.
func scheduledDuties(fs []world.Flight, day time.Time) []schedDuty {
	tails := map[string][]world.Flight{}
	for _, f := range fs {
		if f.Tail != "" {
			tails[f.Tail] = append(tails[f.Tail], f)
		}
	}
	at := func(min int) time.Time { return day.Add(time.Duration(min) * time.Minute) }
	var out []schedDuty
	for _, legs := range tails {
		sort.Slice(legs, func(i, j int) bool { return legs[i].DepMin < legs[j].DepMin })
		var cur *schedDuty
		for _, f := range legs {
			leg := crew.Leg{Depart: at(f.DepMin), Arrive: at(f.ArrMin)}
			if f.ArrMin-f.DepMin >= dayplan.LongHaulMin {
				if cur != nil {
					out = append(out, *cur)
					cur = nil
				}
				out = append(out, schedDuty{from: f.From, seats: f.Seats, long: true, duty: crew.Duty{Rules: crew.Part117, Report: leg.Depart.Add(-dayplan.ReportLead), Release: dayplan.Release, Legs: []crew.Leg{leg}}})
				continue
			}
			if cur != nil {
				trial := cur.duty
				trial.Legs = append(append([]crew.Leg(nil), cur.duty.Legs...), leg)
				if v := trial.Check(); !v.Legal || v.Extended {
					out = append(out, *cur)
					cur = nil
				}
			}
			if cur == nil {
				cur = &schedDuty{from: f.From, duty: crew.Duty{Rules: crew.Part117, Report: leg.Depart.Add(-dayplan.ReportLead), Release: dayplan.Release}}
			}
			cur.duty.Legs = append(cur.duty.Legs, leg)
			cur.seats = max(cur.seats, f.Seats)
		}
		if cur != nil {
			out = append(out, *cur)
		}
	}
	return out
}

// augmented are the rules a long-haul duty is checked under. Part 117's
// Table C, the augmented crew's limits by rest facility and crew size, is
// not transcribed in jetway, so the duty's own length is not limited
// here; the cumulative limits and the rest rules apply as they do to any
// duty.
var augmented = func() crew.Rules {
	r := crew.Part117
	r.FDP = func(int, int) [6]float64 { return [6]float64{24, 24, 24, 24, 24, 24} }
	r.FlightTime = func(int, int) float64 { return 24 }
	return r
}()

func rulesFor(long bool) crew.Rules {
	if long {
		return augmented
	}
	return crew.Part117
}

// legal says whether a member may fly the duty.
func (w *Workforce) legal(m *Member, d dayplan.DutyRef, scheduled bool) crew.Verdict {
	v := view{h: &m.hist, day: w.day}
	if m.Group == FlightAttendant {
		if !scheduled {
			// 121.467(b)(14): a cabin crew is not over its limit when the
			// day runs late; the schedule was legal or they were not on it.
			return crew.Verdict{Legal: true}
		}
		_, length := dutySpan(d.Duty)
		return crew.Part121Cabin.Admits(v, d.Duty, cabinExtra(length), true)
	}
	duty := d.Duty
	duty.Rules = rulesFor(d.LongHaul)
	return duty.Admits(v)
}

// pick finds a member for a position: the line first, in turn, so the
// flying spreads; the reserves next, which is open time covered before the
// day; a member on a day off last, at the agreement's premium.
func (w *Workforce) pick(r *Roster, base string, fl Fleet, g Group, d dayplan.DutyRef) (int32, string) {
	dayN := w.dayN
	try := func(k poolKey, eligible func(m *Member) bool) int32 {
		pool := r.pools[k]
		if len(pool) == 0 {
			return -1
		}
		start := r.cursor[k] % len(pool)
		for n := 0; n < len(pool); n++ {
			i := pool[(start+n)%len(pool)]
			m := &r.Members[i]
			if m.duty != "" || m.called || (m.sick && !m.short) || !eligible(m) {
				continue
			}
			if !w.legal(m, d, true).Legal {
				continue
			}
			r.cursor[k] = (start + n + 1) % len(pool)
			return i
		}
		return -1
	}
	line := poolKey{base, fl, g, false}
	if i := try(line, func(m *Member) bool { return m.line.On(dayN) }); i >= 0 {
		return i, "line"
	}
	if i := try(poolKey{base, fl, g, true}, func(m *Member) bool { return m.line.On(dayN) || m.Standby }); i >= 0 {
		r.Members[i].called = true
		return i, "reserve"
	}
	if r.Contracts[g.unit()].JuniorAllowed {
		if i := try(line, func(m *Member) bool { return !m.line.On(dayN) }); i >= 0 {
			r.Members[i].junior = true
			r.Junior++
			if draw(r.Members[i].ID, itoaN(dayN), "grieve") < r.Contracts[g.unit()].GrievanceOdds() {
				r.Grievances++
			}
			return i, "junior"
		}
	}
	return -1, ""
}

// Assign implements dayplan.Staff.
func (w *Workforce) Assign(d dayplan.DutyRef) (bool, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(d.Carrier)
	if r == nil {
		return true, ""
	}
	id := d.ID()
	if old, ok := r.duties[id]; ok {
		r.free(old)
	}
	block, length := dutySpan(d.Duty)
	fl := FleetOf(d.Seats, d.LongHaul)
	base := r.baseFor(d.From)
	ds := &dutyState{sched: d, actual: d.Duty, layover: base != d.From, fleet: fl, base: base, dirty: true}
	need := complement(d.Seats, d.LongHaul, block, length)
	for _, g := range []Group{Captain, FirstOfficer, FlightAttendant} {
		for n := 0; n < need[g]; n++ {
			i, _ := w.pick(r, base, fl, g, d)
			if i < 0 {
				r.free(ds)
				return false, fmt.Sprintf("no legal %s at %s for the %s duty", g, base, d.Duty.Report.Add(dayplan.ReportLead).Format("1504"))
			}
			r.Members[i].duty = id
			ds.crew = append(ds.crew, i)
		}
	}
	r.duties[id] = ds
	tailKey := d.Carrier + "/" + d.Tail
	r.byTail[tailKey] = appendOnce(r.byTail[tailKey], id)
	return true, ""
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// free lets a duty's members go, as if it had never been staffed.
func (r *Roster) free(ds *dutyState) {
	for _, set := range [][]int32{ds.crew, ds.off, ds.called} {
		for _, i := range set {
			m := &r.Members[i]
			m.duty, m.relieve = "", false
			if m.junior {
				m.junior = false
				r.Junior--
			}
			if m.Reserve {
				m.called = false
			}
		}
	}
	ds.crew, ds.off, ds.called = nil, nil, nil
}

// Check implements dayplan.Staff.
func (w *Workforce) Check(d dayplan.DutyRef) crew.Verdict {
	w.mu.Lock()
	defer w.mu.Unlock()
	fallback := func() crew.Verdict {
		if d.LongHaul {
			return crew.Verdict{Legal: true}
		}
		return d.Duty.Check()
	}
	r := w.roster(d.Carrier)
	if r == nil {
		return fallback()
	}
	ds, ok := r.duties[d.ID()]
	if !ok {
		return fallback()
	}
	ds.actual = d.Duty
	ds.dirty = true
	ds.failing = ds.failing[:0]
	// Short-notice sick calls are found at report, by the crew scheduled.
	// A pilot must be replaced. A flight attendant need not: while another
	// remains, the flight goes one short with seats blocked, as carriers
	// do, and the sick member comes off the duty.
	if d.Relief == 0 {
		var sick []string
		var cabinSick []int32
		cabin := 0
		for _, i := range ds.crew {
			m := &r.Members[i]
			if m.Group == FlightAttendant {
				cabin++
			}
			if !m.sick || !m.short {
				continue
			}
			if m.Group == FlightAttendant {
				cabinSick = append(cabinSick, i)
				continue
			}
			ds.failing = append(ds.failing, i)
			sick = append(sick, string(m.Group)+" "+m.ID)
		}
		if len(cabinSick) > 0 && len(cabinSick) < cabin {
			keep := ds.crew[:0]
			for _, i := range ds.crew {
				off := false
				for _, j := range cabinSick {
					off = off || i == j
				}
				if off {
					r.Members[i].relieve = true
					ds.off = append(ds.off, i)
					ds.short++
				} else {
					keep = append(keep, i)
				}
			}
			ds.crew = keep
		} else if len(cabinSick) > 0 {
			for _, i := range cabinSick {
				ds.failing = append(ds.failing, i)
				sick = append(sick, string(r.Members[i].Group)+" "+r.Members[i].ID)
			}
		}
		if len(sick) > 0 {
			return crew.Verdict{Reason: strings.Join(sick, ", ") + " called in sick"}
		}
	}
	var out crew.Verdict
	out.Legal = true
	for _, i := range ds.crew {
		m := &r.Members[i]
		if m.Group == FlightAttendant {
			continue
		}
		v := w.legal(m, d, false)
		if v.Extended {
			out.Extended = true
		}
		if !v.Legal {
			ds.failing = append(ds.failing, i)
			if out.Legal {
				out = v
				out.Reason = string(m.Group) + " " + m.ID + ": " + v.Reason
			}
		}
	}
	return out
}

// Reserve implements dayplan.Staff: every member the last check found
// unable to fly is replaced by a reserve on call at the station, which
// must be a crew base, of the same fleet and position and legal for the
// rest of the duty. All of them or none.
func (w *Workforce) Reserve(d dayplan.DutyRef, station string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(d.Carrier)
	if r == nil {
		return false
	}
	ds, ok := r.duties[d.ID()]
	if !ok || !r.IsBase(station) {
		return false
	}
	failing := ds.failing
	if len(failing) == 0 {
		for _, i := range ds.crew {
			if r.Members[i].Group != FlightAttendant {
				failing = append(failing, i)
			}
		}
	}
	var got []int32
	for _, i := range failing {
		g := r.Members[i].Group
		found := int32(-1)
		k := poolKey{station, ds.fleet, g, true}
		for _, j := range r.pools[k] {
			m := &r.Members[j]
			if m.called || m.duty != "" || (m.sick && !m.short) || !(m.line.On(w.dayN) || m.Standby) {
				continue
			}
			if m.sick {
				continue
			}
			if !w.legal(m, d, true).Legal {
				continue
			}
			found = j
			break
		}
		if found < 0 {
			for _, j := range got {
				r.Members[j].called, r.Members[j].duty = false, ""
			}
			return false
		}
		r.Members[found].called = true
		r.Members[found].duty = d.ID()
		got = append(got, found)
	}
	// The relieved come off the duty; the reserves go on.
	keep := ds.crew[:0]
	for _, i := range ds.crew {
		off := false
		for _, f := range failing {
			if f == i {
				off = true
			}
		}
		if off {
			r.Members[i].relieve = true
			ds.off = append(ds.off, i)
		} else {
			keep = append(keep, i)
		}
	}
	ds.crew = append(keep, got...)
	ds.called = append(ds.called, got...)
	ds.failing = nil
	ds.dirty = true
	return true
}

// Release implements dayplan.Staff.
func (w *Workforce) Release(carrier, tail string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rosters[carrier]
	if r == nil {
		return
	}
	key := carrier + "/" + tail
	for _, id := range r.byTail[key] {
		if ds, ok := r.duties[id]; ok {
			r.free(ds)
			delete(r.duties, id)
		}
	}
	delete(r.byTail, key)
}

// AddStandby brings members in for the day at a base: volunteers on a day
// off who sit reserve at the agreement's premium. It returns how many came.
func (w *Workforce) AddStandby(carrier, base string, n int) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	if r == nil || !r.IsBase(base) {
		return 0
	}
	// Each pilot position and the cabin on the base's busiest fleet.
	fl, most := Narrowbody, -1
	for k, v := range r.planned {
		if k.base == base && k.group == Captain && v > most {
			fl, most = k.fleet, v
		}
	}
	added := 0
	for i := 0; i < n; i++ {
		for _, g := range []Group{Captain, FirstOfficer, FlightAttendant, FlightAttendant} {
			r.Standby++
			id := fmt.Sprintf("%s-%s-S%04d", r.Carrier, g.code(), r.Standby)
			m := Member{ID: id, Group: g, Base: base, Fleet: fl, Reserve: true, Standby: true, Years: 1 + int(draw(id, "years")*24)}
			m.line = pattern{on: 1, off: 6}
			r.add(m)
		}
		added++
	}
	return added
}

// NextDay closes the ops day: every member's record takes the day as it
// was flown, the line moves on a day, the sick calls are drawn again, and
// every duty is forgotten. The plan staffs the new day afresh.
func (w *Workforce) NextDay() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, r := range w.rosters {
		if r == nil {
			continue
		}
		for i := range r.Members {
			m := &r.Members[i]
			rec := dayRec{}
			if ds, ok := r.duties[m.duty]; ok && m.duty != "" {
				block, length := dutySpan(ds.actual)
				if length > 0 {
					rep := ds.actual.Report.Sub(w.day)
					rec = dayRec{report: uint16(max(0, int(rep/time.Minute))), length: uint16(length / time.Minute), flight: uint16(block / time.Minute), kind: kindFlight}
				}
			} else if m.Reserve && m.line.On(w.dayN) && !m.sick {
				rec = dayRec{report: 300, length: 840, kind: kindReserve}
			}
			m.hist.roll(rec)
		}
		// Standby members came for one day.
		keep := r.Members[:0]
		for _, m := range r.Members {
			if !m.Standby {
				keep = append(keep, m)
			}
		}
		r.Members = keep
		r.pools, r.byID = map[poolKey][]int32{}, map[string]int32{}
		for i := range r.Members {
			m := r.Members[i]
			r.byID[m.ID] = int32(i)
			k := poolKey{m.Base, m.Fleet, m.Group, m.Reserve}
			r.pools[k] = append(r.pools[k], int32(i))
		}
		r.duties, r.byTail = map[string]*dutyState{}, map[string][]string{}
		r.Junior, r.Grievances = 0, 0
	}
	w.dayN++
	for _, r := range w.rosters {
		if r != nil {
			r.today(w.dayN)
		}
	}
}

// Clone is an independent copy of the carrier's roster as it stood before
// the day was staffed, for a simulation to staff its own versions of the
// day without touching the world's.
func (w *Workforce) Clone(carriers ...string) *Workforce {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := &Workforce{m: w.m, day: w.day, dayN: w.dayN, owns: w.owns, rosters: map[string]*Roster{}, carriers: w.carriers, flights: w.flights}
	for _, code := range carriers {
		r := w.roster(code)
		if r == nil {
			continue
		}
		cp := *r
		cp.Members = append([]Member(nil), r.Members...)
		for i := range cp.Members {
			cp.Members[i].duty, cp.Members[i].called, cp.Members[i].relieve = "", false, false
			cp.Members[i].junior = false
		}
		cp.Contracts = map[Unit]*Contract{}
		for u, k := range r.Contracts {
			kc := *k
			cp.Contracts[u] = &kc
		}
		cp.pools = map[poolKey][]int32{}
		for k, v := range r.pools {
			cp.pools[k] = append([]int32(nil), v...)
		}
		cp.cursor = map[poolKey]int{}
		cp.duties, cp.byTail = map[string]*dutyState{}, map[string][]string{}
		cp.byID = make(map[string]int32, len(r.byID))
		for k, v := range r.byID {
			cp.byID[k] = v
		}
		cp.Junior, cp.Grievances = 0, 0
		c.rosters[code] = &cp
	}
	// A clone staffs only the carriers it was made for.
	c.owns = func(code string) bool { _, ok := c.rosters[code]; return ok }
	return c
}

// SetSickRate overrides the share of a carrier's crews who call in sick
// today, as a scenario's sick-out does, and draws the calls again. The
// plan must then staff the carrier's tails again for the calls to bite.
func (w *Workforce) SetSickRate(carrier string, rate float64, seed string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	if r == nil {
		return
	}
	for i := range r.Members {
		m := &r.Members[i]
		m.sick = draw(m.ID, seed, "sickout") < rate
		m.short = m.sick && draw(m.ID, seed, "short") < shortNotice
	}
}

// CrewOf names the crew on a duty: position and employee number.
func (w *Workforce) CrewOf(carrier, tail string, n int) []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rosters[carrier]
	if r == nil {
		return nil
	}
	ds, ok := r.duties[fmt.Sprintf("%s/%s/%d", carrier, tail, n)]
	if !ok {
		return nil
	}
	var out []string
	for _, i := range ds.crew {
		m := &r.Members[i]
		tag := m.Group.code() + " " + m.ID
		if m.called {
			tag += " (reserve)"
		}
		if m.junior {
			tag += " (day off)"
		}
		out = append(out, tag)
	}
	return out
}

var _ dayplan.Staff = (*Workforce)(nil)
