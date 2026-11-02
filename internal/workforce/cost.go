package workforce

import (
	"time"

	"github.com/adamf/jetway/pkg/crew"
)

// Costs is what a carrier's people have cost so far in the day, in cents.
type Costs struct {
	// Pay is the crews' credit at their rates; Premium the extensions and
	// day-off assignments on top; PerDiem and Hotels the time away.
	Pay     int64 `json:"pay"`
	Premium int64 `json:"premium"`
	PerDiem int64 `json:"per_diem"`
	Hotels  int64 `json:"hotels"`
	// Reserves is the guarantee of the reserves on call who were not
	// called, and of the standby brought in.
	Reserves int64 `json:"reserves"`
	// Protected is the pay of crews whose duty was cancelled: pay
	// protection, part of Pay, shown apart because it buys no flying.
	Protected  int64 `json:"protected"`
	Grievances int64 `json:"grievances"`
	Total      int64 `json:"total"`
	Duties     int   `json:"duties"`
}

// hotelNight is a crew room for a night, in dollars, before the
// country's level.
const hotelNight = 130

// layoverRest is the rest a crew takes at an outstation before the next
// day's duty: the time away a night out adds.
const layoverRest = 12 * time.Hour

// price works out a duty's cost if anything about it changed.
func (r *Roster) price(ds *dutyState) {
	if !ds.dirty {
		return
	}
	ds.dirty = false
	ds.pay, ds.premium, ds.perDiem, ds.hotel = 0, 0, 0, 0
	_, length := dutySpan(ds.actual)
	if length == 0 {
		_, length = dutySpan(ds.sched.Duty)
	}
	away := length
	// A duty that ends away from every base sleeps out.
	endsAway := false
	if to := ds.sched.To; to != "" && !r.IsBase(to) {
		endsAway = true
		away += layoverRest
	}
	lvl := countryLevel(r.Country)
	purser := true
	var flown crew.Duty
	if len(ds.actual.Legs) > 0 {
		flown = ds.actual
	} else {
		flown = crew.Duty{Report: ds.sched.Duty.Report, Release: ds.sched.Duty.Release}
	}
	bill := func(i int32, a crew.Assignment) {
		m := &r.Members[i]
		senior := m.Group == Captain
		if m.Group == FlightAttendant {
			senior, purser = purser, false
		}
		rules := r.Contracts[m.Group.unit()].Rules(m.Fleet, senior, m.Years)
		a.Junior = m.junior
		st := rules.Pay(a)
		ds.pay += st.Pay
		ds.premium += st.Premium
		ds.perDiem += st.PerDiem
		if endsAway {
			ds.hotel += int64(hotelNight * lvl * 100)
		}
	}
	called := map[int32]bool{}
	for _, i := range ds.called {
		called[i] = true
	}
	for _, i := range ds.crew {
		if called[i] {
			// A reserve called out is paid for what it flew, from its call.
			bill(i, crew.Assignment{Scheduled: flown, Flown: flown, Away: away})
			continue
		}
		bill(i, crew.Assignment{Scheduled: ds.sched.Duty, Flown: flown, Away: away})
	}
	for _, i := range ds.off {
		if r.Members[i].sick {
			continue // sick leave is the member's own bank, not the day's cost
		}
		// Timed out part way: protected to the schedule.
		bill(i, crew.Assignment{Scheduled: ds.sched.Duty, Flown: crew.Duty{Report: ds.sched.Duty.Report, Release: ds.sched.Duty.Release}, Away: length})
	}
}

// Costs is what the carrier's people have cost by the day's position, in
// minutes: a duty counts once its first leg is due off, a cancelled one
// once the cancellation would be announced, and the reserves' guarantee
// accrues through the day.
func (w *Workforce) Costs(carrier string, pos float64) (Costs, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.roster(carrier)
	if r == nil {
		return Costs{}, false
	}
	var c Costs
	for _, ds := range r.duties {
		if len(ds.sched.Duty.Legs) == 0 {
			continue
		}
		first := ds.sched.Duty.Legs[0].Depart.Sub(w.day).Minutes()
		cancelled := len(ds.actual.Legs) == 0
		if cancelled && pos < first-120 || !cancelled && pos < first {
			continue
		}
		r.price(ds)
		c.Duties++
		c.Pay += ds.pay
		c.Premium += ds.premium
		c.PerDiem += ds.perDiem
		c.Hotels += ds.hotel
		if cancelled {
			c.Protected += ds.pay
		}
	}
	frac := min(1, max(0, pos/1440))
	for i := range r.Members {
		m := &r.Members[i]
		if !m.Reserve || m.called || m.sick || !(m.line.On(w.dayN) || m.Standby) {
			continue
		}
		rules := r.Contracts[m.Group.unit()].Rules(m.Fleet, m.Group == Captain, m.Years)
		day := rules.ReserveDay()
		if m.Standby {
			day = day * 3 / 2
		}
		c.Reserves += int64(float64(day) * frac)
	}
	c.Grievances = int64(r.Grievances) * r.Contracts[Pilots].GrievanceCost
	c.Total = c.Pay + c.Premium + c.PerDiem + c.Hotels + c.Reserves + c.Grievances
	return c, true
}

// GroundCost is what a departure's disruption costs the station's staff
// under the ground agreement: the turn team kept on past the schedule
// for a delay beyond fifteen minutes, and the agents who rebook a
// cancelled flight's passengers, four minutes each, both at overtime.
func (w *Workforce) GroundCost(carrier string, seats, delayMin int, cancelled bool, booked int) int64 {
	w.mu.Lock()
	r := w.roster(carrier)
	var c *Contract
	if r != nil {
		c = r.Contracts[Ground]
	}
	country := ""
	if cr, ok := w.carriers[carrier]; ok {
		country = cr.Country
	}
	w.mu.Unlock()
	if c == nil {
		c = NewContract(carrier, country, Ground, w.day)
	}
	fl := FleetOf(seats, false)
	rate := float64(c.Rate(fl, true, 8)) * c.Pay.ExtensionPremium
	team := map[Fleet]float64{Regional: 4, Narrowbody: 7, Widebody: 14}[fl]
	var cost float64
	if cancelled {
		cost += float64(booked) * 4 / 60 * rate
	} else if delayMin > 15 {
		cost += team * float64(delayMin-15) / 60 * rate
	}
	return int64(cost)
}
