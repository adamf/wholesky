package sim

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/adamf/wholesky/internal/airport"
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/fuel"
	"github.com/adamf/wholesky/internal/growth"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// How a carrier grows in this world.
//
// A day of the world is one day of flying, and it comes round again.
// Each time it ends, every carrier's day is closed into its book: what
// it earned and spent goes to its cash, and the book keeps the days
// (internal/growth). A carrier that wants to be bigger asks what a new
// service would do, which the world works out from the market it would
// be sold into and the costs the scorecard already counts, and orders
// it. The order takes the lessors' deposits and the price of any slots
// out of the carrier's cash and comes due some weeks later, as a
// schedule change does. A schedule changes when the world starts: the
// services that have come due are flown from the next start on.

// openingShare is the cash a carrier begins with, as a share of a year's
// revenue at its schedule. The four largest US carriers ended 2019 with
// liquidity of 13 to 23 per cent of that year's revenue, by their annual
// reports. An estimate inside the range.
const openingShare = 0.18

// The weeks between a decision and the flying: a schedule change takes
// two to ten, and a station the carrier does not have yet one to four
// months.
const (
	leadDays       = 42
	leadNewStation = 90
)

// bookOf is a carrier's book, opened the first time it is asked for.
// Called with the economy's lock held.
func (s *Sim) bookOf(code string) *growth.Book {
	e := s.econ
	if b := e.books[code]; b != nil {
		return b
	}
	var day float64
	for _, f := range s.Flights[code] {
		day += float64(f.Seats) * 0.82 * float64(tariff.AverageFare(s.tariff, f.Carrier, f.From, f.To))
	}
	open := int64(day * 365 * openingShare)
	b := &growth.Book{Opening: open, Cash: open}
	e.books[code] = b
	return b
}

// closeDay closes every carrier's day into its book. It runs as the day
// ends, before the calendar moves.
func (s *Sim) closeDay() {
	if s.econ == nil {
		return
	}
	w := seatWorld{s}
	pos, _ := w.Clock()
	day := int(s.econ.day.Load())
	for code := range s.Tenants {
		if s.External(code) {
			continue
		}
		sc := w.score(code)
		d := growth.Day{Day: day, Revenue: sc.Revenue, Cost: sc.Cost, Flown: sc.Flown, Passengers: sc.Passengers, Seats: sc.Seats}
		for _, f := range s.Flights[code] {
			if st := w.flightStatus(f, s.fate.Of(f), pos); st != "departed" && st != "landed" {
				continue
			}
			booked, seats := w.booked(f)
			d.ASK += float64(seats) * float64(f.KM)
			d.RPK += float64(booked) * float64(f.KM)
		}
		s.econ.mu.Lock()
		s.bookOf(code).Close(d)
		s.econ.mu.Unlock()
	}
	s.answerEntrants()
	s.saveState()
}

// answerWindow is the days after a rival starts a service in which an
// incumbent still answers it: the record is 1 to 3 months.
const answerWindow = 90

// answerEntrants is the carriers' network planning at the end of a day.
// A rival has begun a service on a route that a carrier flies from its
// hub. If the rival is weak, the carrier orders more flights on the
// route (growth.Answer), provided they would earn what they cost to fly
// and it has the cash to start them. The flights come due after the lead
// time of any order, 6 weeks, and the world takes them into its schedule
// at the end of that day.
//
// A carrier answers each of a rival's services once. An answer is not
// itself answered, so 2 carriers do not add flights against each other
// without end. A carrier whose seat has taken network planning in hand
// decides for itself.
func (s *Sim) answerEntrants() {
	if s.econ == nil || !s.runsDesks() {
		return
	}
	day := int(s.econ.day.Load())
	type entry struct {
		rival string
		o     growth.Order
		e     growth.Entrant
	}
	var entries []entry
	answered := map[string]bool{} // "carrier/order"
	s.econ.mu.Lock()
	for code, b := range s.econ.books {
		for _, o := range b.Orders {
			if o.Answers != "" {
				answered[code+"/"+o.Answers] = true
				continue
			}
			if !o.Flying || day-o.Due > answerWindow {
				continue
			}
			e := growth.Entrant{Daily: o.Daily, Aircraft: len(s.econ.fleets[code]), Cash: b.Cash}
			if t := b.Trend(30); t.Days > 0 && t.Cost > t.Revenue {
				e.Loss = (t.Cost - t.Revenue) / int64(t.Days)
			}
			entries = append(entries, entry{code, o, e})
		}
	}
	s.econ.mu.Unlock()
	sort.Slice(entries, func(i, j int) bool { return entries[i].o.ID < entries[j].o.ID })
	for _, en := range entries {
		var codes []string
		for code := range s.Tenants {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			hub := s.carriers[code].Hub
			if code == en.rival || s.External(code) || answered[code+"/"+en.o.ID] || (hub != en.o.From && hub != en.o.To) ||
				(s.Airline != nil && s.Airline.Manual(code, "network")) {
				continue
			}
			far := en.o.To
			if hub == en.o.To {
				far = en.o.From
			}
			own := 0
			types := map[string]int{}
			for _, f := range s.Flights[code] {
				if f.From == hub && f.To == far {
					own++
					types[f.Equipment]++
				}
			}
			add := growth.Answer(own, en.e)
			if add == 0 {
				continue
			}
			typ := ""
			for t, n := range types {
				if n > types[typ] || (n == types[typ] && t < typ) {
					typ = t
				}
			}
			said, err := s.orderFor(code, hub, far, typ, add, en.o.ID)
			if err != nil {
				s.log.Info("no answer in flights to an entrant", "carrier", code, "rival", en.rival, "route", hub+"-"+far, "why", err.Error())
				continue
			}
			answered[code+"/"+en.o.ID] = true
			s.log.Info("answered an entrant with flights", "carrier", code, "rival", en.rival, "route", hub+"-"+far, "order", said)
		}
	}
}

