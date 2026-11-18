package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/pricing"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// The fares side of the world: every carrier's pricing desk, and how a
// fare filed on one machine becomes the fare quoted on all of them.
//
// A fare is filed for a market, both directions, as a multiplier on the
// carrier's filing there. Four times a day the desks look at what moved
// since the last filing and answer (internal/pricing). A carrier nobody
// prices by hand is moved by its desk. A carrier somebody does price by
// hand is asked, with the desk's own answer as what happens if nobody
// replies. One machine runs the desks: the core, or the only machine.
// Every change it or anybody makes is published to the rest of the
// world, so the distribution system that prices a booking prices it at
// the fare the carrier filed.

const (
	// evFare is a fare filed: a carrier's multiplier in a market, or with
	// no market its multiplier everywhere.
	evFare = "fare"
	// evFareAsk is a desk's proposal for a carrier priced by hand, sent to
	// the machine that runs the carrier.
	evFareAsk = "fare-ask"
)

// filings are the minutes of the day fares are filed: ten in the
// morning, one, four and eight, which were the tariff publisher's
// domestic loads for most of the years the record covers.
var filings = []int{10 * 60, 13 * 60, 16 * 60, 20 * 60}

type fareBody struct {
	To   string  `json:"to,omitempty"`
	Mult float64 `json:"mult"`
}

// initPricing builds the desks from the schedule.
func (s *Sim) initPricing(m *world.Manifest) {
	seats := map[string]map[string]int{}
	for _, f := range m.Flights {
		a, b := f.From, f.To
		if a > b {
			a, b = b, a
		}
		k := a + "/" + b
		if seats[k] == nil {
			seats[k] = map[string]int{}
		}
		seats[k][f.Carrier] += f.Seats
	}
	var markets []pricing.Market
	for k, by := range seats {
		p := strings.SplitN(k, "/", 2)
		markets = append(markets, pricing.Market{A: p[0], B: p[1], Seats: by})
	}
	sort.Slice(markets, func(i, j int) bool { return markets[i].A+markets[i].B < markets[j].A+markets[j].B })
	var carriers []pricing.Carrier
	for _, c := range m.Carriers {
		carriers = append(carriers, pricing.Carrier{Code: c.Designator, Model: equipment.ModelOf(c.Designator), Hub: c.Hub, Level: tariff.Level(c.Designator)})
	}
	s.commerce.desk = pricing.New(int64(len(m.Flights))*31+int64(len(m.Carriers)), carriers, markets)
	s.commerce.seats = seats
	if s.eventHandlers == nil {
		s.eventHandlers = map[string]func(context.Context, worldEvent){}
	}
	s.eventHandlers[evFare] = func(_ context.Context, ev worldEvent) {
		var b fareBody
		if json.Unmarshal(ev.Body, &b) == nil {
			s.setFare(ev.Carrier, ev.From, b.To, b.Mult)
		}
	}
	s.eventHandlers[evPolicy] = func(_ context.Context, ev worldEvent) {
		var pol distribution.Policy
		if json.Unmarshal(ev.Body, &pol) == nil && ev.Carrier != "" {
			s.commerce.mu.Lock()
			s.commerce.policy[ev.Carrier] = pol
			s.commerce.mu.Unlock()
		}
	}
	s.eventHandlers[evFareAsk] = func(ctx context.Context, ev worldEvent) {
		var mv pricing.Move
		if json.Unmarshal(ev.Body, &mv) == nil && s.Tenants[mv.Carrier] != nil {
			go s.askFare(ctx, mv)
		}
	}
}

// setFare puts a carrier's multiplier in a market on this machine's
// tariff, both directions; with no market it is the carrier's multiplier
// everywhere. The multiplier is the one the carrier's fares are scaled
// by in all, so the market's own is that over the carrier's.
func (s *Sim) setFare(carrier, a, b string, mult float64) {
	if a == "" || b == "" {
		s.tariff.SetMultiplier(carrier, mult)
		return
	}
	own := mult
	if all := s.tariff.Multiplier(carrier); all > 0 && mult > 0 {
		own = mult / all
	}
	s.tariff.SetMarketMultiplier(carrier, a, b, own)
	s.tariff.SetMarketMultiplier(carrier, b, a, own)
}

