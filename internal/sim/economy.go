package sim

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/airport"
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/fuel"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/world"
)

// The world's money side: what it costs a carrier to fly its day. Every
// number has a source in internal/refdata or in the package that uses it
// -- the fuel price is the Energy Information Administration's, the burn
// EUROCONTROL's, the rents the appraisers', the airport charges the
// Jacobs review's -- and the scorecard adds them up flight by flight.
//
// The world has a calendar. Day 0 is the date the economy starts on; each
// time the simulated day wraps the calendar moves on a day, the fuel
// price with it. A world started inside the fuel record lives the record.

// economy is one world's economy.
type economy struct {
	fuel *fuel.Market
	// day is the world's day number since it began.
	day atomic.Int64
	// deps is each airport's departures a day; region is each airport's
	// part of the world.
	deps   map[string]int
	region map[string]string

	mu     sync.Mutex
	fleets map[string]map[string]equipment.Aircraft // carrier -> tail
	index  map[int]float64                          // the averaged fuel index by day
	hedges map[string][]fuel.Hedge                  // carrier
	burn   map[string]float64                       // carrier -> gallons a day as scheduled
}

// groundHandling is what a handler charges for a turnaround, in dollars:
// ramp, passenger services, pushback. An estimate by size of aircraft;
// no handler publishes a tariff. A low-cost carrier boards by stairs,
// handles itself where it is large and buys little else.
func groundHandling(a refdata.Aircraft, model equipment.Model) float64 {
	v := 1800.0
	switch a.Body {
	case "W":
		v = 6500
	case "T":
		v = 600
	case "R":
		v = 800
	}
	switch model {
	case equipment.LowCost:
		v *= 0.6
	case equipment.UltraLowCost:
		v *= 0.4
	}
	return v
}

// airportDeal is the share of an airport's published charges a carrier
// of a kind pays. The Jacobs index is the tariff; low-cost carriers do
// not pay the tariff. easyJet's airports and ground handling cost it
// about 17 pounds a seat in 2019 on sectors of 1,100 km, and Ryanair's
// about 9 euros a passenger; the Charleroi terms the Commission
// published were one euro for handling and four a passenger back in
// marketing support. The shares are estimates set to those.
func airportDeal(model equipment.Model) float64 {
	switch model {
	case equipment.LowCost:
		return 0.65
	case equipment.UltraLowCost:
		return 0.40
	}
	return 1
}

func (s *Sim) initEconomy(m *world.Manifest, opts Options) {
	start := opts.EconomyStart
	if start.IsZero() {
		start = sellingDate(m)
	}
	e := &economy{fuel: fuel.New(start, m.Seed), deps: map[string]int{}, region: map[string]string{},
		fleets: map[string]map[string]equipment.Aircraft{}, index: map[int]float64{}, hedges: map[string][]fuel.Hedge{}, burn: map[string]float64{}}
	for _, a := range m.Airports {
		e.region[a.IATA] = refdata.Region(a.Country, a.TZ)
	}
	for _, f := range m.Flights {
		e.deps[f.From]++
		tails := e.fleets[f.Carrier]
		if tails == nil {
			tails = map[string]equipment.Aircraft{}
			e.fleets[f.Carrier] = tails
		}
		tail := f.Tail
		if tail == "" {
			tail = "-" + f.Number + f.From
		}
		// A tail is the largest type it flies: a schedule compiled before
		// tails kept one type may chain several.
		if ac, ok := tails[tail]; !ok || f.Seats > ac.Seats {
			tails[tail] = equipment.New(f.Carrier, tail, f.Equipment, f.Seats)
		}
		e.burn[f.Carrier] += fuel.Gallons(fuel.Burn(f.Equipment, f.Seats, f.KM))
	}
	s.econ = e
	// The carriers that hedge had their hedges in place before the world
	// began, bought at the price the market expected then.
	carriers := make([]string, 0, len(e.fleets))
	for code := range e.fleets {
		carriers = append(carriers, code)
	}
	sort.Strings(carriers)
	for _, code := range carriers {
		c := s.carriers[code]
		share := hedgePolicy(code, e.region[c.Hub])
		if share <= 0 {
			continue
		}
		h := e.fuel.Quote(fuel.Swap, 0, 0, 365, e.burn[code]*share, 0)
		h.Bought = -1
		e.hedges[code] = append(e.hedges[code], h)
	}
}

