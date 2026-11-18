// Package growth is how a carrier gets bigger: what a new service would
// carry, earn and cost, the money the carrier has to start it with, and
// the orders it has placed.
//
// A service is a number of daily round trips between two airports with
// one type of aircraft. What it carries comes from four things the
// public record gives a figure for.
//
// A carrier's share of a market's travellers follows its share of the
// flights, a little more than in proportion: the S-curve. Wei and Hansen
// (Transportation Research E, 2005) found it in thirteen US duopoly
// markets from 1989 to 1999. Button and Drexler (2005) found it close to
// a straight line by the early 2000s. The exponent here is 1.2.
//
// A low-cost carrier's entry makes a market larger. Demand grew 28 per
// cent on average in the 109 markets low-cost carriers entered from
// 2012 to 2015 (Beckenstein and Campbell, 2017). The carriers already
// there cut their fares: 21 per cent by the quarter Southwest began to
// fly a route (Goolsbee and Syverson, Quarterly Journal of Economics,
// 2008), and 13 to 18 per cent where another low-cost carrier flew it
// (Brueckner, Lee and Singer, 2013).
//
// A new route takes time to fill. Spirit's Detroit to Philadelphia
// flight went from a 64 per cent load factor to 88.5 in six months with
// nobody fighting it (Spirit Airlines v. Northwest Airlines, 6th Cir.
// 2005).
//
// An aircraft flies so many hours a day. In 2019 the US network
// carriers averaged 10.8 block hours an aircraft, Southwest and JetBlue
// 11.2, Spirit and Frontier 11.9, and the wide-body fleets 13.0 (Form 41
// through the MIT Airline Data Project).
//
// What it costs comes from the same figures the scorecard uses, and is
// the caller's to supply.
package growth

import (
	"fmt"
	"math"
	"sort"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/world"
)

// The figures above.
const (
	// Alpha is the S-curve's exponent on a carrier's flights.
	Alpha = 1.2
	// Switch is how a carrier's share answers to its fares against the
	// others': the elasticity the advance sales use.
	Switch = 0.8
	// Stimulation is how much a low-cost entrant enlarges a market with
	// no low-cost carrier in it; a network carrier's entry is given 10
	// per cent, an estimate.
	Stimulation, NetworkStimulation = 0.28, 0.10
	// Response is what network carriers take off their fares when a
	// low-cost carrier with a tenth of the flights starts, and
	// UltraResponse when an ultra-low-cost carrier does.
	Response, UltraResponse = 0.21, 0.15
	// A new service starts at this share of its mature load and reaches
	// the whole of it in Spool days: 64 over 88.5, in six months.
	StartShare, Spool = 0.72, 180
	// Ceiling is the load factor a service averages at most: some seats
	// go empty on the flights nobody wants.
	Ceiling = 0.90
	// Nonstop is the share of a market's travellers who take a nonstop
	// when one is offered where there was none. An estimate.
	Nonstop = 0.70
)

// Utilisation is the block hours a day an aircraft of a kind flies for
// a carrier of a kind.
func Utilisation(model equipment.Model, widebody bool) float64 {
	switch {
	case widebody:
		return 13.0
	case model == equipment.UltraLowCost:
		return 11.9
	case model == equipment.LowCost:
		return 11.2
	}
	return 10.8
}

// Rival is one carrier already in a market.
type Rival struct {
	Carrier string
	Model   equipment.Model
	// Daily is its round trips a day, and Level where its fares sit
	// against the market's average.
	Daily float64
	Level float64
}

// Proposal is a service a carrier thinks of starting.
type Proposal struct {
	Carrier  string
	Model    equipment.Model
	Level    float64
	From, To string
	KM       int
	// BlockMin is one flight's block time and TurnMin the time on the
	// ground between two.
	BlockMin, TurnMin int
	Aircraft          refdata.Aircraft
	// Daily is the round trips a day.
	Daily int
}

// Market is what a service would be sold into.
type Market struct {
	// Passengers is the travellers a day each way in the market now, on
	// every carrier, and Measured says the figure is the Consumer Airfare
	// Report's and not worked out from the seats flown.
	Passengers float64
	Measured   bool
	// Fare is what a traveller pays there one way on average, in cents.
	Fare   int64
	Rivals []Rival
}