// fileFare sets a fare here and tells every other machine.
func (s *Sim) fileFare(carrier, a, b string, mult float64) {
	carrier, a, b = strings.ToUpper(carrier), strings.ToUpper(a), strings.ToUpper(b)
	s.setFare(carrier, a, b, mult)
	body, _ := json.Marshal(fareBody{To: b, Mult: mult})
	s.publish(worldEvent{Kind: evFare, Carrier: carrier, From: a, Body: body})
	s.saveState()
}

// handPriced reports whether somebody prices a carrier by hand: a seat on
// this machine, or one the lobby has seen on another.
func (s *Sim) handPriced(code string) bool {
	if s.Airline != nil && s.Airline.Manual(code, "pricing") {
		return true
	}
	if s.lobby == nil {
		return false
	}
	s.lobby.mu.RLock()
	defer s.lobby.mu.RUnlock()
	for _, r := range s.lobby.rows {
		if r.Code == code && r.Seat != nil && r.Seat.Manual["pricing"] {
			return true
		}
	}
	return false
}

// runsDesks says this machine is the one that files for the desks: the
// only machine, or the core of several.
func (s *Sim) runsDesks() bool {
	s.externalMu.RLock()
	defer s.externalMu.RUnlock()
	return s.coreURL == ""
}

// pricingTicks runs the filings the clock has passed since the last look.
func (s *Sim) pricingTicks(ctx context.Context, prev, cur int) {
	if s.commerce == nil || s.commerce.desk == nil || !s.runsDesks() {
		return
	}
	for i, at := range filings {
		if prev < at && cur >= at {
			day, _ := s.Day()
			s.pricingTick(ctx, day*len(filings)+i)
		}
	}
}

// pricingTick is one filing.
func (s *Sim) pricingTick(ctx context.Context, tick int) []pricing.Move {
	cond := pricing.Conditions{Tick: tick}
	day, date := s.Day()
	cond.LeadDay = date.Weekday() == time.Thursday || date.Weekday() == time.Friday
	if s.econ != nil && day >= 30 {
		if was := s.econ.fuelIndex(day - 30); was > 0 {
			cond.FuelChange = s.econ.fuelIndex(day)/was - 1
		}
	}
	s.commerce.deskMu.Lock()
	moves := s.commerce.desk.Tick(cond, s.tariff.EffectiveMultiplier, s.handPriced)
	s.commerce.deskMu.Unlock()
	for _, mv := range moves {
		if mv.Proposal {
			if s.Tenants[mv.Carrier] != nil {
				go s.askFare(ctx, mv)
			} else {
				body, _ := json.Marshal(mv)
				s.publish(worldEvent{Kind: evFareAsk, Carrier: mv.Carrier, From: mv.A, Body: body})
			}
			continue
		}
		s.fileFare(mv.Carrier, mv.A, mv.B, mv.Mult)
		s.FareMoves.Add(1)
		s.tapeFare(mv)
	}
	if len(moves) > 0 {
		inc, fights := s.commerce.desk.Open()
		s.log.Info("fares filed", "tick", tick, "moves", len(moves), "increases_open", inc, "fights", fights)
	}
	return moves
}

// tapeFare tells the carriers a filed fare concerns: the one that moved,
// the one it answered, and everyone else in the market with a seat held.
func (s *Sim) tapeFare(mv pricing.Move) {
	if s.Airline == nil {
		return
	}
	text := fmt.Sprintf("%s %s-%s fares ×%.2f: %s", mv.Carrier, mv.A, mv.B, mv.Mult, mv.Why)
	s.Airline.Emit(mv.Carrier, "pricing", text, mv)
	for code := range s.commerce.seats[mv.A+"/"+mv.B] {
		if code == mv.Carrier {
			continue
		}
		if _, held := s.Airline.Seat(code); held {
			s.Airline.Emit(code, "pricing", text, mv)
		}
	}
}