// hedgePolicy is the share of a year's fuel a carrier on autopilot has
// fixed, by what carriers of its kind and region do: European low-cost
// carriers fix 75 to 90 per cent of the year ahead and European network
// carriers 60 to 80 per cent of the next quarter falling away after;
// Asian network carriers 30 to 50; Australasian 80 for a half-year; the
// US carriers stopped hedging about 2016. From the carriers' reports.
func hedgePolicy(carrier, region string) float64 {
	lcc := equipment.ModelOf(carrier) != equipment.Network
	switch region {
	case refdata.Europe:
		if lcc {
			return 0.80
		}
		return 0.50
	case refdata.Asia:
		return 0.35
	case refdata.Oceania:
		return 0.60
	case refdata.MiddleEast, refdata.NorthAmerica:
		return 0
	}
	return 0.15
}

// Day is the world's day number and its calendar date.
func (s *Sim) Day() (int, time.Time) {
	if s.econ == nil {
		return 0, s.BookingDate
	}
	d := int(s.econ.day.Load())
	return d, s.econ.fuel.Date(d)
}

// fuelIndex is the averaged index on a day, remembered: a scorecard asks
// for it once a flight.
func (e *economy) fuelIndex(day int) float64 {
	e.mu.Lock()
	v, ok := e.index[day]
	e.mu.Unlock()
	if ok {
		return v
	}
	v = e.fuel.Index(day)
	e.mu.Lock()
	if len(e.index) > 400 {
		e.index = map[int]float64{}
	}
	e.index[day] = v
	e.mu.Unlock()
	return v
}

// fuelPrice is what a gallon costs a carrier at an airport today.
func (e *economy) fuelPrice(iata string) float64 {
	day := int(e.day.Load())
	return e.fuelIndex(day) + e.fuel.Basis(day, e.region[iata]) + fuel.IntoPlane(e.deps[iata])
}

// aircraftOf is the aircraft that flies a leg.
func (e *economy) aircraftOf(f world.Flight) equipment.Aircraft {
	tail := f.Tail
	if tail == "" {
		tail = "-" + f.Number + f.From
	}
	e.mu.Lock()
	ac, ok := e.fleets[f.Carrier][tail]
	e.mu.Unlock()
	if !ok {
		ac = equipment.New(f.Carrier, tail, f.Equipment, f.Seats)
	}
	return ac
}

// flightCost is what one flown flight cost, in cents, by line.
type flightCost struct {
	Fuel, Maintenance, Airports, Navigation, Handling, Reserves int64
	Gallons                                                     float64
}

func (c flightCost) total() int64 {
	return c.Fuel + c.Maintenance + c.Airports + c.Navigation + c.Handling + c.Reserves
}

// flightCost prices one flight: the fuel it burned at the price of the
// station it left, the maintenance its block hours used, half a
// turnaround's charges at each end, the airspace between, the handler at
// the departure station, and for a leased aircraft the reserves its
// flight hours owe the lessor.
func (e *economy) flightCost(f world.Flight, passengers int) flightCost {
	ac := e.aircraftOf(f)
	t := equipment.Type(f.Equipment, f.Seats)
	block := f.BlockMin
	if block <= 0 {
		block = f.ArrMin - f.DepMin
	}
	hours := float64(block) / 60
	var c flightCost
	c.Gallons = fuel.Gallons(fuel.Burn(f.Equipment, f.Seats, f.KM))
	c.Fuel = cents(c.Gallons * e.fuelPrice(f.From))
	c.Maintenance = cents(ac.Maintenance * hours)
	from, to := e.region[f.From], e.region[f.To]
	model := equipment.ModelOf(f.Carrier)
	turn := airport.TurnCost(airport.Index(f.From, from, e.deps[f.From]), t.MTOW, passengers)/2 +
		airport.TurnCost(airport.Index(f.To, to, e.deps[f.To]), t.MTOW, passengers)/2
	c.Airports = cents(turn * airportDeal(model))
	c.Navigation = cents(airport.EnRoute(f.KM, t.MTOW, from, to))
	c.Handling = cents(groundHandling(t, model))
	if ac.Leased {
		c.Reserves = cents(ac.Reserves * math.Max(0.5, hours-0.3))
	}
	return c
}

func cents(dollars float64) int64 { return int64(math.Round(dollars * 100)) }

