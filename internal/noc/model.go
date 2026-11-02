package noc

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/rights"
	"github.com/adamf/wholesky/internal/workforce"
	"github.com/adamf/wholesky/internal/world"
)

// The world's numbers for passengers who are let down: the share who take
// their money back rather than the alternative offered, by how badly.
const (
	refundSameDay   = 0.05 // rebooked the same day
	refundNextDay   = 0.35 // rebooked onto tomorrow
	refundNoOption  = 1.0  // nothing to rebook onto
	refundLongWait  = 0.10 // a delay that gives the right to a refund
	refundOvernight = 0.15 // a missed connection that runs overnight
	defaultLF       = 0.82
	// DefaultDelayCost is what a minute of departure delay past fifteen
	// costs beyond the crews and the passengers' rights -- fuel, aircraft
	// time, goodwill -- in cents: the world's number, of the order of
	// published cost-of-delay estimates for a narrowbody.
	DefaultDelayCost = 5000
	// DefaultConnecting is the share of passengers arriving at a carrier's
	// hub who connect onward.
	DefaultConnecting = 0.35
)

// Model is one carrier's day, ready to be played under scenarios.
type Model struct {
	World   *world.Manifest
	Carrier string
	Day     time.Time
	// Own is the world's own-delay model; nil for none.
	Own dayplan.Own
	// Roster is the carrier's crew as the day starts; each play staffs
	// its own copy. Nil plays the day with any crew and hub reserves.
	Roster *workforce.Workforce
	// Revenue and Booked are what each leg sold and to how many; nil
	// estimates them from LoadFactor and Fare.
	Revenue    func(world.Flight) int64
	Booked     func(world.Flight) int
	LoadFactor float64
	// Fare is the average fare on a leg, in cents, when Revenue is nil.
	Fare func(world.Flight) int64
	// Base, when set, are cancellations already in the day (a live
	// world's), which every play keeps.
	Base map[string]string
	// History, when set, are own departure delays from a recorded day;
	// each play draws every flight's own delay from them instead of Own.
	History []int
	// Fixed is weather already in force or past, played the same in every
	// play and never thinned for: a live day's morning.
	Fixed      []dayplan.Cell
	FixedStops []dayplan.GroundStop
	// DelayCostPerMin and Connecting override the defaults above; a
	// negative Connecting means none.
	DelayCostPerMin int64
	Connecting      float64

	flights  []world.Flight
	hubs     map[string]bool
	airports map[string]world.Airport
	carriers map[string]world.Carrier
	byPair   map[string][]world.Flight
}

// Prepare indexes the model; Run calls it.
func (m *Model) Prepare() error {
	if m.flights != nil {
		return nil
	}
	m.airports = map[string]world.Airport{}
	for _, a := range m.World.Airports {
		m.airports[a.IATA] = a
	}
	m.carriers = map[string]world.Carrier{}
	for _, c := range m.World.Carriers {
		m.carriers[c.Designator] = c
	}
	m.byPair = map[string][]world.Flight{}
	for _, f := range m.World.Flights {
		if f.Carrier == m.Carrier {
			m.flights = append(m.flights, f)
			m.byPair[f.From+f.To] = append(m.byPair[f.From+f.To], f)
		}
	}
	if len(m.flights) == 0 {
		return fmt.Errorf("%s flies nothing in this world", m.Carrier)
	}
	// Passengers connect where the carrier gathers its flights: its hub,
	// and any station that takes more than one arrival in twelve.
	m.hubs = map[string]bool{m.carriers[m.Carrier].Hub: true}
	arrivals := map[string]int{}
	for _, f := range m.flights {
		arrivals[f.To]++
	}
	for st, n := range arrivals {
		if n*12 >= len(m.flights) {
			m.hubs[st] = true
		}
	}
	for k := range m.byPair {
		fs := m.byPair[k]
		sort.Slice(fs, func(i, j int) bool { return fs[i].DepMin < fs[j].DepMin })
	}
	if m.LoadFactor == 0 {
		m.LoadFactor = defaultLF
	}
	if m.DelayCostPerMin == 0 {
		m.DelayCostPerMin = DefaultDelayCost
	}
	if m.Connecting == 0 {
		m.Connecting = DefaultConnecting
	}
	return nil
}