// Costs is what the service costs, in cents.
type Costs struct {
	// Flight is one flight's fuel, maintenance, airport and navigation
	// charges, handling and crew.
	Fuel, Maintenance, Airports, Navigation, Handling, Crew int64
	// Passenger is what selling one ticket costs.
	Passenger int64
	// Rent is one aircraft's rent a month and Deposit what the lessor
	// holds, in dollars.
	Rent, Deposit float64
	// Slot is what a pair of slots for one daily round trip costs to buy
	// at each end, in dollars; zero where slots are free.
	SlotFrom, SlotTo float64
}

// Projection is what a service is expected to do once it has filled.
type Projection struct {
	Flights  int `json:"flights_a_day"`
	Seats    int `json:"seats_a_day"`
	Aircraft int `json:"aircraft"`
	// BlockHours is the flying a day.
	BlockHours float64 `json:"block_hours_a_day"`
	// MarketNow and MarketAfter are the travellers a day, both ways,
	// before and after the service starts.
	MarketNow   float64 `json:"market_now"`
	MarketAfter float64 `json:"market_after"`
	Measured    bool    `json:"market_measured"`
	Share       float64 `json:"share"`
	// Passengers is what the service carries a day when it has filled.
	Passengers float64 `json:"passengers_a_day"`
	LoadFactor float64 `json:"load_factor"`
	// Fare is the average fare the carrier expects, in cents.
	Fare int64 `json:"fare"`
	// Revenue, Cost and Contribution are a day's, in cents.
	Revenue      int64            `json:"revenue"`
	Cost         int64            `json:"cost"`
	Contribution int64            `json:"contribution"`
	Lines        map[string]int64 `json:"cost_lines"`
	// BreakEven is the load factor at which the service pays for itself.
	BreakEven float64 `json:"break_even_load_factor"`
	// Start is the cash to begin: the lessors' deposits and the slots.
	Start int64 `json:"cash_to_start"`
	// FirstHalf is what the first 180 days bring in, loads still building.
	FirstHalf int64 `json:"first_180_days"`
	// Payback is the days until the service has earned its start back,
	// or -1 when it does not in three years.
	Payback int      `json:"payback_days"`
	Notes   []string `json:"notes,omitempty"`
}

// Share is a carrier's share of a market's travellers: each carrier
// weighs its flights to the power Alpha and its fare level to the power
// of minus Switch.
func Share(own Rival, rivals []Rival) float64 {
	w := func(r Rival) float64 {
		if r.Daily <= 0 {
			return 0
		}
		level := r.Level
		if level <= 0 {
			level = 1
		}
		return math.Pow(r.Daily, Alpha) * math.Pow(level, -Switch)
	}
	sum := w(own)
	if sum == 0 {
		return 0
	}
	for _, r := range rivals {
		sum += w(r)
	}
	return w(own) / sum
}

// Ramp is the share of its mature load a service carries on a day
// after it starts.
func Ramp(day int) float64 {
	if day >= Spool {
		return 1
	}
	if day < 0 {
		day = 0
	}
	return StartShare + (1-StartShare)*float64(day)/Spool
}