// askFare puts a desk's proposal to whoever prices the carrier. The
// desk's own answer is what happens when nobody replies, so a seat that
// goes quiet prices like every other carrier.
func (s *Sim) askFare(ctx context.Context, mv pricing.Move) {
	if s.Airline == nil {
		return
	}
	now := s.tariff.EffectiveMultiplier(mv.Carrier, mv.A, mv.B)
	var title, act string
	switch mv.Kind {
	case pricing.Match:
		title = fmt.Sprintf("%s has cut %s-%s fares", mv.Rival, mv.A, mv.B)
		act = fmt.Sprintf("match: %s-%s to ×%.2f", mv.A, mv.B, mv.Mult)
	case pricing.Follow:
		title = fmt.Sprintf("%s has raised %s-%s fares", mv.Rival, mv.A, mv.B)
		act = fmt.Sprintf("follow: %s-%s to ×%.2f", mv.A, mv.B, mv.Mult)
	case pricing.Rollback:
		title = fmt.Sprintf("Your %s-%s increase was not followed", mv.A, mv.B)
		act = fmt.Sprintf("withdraw: %s-%s back to ×%.2f", mv.A, mv.B, mv.Mult)
	case pricing.Retaliate:
		title = fmt.Sprintf("%s cut fares where you live", mv.Rival)
		act = fmt.Sprintf("answer in its market: %s-%s to ×%.2f", mv.A, mv.B, mv.Mult)
	case pricing.Restore:
		title = fmt.Sprintf("%s has taken its %s-%s cut back", mv.Rival, mv.A, mv.B)
		act = fmt.Sprintf("restore: %s-%s to ×%.2f", mv.A, mv.B, mv.Mult)
	default:
		return
	}
	share := 0.0
	if by := s.commerce.seats[mv.A+"/"+mv.B]; by != nil {
		total := 0
		for _, n := range by {
			total += n
		}
		if total > 0 {
			share = float64(by[mv.Rival]) / float64(total)
		}
	}
	detail := fmt.Sprintf("Your %s-%s fares are at ×%.2f of the filing. %s flies %.0f%% of the seats in the market. Your pricing desk would %s (%s). Travellers shop every carrier in the market, so the price-sensitive ones go where the fare is lower.",
		mv.A, mv.B, now, mv.Rival, share*100, act, mv.Why)
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: mv.Carrier, Department: "pricing", Title: title, Detail: detail,
		Options: []airline.Option{{Key: "act", Label: act}, {Key: "hold", Label: "hold your fares"}}, Default: "act"})
	if choice != "act" {
		return
	}
	s.fileFare(mv.Carrier, mv.A, mv.B, mv.Mult)
	s.Airline.Emit(mv.Carrier, "action", fmt.Sprintf("%s-%s fares now ×%.2f (%s)", mv.A, mv.B, mv.Mult, mv.Kind), nil)
}

// shareOf is what a carrier's price against its rivals' does to the
// travellers who consider one of its flights when the day's advance
// sales are made. The fill sells each carrier's flights on their own, so
// the travellers a dearer carrier loses to a cheaper one are put here.
//
// A carrier's own fares move its traffic more than a market's fares
// move the market's. The difference is the travellers who switch. The
// studies InterVISTAS gathered for IATA in 2007 put the route's
// elasticity near -1.4 and the market's near -0.8, and a carrier's own
// on a contested route is put at -1.5 to -2 and more. The fill's
// travellers already answer to the fare with about -0.9. The switching
// share used here is -0.8, an estimate inside that gap.
func (s *Sim) shareOf(f world.Flight) float64 {
	a, b := f.From, f.To
	if a > b {
		a, b = b, a
	}
	by := s.commerce.seats[a+"/"+b]
	if len(by) < 2 {
		return 1
	}
	var sum, n float64
	for code, seats := range by {
		if code == f.Carrier {
			continue
		}
		sum += s.tariff.EffectiveMultiplier(code, a, b) * float64(seats)
		n += float64(seats)
	}
	if n == 0 || sum == 0 {
		return 1
	}
	rel := s.tariff.EffectiveMultiplier(f.Carrier, a, b) / (sum / n)
	return math.Max(0.6, math.Min(1.5, math.Pow(rel, -0.8)))
}

// faresView is the fares side for the instruments: what the desks have
// open and what has been filed.
func (s *Sim) faresView() map[string]any {
	out := map[string]any{"moves": s.FareMoves.Load()}
	if s.commerce != nil && s.commerce.desk != nil {
		s.commerce.deskMu.Lock()
		inc, fights := s.commerce.desk.Open()
		all, contested := s.commerce.desk.Markets()
		s.commerce.deskMu.Unlock()
		out["increases_open"], out["fights"], out["markets"], out["contested"] = inc, fights, all, contested
	}
	carriers, markets := s.tariff.Multipliers()
	out["carriers"], out["by_market"] = carriers, markets
	return out
}

// serveFares is GET /fares.json: what the pricing desks have open and
// every fare multiplier in force.
func (s *Sim) serveFares(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.faresView()) //nolint:errcheck
}