// Realisation is one draw of a scenario: what actually happens.
type Realisation struct {
	Cells     []dayplan.Cell
	Stops     []dayplan.GroundStop
	SickRate  float64
	Technical map[string]int // flight key: minutes late, or -1 cancelled
	// Trial seeds the draws that are not the scenario's: own delays from
	// history.
	Trial uint64
}

// Realise draws a scenario at the given uniforms, one per dimension.
func (m *Model) Realise(sc Scenario, u []float64, trial uint64) Realisation {
	r := Realisation{Technical: map[string]int{}, Trial: trial}
	pick := u[len(u)-1]
	for i, e := range sc.Events {
		v := u[i*dims : (i+1)*dims]
		if e.Probability > 0 && v[0] >= e.Probability {
			continue
		}
		onset := int(math.Round(e.Onset.Quantile(v[1])))
		dur := int(math.Round(math.Max(15, e.Duration.Quantile(v[2]))))
		switch e.Kind {
		case KindWeather:
			a := m.airports[e.Airport]
			f := math.Min(1, math.Max(0.05, e.Factor.Quantile(v[3])))
			c := dayplan.Cell{Name: nameOr(e.Name, e.Airport+"WX"), Lat: a.Lat, Lon: a.Lon, RadiusKM: e.RadiusKM, Start: onset, End: onset + dur, Factor: f}
			if e.RadiusKM <= 0 {
				c.Airports = []string{e.Airport}
			}
			r.Cells = append(r.Cells, c)
		case KindGroundStop:
			r.Stops = append(r.Stops, dayplan.GroundStop{Airport: e.Airport, Start: onset, End: onset + dur, Cause: "ATC"})
		case KindClosure:
			r.Stops = append(r.Stops, dayplan.GroundStop{Airport: e.Airport, Start: onset, End: onset + dur, Departures: true})
		case KindSickout:
			r.SickRate = math.Max(r.SickRate, e.Rate.Quantile(v[3]))
		case KindAOG:
			n := int(math.Round(e.Rate.Quantile(v[3])))
			var window []world.Flight
			for _, f := range m.flights {
				if f.DepMin >= onset && f.DepMin < onset+dur {
					window = append(window, f)
				}
			}
			for k := 0; k < n && len(window) > 0; k++ {
				x := int(hashU(pick, uint64(k), trial) % uint64(len(window)))
				f := window[x]
				if hashU(pick, uint64(k)+1000, trial)%2 == 0 {
					r.Technical[dayplan.Key(f)] = -1
				} else {
					r.Technical[dayplan.Key(f)] = 150
				}
			}
		}
	}
	return r
}

func nameOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func hashU(u float64, k, trial uint64) uint64 {
	h := math.Float64bits(u)*0x9E3779B97F4A7C15 ^ k*0xBF58476D1CE4E5B9 ^ trial*0x94D049BB133111EB
	h ^= h >> 31
	h *= 0xD6E8FEB86659FD93
	h ^= h >> 32
	return h
}

// Impact is what a day cost the carrier, in cents and counts.
type Impact struct {
	Flights      int `json:"flights"`
	Cancelled    int `json:"cancelled"`
	Delayed      int `json:"delayed"`
	DelayMin     int `json:"delay_min"`
	PaxDisrupted int `json:"pax_disrupted"`
	// PaxMisconnected are the hub's connecting passengers who missed the
	// onward flight because the inbound was late.
	PaxMisconnected int `json:"pax_misconnected"`
	PaxRebooked     int `json:"pax_rebooked"`
	PaxLost         int `json:"pax_lost"`
	// RevenueLost is the fares of passengers who did not fly with the
	// carrier after all: refunds, and those with nothing to rebook onto.
	RevenueLost  int64 `json:"revenue_lost"`
	Compensation int64 `json:"compensation"`
	Care         int64 `json:"care"`
	Crew         int64 `json:"crew"`
	CrewPremium  int64 `json:"crew_premium"`
	Ground       int64 `json:"ground"`
	// DelayCost is the minutes past fifteen at the world's rate.
	DelayCost int64 `json:"delay_cost"`
	// Total is the day's cost of disruption and crews: revenue lost plus
	// every cost.
	Total int64 `json:"total"`
	// Reserves and Timeouts are crew events.
	Reserves int `json:"reserves"`
	Timeouts int `json:"timeouts"`
}