// Project works a proposal out.
func Project(p Proposal, m Market, c Costs) Projection {
	out := Projection{Flights: 2 * p.Daily, Lines: map[string]int64{}, Measured: m.Measured}
	if p.Daily <= 0 || p.Aircraft.Seats <= 0 {
		return out
	}
	out.Seats = out.Flights * p.Aircraft.Seats
	out.BlockHours = float64(out.Flights*p.BlockMin) / 60
	wide := p.Aircraft.Widebody()
	out.Aircraft = int(math.Ceil(out.BlockHours / Utilisation(p.Model, wide)))
	// An aircraft must also have the hours in the day for its turns.
	if byClock := int(math.Ceil(float64(out.Flights*(p.BlockMin+p.TurnMin)) / (18 * 60))); byClock > out.Aircraft {
		out.Aircraft = byClock
	}
	own := Rival{Carrier: p.Carrier, Model: p.Model, Daily: float64(p.Daily), Level: p.Level}

	out.MarketNow = 2 * m.Passengers
	rivals := append([]Rival(nil), m.Rivals...)
	lowCost := p.Model != equipment.Network
	already := false
	var theirs float64
	for _, r := range rivals {
		theirs += r.Daily
		if r.Model != equipment.Network {
			already = true
		}
	}
	switch {
	case len(rivals) == 0:
		// Nobody flies it nonstop: the travellers go by way of somewhere
		// else today, and most of them take a nonstop when there is one.
		out.MarketAfter = out.MarketNow * Nonstop
		if lowCost {
			out.MarketAfter *= 1 + Stimulation
		}
		out.Notes = append(out.Notes, fmt.Sprintf("Nobody flies %s-%s nonstop. %.0f%% of the market's travellers are taken to prefer a nonstop: an estimate.", p.From, p.To, Nonstop*100))
	case lowCost && !already:
		out.MarketAfter = out.MarketNow * (1 + Stimulation)
		out.Notes = append(out.Notes, fmt.Sprintf("The first low-cost carrier in a market enlarges it by %.0f%%.", Stimulation*100))
	case lowCost:
		out.MarketAfter = out.MarketNow * (1 + Stimulation/2)
	default:
		out.MarketAfter = out.MarketNow * (1 + NetworkStimulation)
	}
	// The network carriers answer a low-cost carrier that arrives with a
	// tenth of the flights.
	if lowCost && theirs > 0 && float64(p.Daily)/(theirs+float64(p.Daily)) >= 0.10 {
		cut := Response
		if p.Model == equipment.UltraLowCost {
			cut = UltraResponse
		}
		answered := false
		for i := range rivals {
			if rivals[i].Model == equipment.Network {
				rivals[i].Level *= 1 - cut
				answered = true
			}
		}
		if answered {
			out.Notes = append(out.Notes, fmt.Sprintf("The network carriers there are taken to cut their fares %.0f%% when the service starts.", cut*100))
		}
	}
	out.Share = Share(own, rivals)
	want := out.MarketAfter * out.Share
	out.Passengers = math.Min(want, float64(out.Seats)*Ceiling)
	if want > float64(out.Seats)*Ceiling {
		out.Notes = append(out.Notes, fmt.Sprintf("%.0f travellers a day would take it and it has room for %.0f: more flights or a larger aircraft would carry them.", want, float64(out.Seats)*Ceiling))
	}
	out.LoadFactor = out.Passengers / float64(out.Seats)
	// The carrier's fare against the market's average, which is the
	// average of the carriers there now.
	level := 1.0
	if n := len(m.Rivals); n > 0 {
		var sum, f float64
		for _, r := range m.Rivals {
			sum += r.Level * r.Daily
			f += r.Daily
		}
		if f > 0 && sum > 0 {
			level = sum / f
		}
	}
	out.Fare = int64(float64(m.Fare) * p.Level / level)
	if !m.Measured {
		out.Notes = append(out.Notes, "The market's size is worked out from the seats flown there. No report gives it.")
	}

	perFlight := c.Fuel + c.Maintenance + c.Airports + c.Navigation + c.Handling + c.Crew
	flights := int64(out.Flights)
	out.Lines["fuel"] = c.Fuel * flights
	out.Lines["maintenance"] = c.Maintenance * flights
	out.Lines["airports"] = c.Airports * flights
	out.Lines["navigation"] = c.Navigation * flights
	out.Lines["ground handling"] = c.Handling * flights
	out.Lines["crew"] = c.Crew * flights
	rent := int64(float64(out.Aircraft) * c.Rent * 12 / 365 * 100)
	out.Lines["aircraft rent"] = rent
	fixed := perFlight*flights + rent
	daily := func(passengers float64) (revenue, cost int64) {
		return int64(passengers * float64(out.Fare)), fixed + int64(passengers*float64(c.Passenger))
	}
	out.Revenue, out.Cost = daily(out.Passengers)
	out.Lines["selling"] = out.Cost - fixed
	out.Contribution = out.Revenue - out.Cost
	if margin := float64(out.Fare - c.Passenger); margin > 0 {
		out.BreakEven = float64(fixed) / margin / float64(out.Seats)
	} else {
		out.BreakEven = math.Inf(1)
	}
	out.Start = int64((float64(out.Aircraft)*c.Deposit + float64(p.Daily)*(c.SlotFrom+c.SlotTo)) * 100)
	if c.SlotFrom+c.SlotTo > 0 {
		out.Notes = append(out.Notes, fmt.Sprintf("Slots: %.1f million dollars for %d daily round trips.", float64(p.Daily)*(c.SlotFrom+c.SlotTo)/1e6, p.Daily))
	}
	out.Payback = -1
	var sum int64
	for day := 0; day < 3*365; day++ {
		rev, cost := daily(out.Passengers * Ramp(day))
		sum += rev - cost
		if day == Spool-1 {
			out.FirstHalf = sum
		}
		if out.Payback < 0 && sum >= out.Start {
			out.Payback = day + 1
		}
	}
	return out
}