// fixedCosts is what a carrier's fleet costs for the share of the day
// that has passed, in cents: rent on the leased aircraft, and
// depreciation and interest on the owned ones.
func (e *economy) fixedCosts(carrier string, dayShare float64) (rent, ownership int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, ac := range e.fleets[carrier] {
		day := ac.Rent * 12 / 365 * dayShare
		if ac.Leased {
			rent += cents(day)
		} else {
			ownership += cents(day)
		}
	}
	return rent, ownership
}

// hedgeResult is what a carrier's hedges pay it for the share of the day
// that has passed, in cents: negative when they cost it.
func (e *economy) hedgeResult(carrier string, dayShare float64) int64 {
	day := int(e.day.Load())
	e.mu.Lock()
	hs := append([]fuel.Hedge(nil), e.hedges[carrier]...)
	e.mu.Unlock()
	var v float64
	for _, h := range hs {
		v += h.Settle(e.fuel, day)
	}
	return cents(v * dayShare)
}

// hedge takes a new position for a carrier: a share of its daily burn,
// from tomorrow, for the days given.
func (s *Sim) hedge(carrier string, kind fuel.Kind, percent, days int) (string, error) {
	e := s.econ
	if e == nil {
		return "", fmt.Errorf("this world keeps no economy")
	}
	if percent <= 0 || percent > 100 {
		return "", fmt.Errorf("a hedge covers 1 to 100 per cent of the fuel")
	}
	if days <= 0 {
		days = 180
	}
	if days > 730 {
		return "", fmt.Errorf("no dealer quotes past two years")
	}
	switch kind {
	case fuel.Swap, fuel.Cap, fuel.Collar:
	default:
		return "", fmt.Errorf("a hedge is a swap, a cap or a collar")
	}
	day := int(e.day.Load())
	h := e.fuel.Quote(kind, day, 1, days, e.burn[carrier]*float64(percent)/100, 1.1)
	e.mu.Lock()
	covered := 0.0
	for _, old := range e.hedges[carrier] {
		if old.To > day {
			covered += old.Gallons
		}
	}
	if covered+h.Gallons > e.burn[carrier]*1.0001 {
		e.mu.Unlock()
		return "", fmt.Errorf("that would hedge more fuel than the carrier burns: %.0f%% is covered already", covered/e.burn[carrier]*100)
	}
	e.hedges[carrier] = append(e.hedges[carrier], h)
	e.mu.Unlock()
	s.saveState()
	switch kind {
	case fuel.Swap:
		return fmt.Sprintf("swap: %d%% of the fuel fixed at $%.2f a gallon for %d days (spot $%.2f)", percent, h.Strike, days, e.fuel.Reference(day)), nil
	case fuel.Cap:
		return fmt.Sprintf("cap: %d%% of the fuel capped at $%.2f a gallon for %d days, premium $%.0f", percent, h.Strike, days, h.Premium), nil
	}
	return fmt.Sprintf("collar: %d%% of the fuel between $%.2f and $%.2f a gallon for %d days, no premium", percent, h.Floor, h.Strike, days), nil
}

// premiumsToday is the premiums a carrier paid for options bought today.
func (e *economy) premiumsToday(carrier string) int64 {
	day := int(e.day.Load())
	e.mu.Lock()
	defer e.mu.Unlock()
	var v float64
	for _, h := range e.hedges[carrier] {
		if h.Bought == day {
			v += h.Premium
		}
	}
	return cents(v)
}

// economyView is a carrier's economy for the page and the tools.
type economyView struct {
	Day  int    `json:"day"`
	Date string `json:"date"`
	Fuel struct {
		// Reference is the Gulf Coast spot price today, Index the
		// two-month average carriers pay on, both in dollars a gallon.
		Reference float64 `json:"reference"`
		Index     float64 `json:"index"`
		Recorded  bool    `json:"recorded"`
		// AtHub is what the carrier pays at its hub.
		AtHub float64 `json:"at_hub"`
		// Forward is what the market expects in 3, 6 and 12 months.
		Forward [3]float64 `json:"forward"`
		// History is the reference on each of the last sixty days.
		History []float64 `json:"history"`
		// GallonsPerDay is the carrier's burn as scheduled; Hedged the
		// share of it under a hedge today.
		GallonsPerDay float64      `json:"gallons_per_day"`
		Hedged        float64      `json:"hedged"`
		Hedges        []hedgeView  `json:"hedges"`
		Shocks        []fuel.Shock `json:"shocks,omitempty"`
	} `json:"fuel"`
	Fleet struct {
		Aircraft   int                  `json:"aircraft"`
		Leased     int                  `json:"leased"`
		RentMonth  float64              `json:"rent_month"`
		Value      float64              `json:"value"`
		AverageAge float64              `json:"average_age"`
		ByType     []fleetType          `json:"by_type"`
		Tails      []equipment.Aircraft `json:"tails"`
		Offers     []equipment.Offer    `json:"offers"`
	} `json:"fleet"`
}

