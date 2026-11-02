package workforce

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/crew"
)

// Headcount is one pool of a carrier's crew: a base, a fleet, a position.
type Headcount struct {
	Base  string `json:"base"`
	Fleet Fleet  `json:"fleet"`
	Group Group  `json:"group"`
	// Line and Reserve are the members holding each kind of line; OnToday
	// the line holders whose line works today; Flying those on a duty;
	// OnCall the reserves on call; Called those called out; Sick those who
	// called in.
	Line    int `json:"line"`
	Reserve int `json:"reserve"`
	OnToday int `json:"on_today"`
	Flying  int `json:"flying"`
	OnCall  int `json:"on_call"`
	Called  int `json:"called"`
	Sick    int `json:"sick"`
	Junior  int `json:"junior"`
}

// Limit is one cumulative limit as a member stands against it.
type Limit struct {
	Name      string `json:"name"`
	UsedMin   int    `json:"used_min"`
	LimitMin  int    `json:"limit_min"`
	RemainMin int    `json:"remaining_min"`
	Cite      string `json:"cite"`
}

func limits(ts []crew.Total) []Limit {
	out := make([]Limit, 0, len(ts))
	for _, t := range ts {
		out = append(out, Limit{Name: t.Name, UsedMin: int(t.Used / time.Minute), LimitMin: int(t.Limit / time.Minute), RemainMin: int(t.Remaining() / time.Minute), Cite: t.Cite})
	}
	return out
}

// MemberView is one crew member for the page.
type MemberView struct {
	ID        string  `json:"id"`
	Group     Group   `json:"group"`
	Base      string  `json:"base"`
	Fleet     Fleet   `json:"fleet"`
	Seniority int     `json:"seniority"`
	Years     int     `json:"years"`
	Reserve   bool    `json:"reserve"`
	Status    string  `json:"status"`
	Duty      string  `json:"duty,omitempty"`
	Limits    []Limit `json:"limits"`
	// Days are the last 28 days, newest first: report and release as
	// HHMM, block in minutes, the kind.
	Days []DayView `json:"days,omitempty"`
}

// DayView is one past day of a member's record.
type DayView struct {
	DaysAgo int    `json:"days_ago"`
	Kind    string `json:"kind"`
	Report  string `json:"report,omitempty"`
	Release string `json:"release,omitempty"`
	Block   int    `json:"block_min,omitempty"`
}

// Overview is a carrier's people in one picture.
type Overview struct {
	Carrier    string      `json:"carrier"`
	Day        string      `json:"day"`
	DayN       int         `json:"day_n"`
	Bases      []string    `json:"bases"`
	Members    int         `json:"members"`
	Headcount  []Headcount `json:"headcount"`
	Contracts  []*Contract `json:"contracts"`
	Costs      Costs       `json:"costs"`
	Duties     int         `json:"duties"`
	Relieved   int         `json:"relieved"`
	Called     int         `json:"called"`
	Junior     int         `json:"junior"`
	Grievances int         `json:"grievances"`
	Standby    int         `json:"standby"`
	// Watch are the pilots with the least time left under a cumulative
	// limit once today's duty is flown.
	Watch []MemberView `json:"watch"`
}

func (w *Workforce) status(r *Roster, m *Member) string {
	switch {
	case m.sick:
		return "sick"
	case m.relieve:
		return "relieved"
	case m.duty != "" && m.called:
		return "reserve, called"
	case m.duty != "" && m.junior:
		return "flying on a day off"
	case m.duty != "":
		return "flying"
	case m.Reserve && (m.line.On(w.dayN) || m.Standby):
		return "reserve, on call"
	case m.line.On(w.dayN):
		return "line, not needed"
	}
	return "off"
}

// member builds a member's view; days says include the record.
func (w *Workforce) member(r *Roster, i int32, days bool) MemberView {
	m := &r.Members[i]
	mv := MemberView{ID: m.ID, Group: m.Group, Base: m.Base, Fleet: m.Fleet, Seniority: m.Seniority, Years: m.Years, Reserve: m.Reserve, Status: w.status(r, m), Duty: m.duty}
	v := view{h: &m.hist, day: w.day}
	if ds, ok := r.duties[m.duty]; ok && m.duty != "" && m.Group != FlightAttendant && len(ds.actual.Legs) > 0 {
		d := ds.actual
		d.Rules = rulesFor(ds.sched.LongHaul)
		mv.Limits = limits(d.Totals(v))
	} else if m.Group != FlightAttendant {
		mv.Limits = limits(crew.Part117.Remaining(v, w.day))
	}
	if days {
		for n := range m.hist.days {
			rec := m.hist.days[n]
			dv := DayView{DaysAgo: n + 1, Kind: "off"}
			switch rec.kind {
			case kindFlight:
				dv.Kind = "flight duty"
			case kindReserve:
				dv.Kind = "reserve"
			}
			if rec.kind != kindNone {
				dv.Report = hhmm(int(rec.report))
				dv.Release = hhmm(int(rec.report) + int(rec.length))
				dv.Block = int(rec.flight)
			}
			mv.Days = append(mv.Days, dv)
		}
	}
	return mv
}