// Metrics are the impact's fields by name, for the statistics.
func (i Impact) Metrics() map[string]float64 {
	return map[string]float64{
		"cancelled": float64(i.Cancelled), "delayed": float64(i.Delayed), "delay_min": float64(i.DelayMin),
		"pax_disrupted": float64(i.PaxDisrupted), "pax_lost": float64(i.PaxLost), "pax_misconnected": float64(i.PaxMisconnected), "delay_cost": float64(i.DelayCost),
		"revenue_lost": float64(i.RevenueLost), "compensation": float64(i.Compensation), "care": float64(i.Care),
		"crew": float64(i.Crew), "crew_premium": float64(i.CrewPremium), "ground": float64(i.Ground), "total": float64(i.Total),
		"reserves": float64(i.Reserves), "timeouts": float64(i.Timeouts),
	}
}

// sub is the part of the world one play needs: the carrier's flights, and
// every flight into or out of the airports the realisation touches, which
// compete with the carrier's for the slots.
func (m *Model) sub(r Realisation) *world.Manifest {
	touched := map[string]bool{}
	for _, c := range append(append([]dayplan.Cell(nil), m.Fixed...), r.Cells...) {
		for _, a := range c.Airports {
			touched[a] = true
		}
		if len(c.Airports) == 0 {
			for _, a := range m.World.Airports {
				if km(c.Lat, c.Lon, a.Lat, a.Lon) <= c.RadiusKM {
					touched[a.IATA] = true
				}
			}
		}
	}
	for _, s := range append(append([]dayplan.GroundStop(nil), m.FixedStops...), r.Stops...) {
		touched[s.Airport] = true
	}
	out := &world.Manifest{Airports: m.World.Airports, Carriers: m.World.Carriers}
	for _, f := range m.World.Flights {
		if f.Carrier == m.Carrier || touched[f.To] || touched[f.From] {
			out.Flights = append(out.Flights, f)
		}
	}
	return out
}

func km(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p := math.Pi / 180
	dLat, dLon := (lat2-lat1)*p, (lon2-lon1)*p
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*p)*math.Cos(lat2*p)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// Play runs the day once: the realisation's weather, stops, sick calls and
// technical aircraft, the decided policy's cancellations and standby
// crews, the world's own delays or ones drawn from history.
func (m *Model) Play(r Realisation, d Decided) Impact {
	cancel := d.Cancel
	sub := m.sub(r)
	own := m.Own
	if len(m.History) > 0 {
		own = m.historical(r.Trial)
	}
	tech := r.Technical
	wrapped := func(f world.Flight, day time.Time) (int, int) {
		dep, arr := 0, 0
		if own != nil {
			dep, arr = own(f, day)
		}
		if late, ok := tech[dayplan.Key(f)]; ok && late > 0 {
			dep, arr = dep+late, arr+late
		}
		return dep, arr
	}
	all := map[string]string{}
	for k, v := range m.Base {
		all[k] = v
	}
	for k, v := range cancel {
		all[k] = v
	}
	for k, late := range tech {
		if late < 0 {
			all[k] = "technical: aircraft unserviceable"
		}
	}
	cells := append(append([]dayplan.Cell(nil), m.Fixed...), r.Cells...)
	stops := append(append([]dayplan.GroundStop(nil), m.FixedStops...), r.Stops...)
	opts := dayplan.Options{Weather: cells, GroundStops: stops, Cancel: all}
	if len(cells) == 0 {
		opts.Calm = true
	}
	var roster *workforce.Workforce
	if m.Roster != nil {
		roster = m.Roster.Clone(m.Carrier)
		if r.SickRate > 0 {
			roster.SetSickRate(m.Carrier, r.SickRate, fmt.Sprint(r.Trial))
		}
		for base, n := range d.Standby {
			roster.AddStandby(m.Carrier, base, n)
		}
		opts.Staff = roster
	}
	plan := dayplan.BuildWith(sub, m.Day, wrapped, opts)
	return m.price(plan, roster)
}