// financeView is a carrier's money across days, for its seat.
type financeView struct {
	Day     int            `json:"day"`
	Opening int64          `json:"opening_cash"`
	Cash    int64          `json:"cash"`
	All     growth.Trend   `json:"all_days"`
	Week    growth.Trend   `json:"last_7_days"`
	Days    []growth.Day   `json:"days"`
	Orders  []growth.Order `json:"orders"`
	Note    string         `json:"note"`
}

// Finance implements airline.Growth.
func (w seatWorld) Finance(carrier string) (any, bool) {
	s := w.s
	code := strings.ToUpper(carrier)
	if s.econ == nil || len(s.Flights[code]) == 0 {
		return nil, false
	}
	s.econ.mu.Lock()
	defer s.econ.mu.Unlock()
	b := s.bookOf(code)
	days := b.Days
	if len(days) > 30 {
		days = days[len(days)-30:]
	}
	return financeView{Day: int(s.econ.day.Load()), Opening: b.Opening, Cash: b.Cash, All: b.Trend(0), Week: b.Trend(7),
		Days: append([]growth.Day{}, days...), Orders: append([]growth.Order{}, b.Orders...),
		Note: "Money is in cents. A day is closed into the book when it ends. Opening cash is 18% of a year's revenue at the filed schedule. A world on one machine starts again at the end of the day an order comes due and flies it. A world on several machines flies it from its next start."}, true
}

// marketFor is what is known of a market a service would be sold into.
func (s *Sim) marketFor(code, from, to string, km int) growth.Market {
	m := growth.Market{}
	a, b := from, to
	if a > b {
		a, b = b, a
	}
	flights := map[string]int{}
	var seats float64
	for _, f := range s.commerce.pair[from+"/"+to] {
		flights[f.Carrier]++
		seats += float64(f.Seats)
	}
	for c, n := range flights {
		if c == code {
			continue
		}
		m.Rivals = append(m.Rivals, growth.Rival{Carrier: c, Model: equipment.ModelOf(c), Daily: float64(n),
			Level: tariff.Level(c) * s.tariff.EffectiveMultiplier(c, a, b)})
	}
	sort.Slice(m.Rivals, func(i, j int) bool { return m.Rivals[i].Carrier < m.Rivals[j].Carrier })
	// The Consumer Airfare Report measures the travellers of the largest
	// US airport pairs. Elsewhere the seats flown, at the load factor the
	// world fills to and with three in four of them local, stand in.
	for _, p := range refdata.AirportPairs() {
		if (p.City1 == from && p.City2 == to) || (p.City1 == to && p.City2 == from) {
			m.Passengers, m.Measured = p.Passengers, true
			m.Fare = int64(p.Fare * 100)
			break
		}
	}
	if !m.Measured {
		m.Passengers = seats * 0.82 * 0.75
		if m.Passengers == 0 {
			// Nobody flies it and no report counts it: the travellers who
			// make the trip some other way, by the gravity of the two
			// airports' own traffic. An estimate of an estimate, and the
			// projection says so.
			m.Passengers = math.Sqrt(float64(s.econ.deps[from])*float64(s.econ.deps[to])) * 1.5 / math.Max(1, math.Sqrt(float64(km)/1000))
		}
	}
	if m.Fare == 0 {
		m.Fare = int64(float64(s.baseFare(from, to, km)) * tariff.MixRatio())
	}
	return m
}