// Day is one day of a carrier's flying, closed.
type Day struct {
	Day        int   `json:"day"`
	Revenue    int64 `json:"revenue"`
	Cost       int64 `json:"cost"`
	Profit     int64 `json:"profit"`
	Flown      int   `json:"flown"`
	Passengers int   `json:"passengers"`
	Seats      int   `json:"seats"`
	// ASK and RPK are the seats and the passengers times the kilometres
	// they flew.
	ASK float64 `json:"ask"`
	RPK float64 `json:"rpk"`
}

// Order is something a carrier has committed to.
type Order struct {
	ID       string  `json:"id"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Type     string  `json:"type"`
	Age      float64 `json:"age_years"`
	Daily    int     `json:"daily"`
	Aircraft int     `json:"aircraft"`
	// Placed and Due are day numbers: the day it was ordered and the day
	// the service can start.
	Placed int `json:"placed"`
	Due    int `json:"due"`
	// Paid is the cash that went out when it was placed, in cents.
	Paid int64 `json:"paid"`
	// Flying says the schedule holds the service.
	Flying bool `json:"flying"`
}

// Book is a carrier's money across days.
type Book struct {
	// Opening is the cash the carrier began with and Cash what it has
	// now, in cents.
	Opening int64   `json:"opening"`
	Cash    int64   `json:"cash"`
	Days    []Day   `json:"days,omitempty"`
	Orders  []Order `json:"orders,omitempty"`
}

// keep is how many days a book remembers.
const keep = 400

// Close adds a finished day: what it made goes to the cash.
func (b *Book) Close(d Day) {
	d.Profit = d.Revenue - d.Cost
	if n := len(b.Days); n > 0 && b.Days[n-1].Day == d.Day {
		// The same day closed twice: a restart. The later count stands.
		b.Cash -= b.Days[n-1].Profit
		b.Days = b.Days[:n-1]
	}
	b.Cash += d.Profit
	b.Days = append(b.Days, d)
	if len(b.Days) > keep {
		b.Days = append([]Day(nil), b.Days[len(b.Days)-keep:]...)
	}
}

// Place commits to an order, or says why the carrier cannot.
func (b *Book) Place(o Order) error {
	if o.Paid > b.Cash {
		return fmt.Errorf("the order needs %d dollars to start and the carrier has %d", o.Paid/100, b.Cash/100)
	}
	b.Cash -= o.Paid
	b.Orders = append(b.Orders, o)
	sort.SliceStable(b.Orders, func(i, j int) bool { return b.Orders[i].Due < b.Orders[j].Due })
	return nil
}

// Trend is a book summed up.
type Trend struct {
	Days    int   `json:"days"`
	Revenue int64 `json:"revenue"`
	Cost    int64 `json:"cost"`
	Profit  int64 `json:"profit"`
	// Margin is profit over revenue; LoadFactor passengers' kilometres
	// over seats'.
	Margin     float64 `json:"margin"`
	LoadFactor float64 `json:"load_factor"`
	// RASK and CASK are revenue and cost for each seat flown a
	// kilometre, and Yield revenue for each passenger flown one, in
	// cents.
	RASK  float64 `json:"rask"`
	CASK  float64 `json:"cask"`
	Yield float64 `json:"yield"`
	// Growth is the seat kilometres of the last day over the first.
	Growth float64 `json:"growth"`
	Cash   int64   `json:"cash"`
	// Runway is the days the cash lasts at the average loss, or zero
	// when the carrier makes money.
	Runway int `json:"runway_days,omitempty"`
}

// Trend sums the last n days of a book; zero is all of them.
func (b *Book) Trend(n int) Trend {
	days := b.Days
	if n > 0 && len(days) > n {
		days = days[len(days)-n:]
	}
	t := Trend{Days: len(days), Cash: b.Cash}
	var ask, rpk float64
	for _, d := range days {
		t.Revenue += d.Revenue
		t.Cost += d.Cost
		ask += d.ASK
		rpk += d.RPK
	}
	t.Profit = t.Revenue - t.Cost
	if t.Revenue > 0 {
		t.Margin = float64(t.Profit) / float64(t.Revenue)
	}
	if ask > 0 {
		t.LoadFactor = rpk / ask
		t.RASK, t.CASK = float64(t.Revenue)/ask, float64(t.Cost)/ask
	}
	if rpk > 0 {
		t.Yield = float64(t.Revenue) / rpk
	}
	if len(days) > 1 && days[0].ASK > 0 {
		t.Growth = days[len(days)-1].ASK / days[0].ASK
	}
	if t.Profit < 0 && t.Days > 0 && b.Cash > 0 {
		t.Runway = int(float64(b.Cash) / (float64(-t.Profit) / float64(t.Days)))
	}
	return t
}

// Schedule is the flights an order adds to a carrier's schedule: its
// daily round trips out of From and back, flown by the order's aircraft
// in turn. existing is the carrier's schedule before, which gives the
// hour its day starts and the flight numbers already taken; taken is
// every tail in the world. Each aircraft leaves when the last is clear,
// comes back, takes its turn on the ground and goes again, and the day
// stops at ten at night.
func Schedule(carrier string, o Order, existing []world.Flight, taken map[string]bool, km, block, turn, seats int) []world.Flight {
	if o.Daily <= 0 || o.Aircraft <= 0 || block <= 0 {
		return nil
	}
	first, top := 6*60, 0
	used := map[string]bool{}
	started := false
	for _, f := range existing {
		used[f.Number] = true
		if n := number(f.Number); n > top {
			top = n
		}
		if f.From == o.From && (!started || f.DepMin < first) {
			first, started = f.DepMin, true
		}
	}
	next := func() string {
		for {
			top++
			if top > 9999 {
				top = 1
			}
			if n := fmt.Sprintf("%04d", top); !used[n] {
				used[n] = true
				return n
			}
		}
	}
	tails := make([]string, 0, o.Aircraft)
	for k := 1; len(tails) < o.Aircraft; k++ {
		if t := fmt.Sprintf("G%s%03d", carrier, k); !taken[t] {
			taken[t] = true
			tails = append(tails, t)
		}
	}
	free := make([]int, o.Aircraft) // when each aircraft is next at From
	for a := range free {
		free[a] = first + a*45
	}
	var out []world.Flight
	for i := 0; i < o.Daily; i++ {
		a := i % o.Aircraft
		dep := free[a]
		if dep > 22*60 {
			break
		}
		back := dep + block + turn
		out = append(out,
			world.Flight{Carrier: carrier, Number: next(), From: o.From, To: o.To, DepMin: dep, ArrMin: dep + block, BlockMin: block, Equipment: o.Type, Seats: seats, KM: km, Tail: tails[a]},
			world.Flight{Carrier: carrier, Number: next(), From: o.To, To: o.From, DepMin: back, ArrMin: back + block, BlockMin: block, Equipment: o.Type, Seats: seats, KM: km, Tail: tails[a]})
		free[a] = back + block + turn
	}
	return out
}

func number(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