// price is what the played day cost.
func (m *Model) price(plan *dayplan.Plan, roster *workforce.Workforce) Impact {
	var im Impact
	type leg struct {
		f    world.Flight
		fate dayplan.Flight
	}
	carrier := m.carriers[m.Carrier]
	spare := map[string]int{}
	booked := func(f world.Flight) int {
		if m.Booked != nil {
			return m.Booked(f)
		}
		return int(math.Round(float64(f.Seats) * m.LoadFactor))
	}
	fare := func(f world.Flight, n int) int64 {
		if m.Revenue != nil && n > 0 {
			if v := m.Revenue(f); v > 0 {
				return v / int64(n)
			}
		}
		if m.Fare != nil {
			return m.Fare(f)
		}
		return int64(8900 + f.KM*12)
	}
	var cancelled []leg
	for _, f := range m.flights {
		fate := plan.Of(f)
		im.Flights++
		if fate.Cancelled {
			im.Cancelled++
			cancelled = append(cancelled, leg{f, fate})
			continue
		}
		spare[dayplan.Key(f)] = max(0, f.Seats-booked(f))
		if fate.DepDelay > 15 {
			im.Delayed++
			im.DelayMin += fate.DepDelay
			im.DelayCost += int64(fate.DepDelay-15) * m.DelayCostPerMin
		}
	}
	regime := func(f world.Flight) (rights.Regime, bool) {
		from, to := m.airports[f.From], m.airports[f.To]
		reg := rights.Of(from.Country, to.Country, carrier.Country)
		return reg, rights.Intra(reg, from.Country, to.Country)
	}
	cause := func(f world.Flight, fate dayplan.Flight) rights.Cause {
		switch {
		case fate.Cancelled && strings.HasPrefix(fate.Reason, "crew"):
			return rights.Crew
		case fate.Cancelled && strings.HasPrefix(fate.Reason, "technical"):
			return rights.Technical
		case fate.Cancelled && (fate.Code == "B" || strings.HasPrefix(fate.Reason, "noc")):
			return rights.Weather
		case fate.Cancelled && fate.Code == "C":
			return rights.ATC
		case fate.Cancelled:
			return rights.Carrier
		}
		reserve := 0
		if fate.Reserve {
			reserve = dayplan.ReserveCall
		}
		if fate.ATFM > 0 && fate.ATFM >= max(fate.Own, reserve) {
			if fate.Cause != nil && strings.HasPrefix(fate.Cause.String(), "W") {
				return rights.Weather
			}
			return rights.ATC
		}
		if reserve > 0 {
			return rights.Crew
		}
		return rights.Carrier
	}
	// Delays first: the passengers on board.
	for _, f := range m.flights {
		fate := plan.Of(f)
		if fate.Cancelled || fate.ArrDelay < 120 {
			continue
		}
		n := booked(f)
		reg, domestic := regime(f)
		owed := rights.For(rights.Disruption{Regime: reg, KM: f.KM, Intra: domestic,
			Cause: cause(f, fate), ArrDelay: fate.ArrDelay, DepDelay: fate.DepDelay, Overnight: f.DepMin+fate.DepDelay >= 24*60, Domestic: domestic})
		im.PaxDisrupted += n
		im.Compensation += int64(n) * owed.Compensation
		im.Care += int64(n) * owed.Care
		if owed.RefundRight {
			lost := int(math.Round(float64(n) * refundLongWait))
			im.PaxLost += lost
			im.RevenueLost += int64(lost) * fare(f, n)
		}
	}
	// Missed connections at the hubs: of the passengers connecting off a
	// late arrival, more miss the onward flight the later it is -- none at
	// half an hour, all past two and a half. They go on the next flight,
	// about ninety minutes later, or tomorrow when that is past 21:00, and
	// the law weighs the delay at their final destination (the Court of
	// Justice in Folkerts, C-11/11).
	if m.Connecting > 0 {
		for _, f := range m.flights {
			fate := plan.Of(f)
			if fate.Cancelled || !m.hubs[f.To] || fate.ArrDelay <= 30 {
				continue
			}
			share := m.Connecting * math.Min(1, float64(fate.ArrDelay-30)/120)
			n := int(math.Round(float64(booked(f)) * share))
			if n == 0 {
				continue
			}
			reg, domestic := regime(f)
			final := fate.ArrDelay + 90
			overnight := f.ArrMin+final >= 21*60+90
			lostShare := refundSameDay
			if overnight {
				final, lostShare = 24*60, refundOvernight
			}
			owed := rights.For(rights.Disruption{Regime: reg, KM: f.KM, Intra: domestic, Cause: cause(f, fate), ArrDelay: final,
				DepDelay: final, Overnight: overnight, Domestic: domestic})
			lost := int(math.Round(float64(n) * lostShare))
			im.PaxMisconnected += n
			im.PaxDisrupted += n
			im.PaxRebooked += n - lost
			im.PaxLost += lost
			im.RevenueLost += int64(lost) * fare(f, booked(f))
			im.Compensation += int64(n) * owed.Compensation
			im.Care += int64(n) * owed.Care
		}
	}
	// Cancellations: each passenger onto the carrier's next flight over the
	// pair with a seat, today, else tomorrow's.
	sort.Slice(cancelled, func(i, j int) bool { return cancelled[i].f.DepMin < cancelled[j].f.DepMin })
	for _, c := range cancelled {
		f := c.f
		n := booked(f)
		im.PaxDisrupted += n
		reg, domestic := regime(f)
		left := n
		pf := fare(f, n)
		cs := cause(f, c.fate)
		for _, alt := range m.byPair[f.From+f.To] {
			if left == 0 {
				break
			}
			if alt.DepMin <= f.DepMin {
				continue
			}
			k := dayplan.Key(alt)
			take := min(left, spare[k])
			if take == 0 {
				continue
			}
			spare[k] -= take
			left -= take
			afate := plan.Of(alt)
			arrDelay := alt.ArrMin + afate.ArrDelay - f.ArrMin
			owed := rights.For(rights.Disruption{Regime: reg, KM: f.KM, Cause: cs, Cancelled: true, Rerouted: true, ArrDelay: arrDelay,
				DepDelay: alt.DepMin - f.DepMin, Domestic: domestic, Intra: domestic})
			lost := int(math.Round(float64(take) * refundSameDay))
			im.PaxRebooked += take - lost
			im.PaxLost += lost
			im.RevenueLost += int64(lost) * pf
			im.Compensation += int64(take-lost) * owed.Compensation
			im.Care += int64(take) * owed.Care
		}
		if left == 0 {
			continue
		}
		// Tomorrow's flights over the pair: a night away and a day late.
		share := refundNextDay
		if len(m.byPair[f.From+f.To]) == 0 {
			share = refundNoOption
		}
		owed := rights.For(rights.Disruption{Regime: reg, KM: f.KM, Cause: cs, Cancelled: true, Rerouted: share < 1, ArrDelay: 24 * 60,
			DepDelay: 24 * 60, Overnight: true, Domestic: domestic, Intra: domestic})
		lost := int(math.Round(float64(left) * share))
		im.PaxRebooked += left - lost
		im.PaxLost += lost
		im.RevenueLost += int64(lost) * pf
		im.Compensation += int64(left) * owed.Compensation
		im.Care += int64(left-lost) * owed.Care
	}
	if roster != nil {
		if c, ok := roster.Costs(m.Carrier, 24*60); ok {
			im.Crew = c.Pay + c.PerDiem + c.Hotels + c.Reserves + c.Grievances
			im.CrewPremium = c.Premium
		}
		for _, f := range m.flights {
			fate := plan.Of(f)
			im.Ground += roster.GroundCost(m.Carrier, f.Seats, fate.DepDelay, fate.Cancelled, booked(f))
		}
	}
	_, _, sum := plan.Picture()
	im.Reserves, im.Timeouts = sum.Reserves, sum.TimedOut
	im.Total = im.RevenueLost + im.Compensation + im.Care + im.Crew + im.CrewPremium + im.Ground + im.DelayCost
	return im
}

// historical is an own-delay model drawn from the recorded day: each
// flight's delay is one of the record's, picked by the flight and the
// trial. A play is a day that went like some recorded day's flights did.
func (m *Model) historical(trial uint64) dayplan.Own {
	return func(f world.Flight, _ time.Time) (int, int) {
		h := fnv(dayplan.Key(f), trial)
		d := max(0, m.History[h%uint64(len(m.History))])
		return d, d
	}
}

func fnv(s string, salt uint64) uint64 {
	h := uint64(1469598103934665603) ^ salt*0x9E3779B97F4A7C15
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	h ^= h >> 29
	return h
}