// Growth implements airline.Growth: what a service would do.
func (w seatWorld) Growth(carrier, from, to, typ string, daily int) (any, error) {
	p, m, c, err := w.s.proposal(carrier, from, to, typ, daily)
	if err != nil {
		return nil, err
	}
	out := growth.Project(p, m, c)
	lead := w.s.leadFor(p.Carrier, p.From, p.To)
	out.Notes = append(out.Notes, fmt.Sprintf("Ordered today it comes due in %d days and is flown from the next start of the world after that.", lead))
	return map[string]any{"service": map[string]any{"carrier": p.Carrier, "from": p.From, "to": p.To, "km": p.KM, "type": p.Aircraft.IATA, "aircraft_name": p.Aircraft.Name,
		"seats": p.Aircraft.Seats, "daily": p.Daily, "block_min": p.BlockMin, "turn_min": p.TurnMin}, "market": m, "projection": out}, nil
}

// leadFor is the days from an order to its flying.
func (s *Sim) leadFor(code, from, to string) int {
	has := map[string]bool{}
	for _, f := range s.Flights[code] {
		has[f.From] = true
	}
	if has[from] && has[to] {
		return leadDays
	}
	return leadNewStation
}

// proposal builds the service, the market and the costs for a carrier.
func (s *Sim) proposal(carrier, from, to, typ string, daily int) (growth.Proposal, growth.Market, growth.Costs, error) {
	code, from, to := strings.ToUpper(carrier), strings.ToUpper(from), strings.ToUpper(to)
	var p growth.Proposal
	var c growth.Costs
	if s.econ == nil || s.commerce == nil {
		return p, growth.Market{}, c, fmt.Errorf("this world keeps no economy")
	}
	if _, ok := s.airports[from]; !ok {
		return p, growth.Market{}, c, fmt.Errorf("no airport %s in this world", from)
	}
	if _, ok := s.airports[to]; !ok || from == to {
		return p, growth.Market{}, c, fmt.Errorf("no airport %s in this world to fly to from %s", to, from)
	}
	ac, ok := refdata.AircraftType(strings.ToUpper(typ))
	if !ok {
		return p, growth.Market{}, c, fmt.Errorf("no aircraft type %q: see the lessors' offers in the economy", typ)
	}
	if daily <= 0 || daily > 20 {
		return p, growth.Market{}, c, fmt.Errorf("a service is 1 to 20 round trips a day")
	}
	km := s.tariff.Distance(from, to)
	if km <= 0 {
		return p, growth.Market{}, c, fmt.Errorf("the distance from %s to %s is not known", from, to)
	}
	model := equipment.ModelOf(code)
	p = growth.Proposal{Carrier: code, Model: model, Level: tariff.Level(code) * s.tariff.Multiplier(code), From: from, To: to, KM: km,
		BlockMin: world.BlockMinutes(float64(km)), TurnMin: world.ScheduledTurn(ac.IATA, ac.Seats, model, km), Aircraft: ac, Daily: daily}
	m := s.marketFor(code, from, to, km)

	// One flight's costs, by the figures the scorecard uses, at four
	// seats in five sold.
	e := s.econ
	f := world.Flight{Carrier: code, Number: "0000", From: from, To: to, BlockMin: p.BlockMin, Equipment: ac.IATA, Seats: ac.Seats, KM: km}
	pax := ac.Seats * 8 / 10
	hours := float64(p.BlockMin) / 60
	age := 8.0
	c.Fuel = cents(fuel.Gallons(fuel.Burn(ac.IATA, ac.Seats, km)) * (e.fuelPrice(from) + e.fuelPrice(to)) / 2)
	rent := equipment.Rent(ac, age)
	c.Maintenance = cents(equipment.MaintenancePerBlockHour(ac, age)*hours + rent*0.44/300*math.Max(0.5, hours-0.3))
	turn := airport.TurnCost(airport.Index(from, e.region[from], e.deps[from]), ac.MTOW, pax)/2 +
		airport.TurnCost(airport.Index(to, e.region[to], e.deps[to]), ac.MTOW, pax)/2
	c.Airports = cents(turn * airportDeal(model))
	c.Navigation = cents(airport.EnRoute(km, ac.MTOW, e.region[from], e.region[to]))
	c.Handling = cents(groundHandling(ac, model))
	c.Crew = int64(p.BlockMin) * costPerBlockMinCrew
	// What a ticket costs to sell: the carrier's own average so far, or
	// its channels' at the market's fare.
	if acct := s.accountOf(code); acct.Tickets > 100 {
		c.Passenger = acct.Distribution() / int64(acct.Tickets)
	} else {
		c.Passenger = s.costOfSale(nil, code, 1, m.Fare, m.Fare*112/100).Total()
	}
	c.Rent, c.Deposit = rent, rent*2
	if airport.Coordinated(from) {
		c.SlotFrom = airport.SlotValue(from, f.DepMin+9*60, ac.Widebody())
	}
	if airport.Coordinated(to) {
		c.SlotTo = airport.SlotValue(to, 12*60, ac.Widebody())
	}
	return p, m, c, nil
}