func hhmm(min int) string { return fmt.Sprintf("%02d%02d", (min/60)%24, min%60) }

// Overview is the carrier's people today; false when not staffed here.
func (w *Workforce) Overview(carrier string, pos float64) (Overview, bool) {
	costs, ok := w.Costs(carrier, pos)
	if !ok {
		return Overview{}, false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	o := Overview{Carrier: r.Carrier, Day: w.day.Format("2006-01-02"), DayN: w.dayN, Bases: r.Bases, Members: len(r.Members),
		Costs: costs, Duties: len(r.duties), Junior: r.Junior, Grievances: r.Grievances, Standby: r.Standby}
	for _, u := range []Unit{Pilots, FlightAttendants, Ground} {
		o.Contracts = append(o.Contracts, r.Contracts[u])
	}
	heads := map[poolKey]*Headcount{}
	type cand struct {
		i      int32
		remain int
	}
	var watch []cand
	for i := range r.Members {
		m := &r.Members[i]
		k := poolKey{m.Base, m.Fleet, m.Group, false}
		h := heads[k]
		if h == nil {
			h = &Headcount{Base: m.Base, Fleet: m.Fleet, Group: m.Group}
			heads[k] = h
		}
		on := m.line.On(w.dayN) || m.Standby
		if m.Reserve {
			h.Reserve++
			if on && !m.sick {
				h.OnCall++
			}
		} else {
			h.Line++
			if on {
				h.OnToday++
			}
		}
		if m.sick {
			h.Sick++
		}
		if m.duty != "" && !m.relieve {
			h.Flying++
		}
		if m.called {
			h.Called++
			o.Called++
		}
		if m.junior {
			h.Junior++
		}
		if m.relieve {
			o.Relieved++
		}
		if m.Group != FlightAttendant && m.duty != "" {
			mv := w.member(r, int32(i), false)
			least := 1 << 30
			for _, l := range mv.Limits {
				least = min(least, l.RemainMin)
			}
			watch = append(watch, cand{int32(i), least})
		}
	}
	for _, h := range heads {
		o.Headcount = append(o.Headcount, *h)
	}
	sort.Slice(o.Headcount, func(i, j int) bool {
		a, b := o.Headcount[i], o.Headcount[j]
		if a.Base != b.Base {
			return a.Base < b.Base
		}
		if a.Fleet != b.Fleet {
			return a.Fleet < b.Fleet
		}
		return a.Group < b.Group
	})
	sort.Slice(watch, func(i, j int) bool {
		if watch[i].remain != watch[j].remain {
			return watch[i].remain < watch[j].remain
		}
		return watch[i].i < watch[j].i
	})
	for _, c := range watch[:min(12, len(watch))] {
		o.Watch = append(o.Watch, w.member(r, c.i, false))
	}
	return o, true
}

// Member is one crew member's record by id.
func (w *Workforce) Member(carrier, id string) (MemberView, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(strings.ToUpper(carrier))
	if r == nil {
		return MemberView{}, false
	}
	i, ok := r.byID[strings.ToUpper(id)]
	if !ok {
		return MemberView{}, false
	}
	return w.member(r, i, true), true
}

// Contract is a carrier's agreement for a unit.
func (w *Workforce) Contract(carrier string, u Unit) (*Contract, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	if r == nil {
		return nil, false
	}
	c, ok := r.Contracts[u]
	return c, ok
}

// Respond answers a union's open proposal: accept settles at the ask,
// counter offers half and keeps the talks open, refuse breaks them off.
// The standing moves the next day's sick calls; a settled raise moves the
// scale at once.
func (w *Workforce) Respond(carrier string, u Unit, choice string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	if r == nil {
		return "", fmt.Errorf("this machine does not staff %s", carrier)
	}
	c := r.Contracts[u]
	if c == nil || c.Proposal == 0 {
		return "", fmt.Errorf("no open proposal from the %s", u)
	}
	ask := c.Proposal
	switch choice {
	case "accept":
		c.Settle(ask, w.day)
		return fmt.Sprintf("%s agreement settled at %.1f%%; in force to %s", u, ask*100, c.Amendable.Format("Jan 2006")), nil
	case "counter":
		// Half the ask: the union takes it about half the time.
		if draw(carrier, string(u), itoaN(w.dayN), "counter") < 0.5 {
			c.Settle(ask/2, w.day)
			return fmt.Sprintf("the %s took %.1f%%; in force to %s", c.Union, ask*50, c.Amendable.Format("Jan 2006")), nil
		}
		c.Status = Negotiating
		c.Proposal = ask * 0.85
		return fmt.Sprintf("the %s refused %.1f%% and now asks %.1f%%", c.Union, ask*50, c.Proposal*100), nil
	case "refuse":
		c.Status = Impasse
		c.JuniorAllowed = false
		return fmt.Sprintf("talks with the %s broken off: impasse, no day-off assignments, sick calls up", c.Union), nil
	}
	return "", fmt.Errorf("answer accept, counter or refuse")
}

// Carriers is the carriers staffed here so far.
func (w *Workforce) Carriers() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for code, r := range w.rosters {
		if r != nil {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}