type hedgeView struct {
	fuel.Hedge
	// Value is the hedge's mark to market today, in dollars.
	Value float64 `json:"value"`
}

type fleetType struct {
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	Aircraft int     `json:"aircraft"`
	Leased   int     `json:"leased"`
	Age      float64 `json:"average_age"`
	Rent     float64 `json:"rent_month"`
}

// Economy implements airline.Economics: a carrier's fuel and fleet.
func (w seatWorld) Economy(code string) (any, bool) {
	s := w.s
	code = strings.ToUpper(code)
	e := s.econ
	if e == nil {
		return nil, false
	}
	if _, ok := e.fleets[code]; !ok {
		return nil, false
	}
	day, date := s.Day()
	var v economyView
	v.Day, v.Date = day, date.Format("2006-01-02")
	v.Fuel.Reference, v.Fuel.Index, v.Fuel.Recorded = round2(e.fuel.Reference(day)), round2(e.fuelIndex(day)), e.fuel.Recorded(day)
	v.Fuel.AtHub = round2(e.fuelPrice(s.carriers[code].Hub))
	for i, ahead := range []int{91, 182, 365} {
		v.Fuel.Forward[i] = round2(e.fuel.Forward(day, ahead))
	}
	for d := day - 59; d <= day; d++ {
		v.Fuel.History = append(v.Fuel.History, round2(e.fuel.Reference(d)))
	}
	v.Fuel.GallonsPerDay = math.Round(e.burn[code])
	v.Fuel.Shocks = e.fuel.Shocks()
	e.mu.Lock()
	for _, h := range e.hedges[code] {
		if h.To < day {
			continue
		}
		if h.From <= day {
			v.Fuel.Hedged += h.Gallons
		}
		v.Fuel.Hedges = append(v.Fuel.Hedges, hedgeView{Hedge: h})
	}
	byType := map[string]*fleetType{}
	for _, ac := range e.fleets[code] {
		v.Fleet.Aircraft++
		v.Fleet.Value += ac.Value
		v.Fleet.AverageAge += ac.Age
		ft := byType[ac.Type]
		if ft == nil {
			ft = &fleetType{Type: ac.Type, Name: ac.Name}
			byType[ac.Type] = ft
		}
		ft.Aircraft++
		ft.Age += ac.Age
		if ac.Leased {
			v.Fleet.Leased++
			v.Fleet.RentMonth += ac.Rent
			ft.Leased++
			ft.Rent += ac.Rent
		}
		v.Fleet.Tails = append(v.Fleet.Tails, ac)
	}
	e.mu.Unlock()
	for i := range v.Fuel.Hedges {
		v.Fuel.Hedges[i].Value = math.Round(v.Fuel.Hedges[i].Hedge.Value(e.fuel, day))
	}
	if e.burn[code] > 0 {
		v.Fuel.Hedged = math.Round(v.Fuel.Hedged/e.burn[code]*100) / 100
	}
	if v.Fleet.Aircraft > 0 {
		v.Fleet.AverageAge = math.Round(v.Fleet.AverageAge/float64(v.Fleet.Aircraft)*10) / 10
	}
	for _, ft := range byType {
		ft.Age = math.Round(ft.Age/float64(ft.Aircraft)*10) / 10
		v.Fleet.ByType = append(v.Fleet.ByType, *ft)
	}
	sort.Slice(v.Fleet.ByType, func(i, j int) bool { return v.Fleet.ByType[i].Aircraft > v.Fleet.ByType[j].Aircraft })
	sort.Slice(v.Fleet.Tails, func(i, j int) bool { return v.Fleet.Tails[i].Tail < v.Fleet.Tails[j].Tail })
	if len(v.Fleet.Tails) > 400 {
		v.Fleet.Tails = v.Fleet.Tails[:400]
	}
	v.Fleet.Offers = equipment.Market(1)
	return v, true
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Turn implements airline.Economics: the turnaround before a departure,
// task by task -- what the aircraft that flies it does on the ground
// between landing and leaving, and how long each job takes with the
// passengers it brought and the passengers it takes.
func (w seatWorld) Turn(code, flight, board string) (any, bool) {
	s := w.s
	code = strings.ToUpper(code)
	f, ok := w.findFlight(code, flight, board)
	if !ok {
		return nil, false
	}
	type view struct {
		Flight   string `json:"flight"`
		Station  string `json:"station"`
		Aircraft string `json:"aircraft"`
		Tail     string `json:"tail"`
		// Inbound is the leg the aircraft arrives on, empty for the first
		// of its day.
		Inbound    string `json:"inbound,omitempty"`
		Arrives    string `json:"arrives,omitempty"`
		Departs    string `json:"departs"`
		GroundTime int    `json:"ground_time,omitempty"`
		equipment.Turn
		// Slack is the scheduled ground time less the minimum: what a late
		// arrival can use before the departure is late.
		Slack int `json:"slack"`
	}
	v := view{Flight: f.Carrier + f.Number, Station: f.From, Aircraft: equipment.Type(f.Equipment, f.Seats).Name, Tail: f.Tail, Departs: hhmm(f.DepMin)}
	off := f.Seats * 8 / 10
	var prev *world.Flight
	if f.Tail != "" {
		for i, g := range s.Flights[code] {
			if g.Tail == f.Tail && g.To == f.From && g.ArrMin <= f.DepMin && (prev == nil || g.ArrMin > prev.ArrMin) {
				prev = &s.Flights[code][i]
			}
		}
	}
	on, _ := w.booked(f)
	if on == 0 {
		on = f.Seats * 8 / 10
	}
	km := f.KM
	if prev != nil {
		v.Inbound, v.Arrives = prev.Carrier+prev.Number+" from "+prev.From, hhmm(prev.ArrMin)
		v.GroundTime = f.DepMin - prev.ArrMin
		if n, _ := w.booked(*prev); n > 0 {
			off = n
		}
		km = max(km, prev.KM)
	}
	v.Turn = equipment.TurnFor(equipment.Ground{Type: f.Equipment, Seats: f.Seats, Off: off, On: on, NextKM: f.KM, Model: equipment.ModelOf(code), LongHaul: km >= 4500})
	if prev != nil {
		v.Slack = v.GroundTime - int(math.Ceil(v.Turn.Minimum))
	}
	return v, true
}

var _ airline.Economics = seatWorld{}

// minTurn is the least ground time the aircraft of a leg needs before
// it, for the day plan: the type's turnaround with a full cabin, the way
// its carrier works. It is asked a hundred thousand times when a whole
// sky is planned, so the answers are kept.
func minTurn(f world.Flight) int {
	model := equipment.ModelOf(f.Carrier)
	key := fmt.Sprintf("%s/%d/%s/%v", f.Equipment, f.Seats, model, f.KM >= 4500)
	if v, ok := minTurns.Load(key); ok {
		return v.(int)
	}
	km := 1200
	if f.KM >= 4500 {
		km = 7000
	}
	v := equipment.MinimumTurn(f.Equipment, f.Seats, model, km)
	minTurns.Store(key, v)
	return v
}

var minTurns sync.Map

// serveEconomy is GET /economy.json: the world's calendar and its fuel.
func (s *Sim) serveEconomy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.econ == nil {
		json.NewEncoder(w).Encode(map[string]any{}) //nolint:errcheck
		return
	}
	day, date := s.Day()
	e := s.econ
	regions := map[string]float64{}
	for _, reg := range []string{refdata.NorthAmerica, refdata.Europe, refdata.Asia, refdata.MiddleEast, refdata.LatinAmerica, refdata.Africa, refdata.Oceania} {
		regions[reg] = round2(e.fuelIndex(day) + e.fuel.Basis(day, reg))
	}
	var history []float64
	for d := day - 179; d <= day; d++ {
		history = append(history, round2(e.fuel.Reference(d)))
	}
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"day": day, "date": date.Format("2006-01-02"),
		"fuel": map[string]any{
			"reference": round2(e.fuel.Reference(day)), "index": round2(e.fuelIndex(day)), "recorded": e.fuel.Recorded(day),
			"long_run": fuel.LongRun, "by_region": regions, "history": history, "shocks": e.fuel.Shocks(),
			"forward": []float64{round2(e.fuel.Forward(day, 91)), round2(e.fuel.Forward(day, 182)), round2(e.fuel.Forward(day, 365))},
			"source":  "US Energy Information Administration, Gulf Coast kerosene-type jet fuel spot price",
		},
	})
}