// order places a service on a carrier's book: the cash to start it goes
// out now and the service comes due after its lead time.
func (s *Sim) order(carrier, from, to, typ string, daily int) (string, error) {
	return s.orderFor(carrier, from, to, typ, daily, "")
}

// orderFor is order, for a service that answers a rival's order.
func (s *Sim) orderFor(carrier, from, to, typ string, daily int, answers string) (string, error) {
	p, m, c, err := s.proposal(carrier, from, to, typ, daily)
	if err != nil {
		return "", err
	}
	proj := growth.Project(p, m, c)
	if answers != "" && !proj.Covers() {
		return "", fmt.Errorf("%d more a day %s-%s would earn %d dollars a day and cost %d to fly", daily, p.From, p.To, proj.Revenue/100, (proj.Cost-proj.Lines["aircraft rent"])/100)
	}
	day := int(s.econ.day.Load())
	o := growth.Order{From: p.From, To: p.To, Type: p.Aircraft.IATA, Age: 8, Daily: daily, Aircraft: proj.Aircraft,
		Placed: day, Due: day + s.leadFor(p.Carrier, p.From, p.To), Paid: proj.Start, Answers: answers}
	s.econ.mu.Lock()
	b := s.bookOf(p.Carrier)
	o.ID = fmt.Sprintf("%s-%d-%d", p.Carrier, day, len(b.Orders)+1)
	err = b.Place(o)
	cash := b.Cash
	s.econ.mu.Unlock()
	if err != nil {
		return "", err
	}
	s.saveState()
	return fmt.Sprintf("ordered %s: %d daily %s-%s on %d %s, %d dollars paid to start, due on day %d; cash now %d dollars. A world on one machine starts again at the end of that day and flies it; a world on several flies it from its next start.",
		o.ID, daily, o.From, o.To, o.Aircraft, p.Aircraft.Name, o.Paid/100, o.Due, cash/100), nil
}

// GrowManifest adds to a world's schedule the services its carriers
// have ordered that have come due, from a state file's contents: the
// world's own state and the peers' it keeps. It is called before a
// world is built, and by the world compiler for a world on several
// machines, which must all start from one file. It returns the flights
// added.
func GrowManifest(m *world.Manifest, state []byte) int {
	var st worldState
	if len(state) == 0 || json.Unmarshal(state, &st) != nil {
		return 0
	}
	day := int64(0)
	books := map[string]*growth.Book{}
	take := func(e *savedEconomy) {
		if e == nil {
			return
		}
		if e.Day > day {
			day = e.Day
		}
		for code, b := range e.Books {
			books[code] = b
		}
	}
	take(st.Economy)
	for _, raw := range st.Peers {
		var peer worldState
		if json.Unmarshal(raw, &peer) == nil {
			take(peer.Economy)
		}
	}
	if len(books) == 0 {
		return 0
	}
	place := map[string]world.Airport{}
	for _, a := range m.Airports {
		place[a.IATA] = a
	}
	taken := map[string]bool{}
	by := map[string][]world.Flight{}
	for _, f := range m.Flights {
		taken[f.Tail] = true
		by[f.Carrier] = append(by[f.Carrier], f)
	}
	codes := make([]string, 0, len(books))
	for code := range books {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	added := 0
	for _, code := range codes {
		for _, o := range books[code].Orders {
			from, okA := place[o.From]
			to, okB := place[o.To]
			ac, okT := refdata.AircraftType(o.Type)
			if int64(o.Due) > day || !okA || !okB || !okT || len(by[code]) == 0 {
				continue
			}
			km := int(world.Haversine(from.Lat, from.Lon, to.Lat, to.Lon))
			model := equipment.ModelOf(code)
			fs := growth.Schedule(code, o, by[code], taken, km, world.BlockMinutes(float64(km)), world.ScheduledTurn(ac.IATA, ac.Seats, model, km), ac.Seats)
			m.Flights = append(m.Flights, fs...)
			by[code] = append(by[code], fs...)
			added += len(fs)
		}
	}
	return added
}
