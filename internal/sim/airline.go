package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/dcs"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/extras"
	"github.com/adamf/wholesky/internal/fuel"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/noc"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/rights"
	"github.com/adamf/wholesky/internal/workforce"
	"github.com/adamf/wholesky/internal/world"
)

// The seats' view of the world: what someone running a carrier sees and
// can pull. Everything here is wired to the same tenant, plan and ledger
// the autopilot uses, so a seat's action is the world's action -- an ASM on
// the wire, a fate changed in the plan -- and not a display.

// The cost shape. An airline's cost base per block minute grows with the
// aircraft: fuel, maintenance, ownership, overheads. Crews are priced by
// their agreements (internal/workforce) and the ground staff's overtime
// with them; where the world keeps no rosters, a crew's share of the block
// minute stands in. A delay past fifteen minutes costs fuel, aircraft
// time and goodwill; passengers let down are owed what the law covering
// their flight says (internal/rights), and some take their money back. A
// mishandled bag is a claim. These are the world's numbers, labelled as
// such; the comparison between seats is the point.
const (
	costPerBlockMinBase  = 1000  // cents per block minute, any aircraft
	costPerBlockMinCrew  = 1500  // cents per block minute, crews, when no roster prices them
	costPerBlockMinSeat  = 45    // cents per block minute per seat
	costPerDelayMin      = 5000  // cents per departure minute past fifteen
	costPerCancelledPax  = 30000 // cents per booked passenger, the reprotection estimate a decision quotes
	costPerReserveCall   = 800000
	costPerMishandledBag = 15000
	// refundShare is how many of a cancelled flight's passengers take the
	// fare back rather than the next flight.
	refundShare = 0.25
)

type seatWorld struct{ s *Sim }

// Clock implements airline.World.
func (w seatWorld) Clock() (float64, int) {
	pos := w.s.clock.Pos(time.Now())
	warp := 0
	if w.s.Eye != nil && w.s.Eye.WarpNow != nil {
		warp = w.s.Eye.WarpNow()
	}
	return pos, warp
}

// Carriers implements airline.World: the carriers this machine runs, with
// scorecards from the last pass -- a machine runs hundreds, and a lobby
// that computed each on every request would not answer in time.
func (w seatWorld) Carriers() []airline.CarrierInfo {
	out := append(w.OwnCarriers(), w.s.joinedCarriers()...)
	airline.RankAgainstTheWorld(out)
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// OwnCarriers implements airline.OwnCarriers: this world's carriers alone,
// with no request to any joined world -- which is what a joined world asks
// for, so two lobbies never ask each other in a loop.
func (w seatWorld) OwnCarriers() []airline.CarrierInfo {
	scores := w.s.scores()
	var out []airline.CarrierInfo
	codes := map[string]bool{}
	for code := range w.s.Tenants {
		codes[code] = true
	}
	for _, code := range w.s.Externals() {
		if _, known := w.s.carriers[code]; known {
			codes[code] = true
		}
	}
	for code := range codes {
		c := w.s.carriers[code]
		sc, ok := scores[code]
		if !ok {
			sc = w.score(code)
		}
		info := airline.CarrierInfo{Code: code, Name: c.Name, Hub: c.Hub, Flights: len(w.s.Flights[code]), Score: sc}
		if w.s.External(code) {
			info.External = true
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// Has implements airline.HasCarrier: a carrier this machine runs or has
// handed to a node.
func (w seatWorld) Has(code string) bool {
	code = strings.ToUpper(code)
	if _, ok := w.s.Tenants[code]; ok {
		return true
	}
	return w.s.External(code)
}

// scores is every local carrier's scorecard, recomputed in one pass when
// the last is more than a few seconds old.
func (s *Sim) scores() map[string]airline.Scorecard {
	s.scoreMu.Lock()
	defer s.scoreMu.Unlock()
	if time.Since(s.scoreAt) < 10*time.Second && s.scoreCache != nil {
		return s.scoreCache
	}
	out := make(map[string]airline.Scorecard, len(s.Tenants))
	w := seatWorld{s}
	for code := range s.Tenants {
		out[code] = w.score(code)
	}
	for _, code := range s.Externals() {
		if _, ok := out[code]; !ok {
			out[code] = w.score(code)
		}
	}
	s.scoreCache, s.scoreAt = out, time.Now()
	return out
}

// flightStatus is where a flight is in its day.
func (w seatWorld) flightStatus(f world.Flight, fate dayplan.Flight, pos float64) string {
	t := w.s.Tenants[f.Carrier]
	if t != nil && !w.s.External(f.Carrier) && t.Cancelled(f, w.s.BookingDate) {
		return "cancelled"
	}
	if fate.Cancelled {
		// The plan's cancellation is a fact of the day only once it has
		// been announced; until then the flight is scheduled and a seat may
		// still rescue it.
		if pos >= float64(f.DepMin-cancelledBefore) {
			return "cancelled"
		}
		return "scheduled"
	}
	dep := float64(f.DepMin + fate.DepDelay)
	arr := float64(f.ArrMin + fate.ArrDelay)
	switch {
	case pos >= arr:
		return "landed"
	case pos >= dep:
		return "departed"
	case pos >= dep-30:
		return "boarding"
	case pos >= float64(f.DepMin)-180:
		return "open"
	}
	return "scheduled"
}

func (w seatWorld) booked(f world.Flight) (booked, seats int) {
	t := w.s.Tenants[f.Carrier]
	if t == nil || t.Inventory == nil || w.s.External(f.Carrier) {
		// No book of the carrier's here: what the distribution systems
		// sold on the leg, from the ledger or the core's feed.
		return w.s.legSeats(f), f.Seats
	}
	wire := strings.ToUpper(w.s.BookingDate.Format("02Jan"))
	for comp, n := range host.Cabins(f, w.s.capacity) {
		seats += n
		for _, sold := range t.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, comp) {
			booked += sold
		}
	}
	// The schedule's own seat count is the cabin when the fleet table
	// knows less; the distribution systems' word on seats sold is the
	// floor when the inventory here knows less.
	seats = max(seats, f.Seats)
	booked = max(booked, w.s.legSeats(f))
	return booked, seats
}

// Flights implements airline.World.
func (w seatWorld) Flights(carrier string) []airline.FlightState {
	code := strings.ToUpper(carrier)
	pos, _ := w.Clock()
	t := w.s.Tenants[code]
	var out []airline.FlightState
	for _, f := range w.s.Flights[code] {
		fate := w.s.fate.Of(f)
		booked, seats := w.booked(f)
		st := airline.FlightState{Flight: f.Carrier + f.Number, From: f.From, To: f.To, STD: hhmm(f.DepMin), ETD: hhmm(f.DepMin + fate.DepDelay), STA: hhmm(f.ArrMin + fate.ArrDelay),
			DelayMin: fate.DepDelay, Status: w.flightStatus(f, fate, pos), Tail: f.Tail, Type: f.Equipment, Seats: seats, Booked: booked,
			Revenue: w.s.legRevenue(f)}
		st.Delay, st.Crew = fateLines(fate)
		if w.s.Workforce != nil && fate.Duty > 0 {
			if names := w.s.Workforce.CrewOf(code, f.Tail, fate.Duty); len(names) > 0 {
				st.Crew += " · " + strings.Join(names[:min(2, len(names))], ", ")
				if len(names) > 2 {
					st.Crew += fmt.Sprintf(" and %d cabin", len(names)-2)
				}
			}
		}
		if fate.Cancelled {
			st.Cancelled = fate.Reason
		}
		if t != nil {
			st.Boarded = t.Boarded(f, w.s.BookingDate)
			if sum, ok := t.Summarise(f.Carrier+f.Number, f.From); ok {
				st.Slot, st.Retimed, st.Substituted, st.Rushed = sum.Slot, sum.Retimed, sum.Substituted, sum.Rushed
			}
		}
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].STD < out[j].STD })
	return out
}

// Score implements airline.World: the carrier's day so far, fresh.
func (w seatWorld) Score(carrier string) airline.Scorecard { return w.score(carrier) }

func (w seatWorld) score(carrier string) airline.Scorecard {
	pos, _ := w.Clock()
	return w.scoreAt(carrier, pos)
}

// dayEnd is a position past the end of any day's flying: the minute at
// which every flight of a day that was not cancelled has left and landed,
// also one that left late or lands after midnight. A day is closed into
// the book at this position, and not at the clock's, because the clock
// has already begun the next day when the close runs.
const dayEnd = 2 * 24 * 60

// scoreAt is a carrier's day as it stands at a position of the clock, in
// minutes from the day's start.
func (w seatWorld) scoreAt(carrier string, pos float64) airline.Scorecard {
	code := strings.ToUpper(carrier)
	sc := airline.Scorecard{Carrier: code, Costs: map[string]int64{}}
	t := w.s.Tenants[code]
	var roster bool
	if w.s.Workforce != nil {
		if wc, ok := w.s.Workforce.Costs(code, pos); ok {
			roster = true
			sc.Costs["crew"] = wc.Pay + wc.PerDiem + wc.Hotels
			sc.Costs["crew premiums"] = wc.Premium
			sc.Costs["reserves"] = wc.Reserves
			sc.Costs["labour"] = wc.Grievances
		}
	}
	for _, f := range w.s.Flights[code] {
		fate := w.s.fate.Of(f)
		sc.Flights++
		booked, seats := w.booked(f)
		sc.Revenue += w.s.legRevenue(f)
		status := w.flightStatus(f, fate, pos)
		if fate.ATFM > 0 {
			sc.Slots++
		}
		if fate.Reserve && (status == "departed" || status == "landed") {
			sc.Reserves++
			if !roster {
				sc.Costs["reserves"] += costPerReserveCall
			}
		}
		switch status {
		case "cancelled":
			sc.Cancelled++
			owed := w.s.owed(f, fate, true)
			sc.Costs["passenger rights"] += int64(booked) * (owed.Compensation + owed.Care)
			if booked > 0 {
				sc.Costs["refunds"] += int64(float64(w.s.legRevenue(f)) * refundShare)
			}
			if roster {
				sc.Costs["ground staff"] += w.s.Workforce.GroundCost(code, seats, 0, true, booked)
			}
		case "departed", "landed":
			sc.Flown++
			// The passengers a flight carried are those its door counted.
			// Some who booked did not come, and a cabin the carrier
			// overbooked had more bookings than seats.
			if t != nil {
				if n, ok := t.Carried(f, w.s.BookingDate); ok {
					booked = n
				}
			}
			booked = min(booked, seats)
			sc.Passengers += booked
			sc.Seats += seats
			block := f.BlockMin
			if block <= 0 {
				block = f.ArrMin - f.DepMin
			}
			if w.s.econ != nil {
				fc := w.s.econ.flightCost(f, booked)
				sc.Costs["fuel"] += fc.Fuel
				sc.Costs["maintenance"] += fc.Maintenance + fc.Reserves
				sc.Costs["airports"] += fc.Airports
				sc.Costs["navigation"] += fc.Navigation
				sc.Costs["ground handling"] += fc.Handling
				if !roster {
					sc.Costs["crew"] += int64(block) * costPerBlockMinCrew
				}
			} else {
				perMin := costPerBlockMinBase + costPerBlockMinSeat*int64(seats)
				if !roster {
					perMin += costPerBlockMinCrew
				}
				sc.Costs["block hours"] += int64(block) * perMin
			}
			if fate.DepDelay <= 15 {
				sc.OnTime++
			} else {
				sc.DelayMin += fate.DepDelay
				sc.Costs["delays"] += int64(fate.DepDelay-15) * costPerDelayMin
				if roster {
					sc.Costs["ground staff"] += w.s.Workforce.GroundCost(code, seats, fate.DepDelay, false, booked)
				}
				if fate.ArrDelay >= 120 {
					owed := w.s.owed(f, fate, false)
					sc.Costs["passenger rights"] += int64(booked) * (owed.Compensation + owed.Care)
				}
			}
		default:
			sc.Remaining++
		}
	}
	if t != nil {
		missed := t.Missed()
		for _, m := range missed {
			sc.Misconnected += m.Pax
		}
		sc.Costs["passenger rights"] += w.s.misconnectCost(missed)
		// What the passengers who flew bought beside the seat.
		sold, _ := t.Extras()
		if len(sold) > 0 {
			sc.Extras = sold
			for _, cents := range sold {
				sc.Ancillary += cents
			}
			sc.Revenue = extras.For(code).Revenue(sc.Revenue, sc.Ancillary)
		}
		denied := t.Denied()
		for _, d := range denied {
			sc.Denied++
			if d.Voluntary {
				sc.Volunteers++
			}
		}
		sc.Costs["denied boarding"] += w.s.deniedCost(denied)
		tr := t.Tracing()
		sc.Bags.Mishandled = tr.AHL
		sc.Costs["bags"] += int64(tr.AHL) * costPerMishandledBag
	}
	// What the selling cost: the distribution systems' booking fees, the
	// commissions and overrides paid to sellers, and the carrier's own
	// cost of a sale under one line; the cost of taking the money under
	// another. What the carrier kept on coupons its partners flew is
	// revenue no leg of its own carries.
	acct := w.s.accountOf(code)
	sc.Costs["distribution"] += acct.System + acct.Commission + acct.Own
	sc.Costs["card fees"] += acct.Payment
	sc.Revenue += acct.Kept
	sc.Sales = airline.Sales{Tickets: acct.Tickets, Channels: acct.Channels, Kept: acct.Kept, Paid: acct.Paid, Taxes: acct.Taxes}
	// A joint venture shares what its routes earn: what the carrier
	// received from the pool, or paid into it, is revenue too.
	if pool, _ := w.s.pooled(); pool[code] != 0 {
		sc.Revenue += pool[code]
		sc.Sales.Pooled = pool[code]
	}
	if e := w.s.econ; e != nil {
		// The fleet costs what it costs whether it flies or not: the share
		// of a day's rent and ownership that has passed, and what the fuel
		// hedges paid or cost over the same hours.
		share := math.Min(1, math.Max(0, pos/(24*60)))
		rent, own := e.fixedCosts(code, share)
		sc.Costs["aircraft rent"] += rent
		sc.Costs["aircraft ownership"] += own
		sc.Costs["fuel hedges"] -= e.hedgeResult(code, share)
		sc.Costs["fuel hedges"] += e.premiumsToday(code)
	}
	for k, v := range sc.Costs {
		if v == 0 {
			delete(sc.Costs, k)
			continue
		}
		sc.Cost += v
	}
	if seat, ok := w.s.Airline.Seat(code); ok {
		sc.Decisions, sc.Defaulted = seat.Answered, seat.Defaulted
	}
	sc.Rank()
	return sc
}

// owed is what each passenger of a disrupted flight is owed under the
// law covering it, as the scorecard counts it: a cancelled passenger
// rebooked onto a later flight the same day, about three hours late.
func (s *Sim) owed(f world.Flight, fate dayplan.Flight, cancelled bool) rights.Owed {
	from, to := s.airports[f.From], s.airports[f.To]
	reg := rights.Of(from.Country, to.Country, s.carriers[f.Carrier].Country)
	intra := rights.Intra(reg, from.Country, to.Country)
	d := rights.Disruption{Regime: reg, KM: f.KM, Intra: intra, Domestic: intra, Cause: causeOf(fate), Cancelled: cancelled,
		ArrDelay: fate.ArrDelay, DepDelay: fate.DepDelay, Overnight: f.DepMin+fate.DepDelay >= 24*60}
	if cancelled {
		d.Rerouted, d.ArrDelay, d.DepDelay = true, 180, 180
	}
	return rights.For(d)
}

// misconnectCost is what a carrier owes the passengers who missed a
// connection, in cents.
func (s *Sim) misconnectCost(missed []host.Missed) int64 {
	var cents int64
	for _, m := range missed {
		owed := s.owedMisconnect(m)
		cents += int64(m.Pax) * (owed.Compensation + owed.Care)
	}
	return cents
}

// deniedCost is what a carrier pays the passengers it denied boarding,
// in cents.
func (s *Sim) deniedCost(denied []host.Denied) int64 {
	var cents int64
	for _, d := range denied {
		cents += s.owedDenied(d)
	}
	return cents
}

// owedDenied is what a carrier pays one passenger denied boarding. A
// volunteer has the carrier's offer. A passenger turned away has what
// the law covering the flight gives, by the fare of the class booked and
// by the wait for the carrier's next flight over the same pair, later
// that day or the next morning.
func (s *Sim) owedDenied(d host.Denied) int64 {
	f := d.Flight
	var fare int64
	if s.tariff != nil {
		for _, x := range s.tariff.Fares(f.Carrier, f.From, f.To) {
			if x.Class == d.Class && !x.RoundTrip && len(x.Rule.Sale.Accounts) == 0 && x.OneWay.Amount > fare {
				fare = x.OneWay.Amount
			}
		}
	}
	if d.Voluntary {
		return rights.Offer(fare)
	}
	wait, overnight := s.nextFlight(f)
	from, to := s.airports[f.From], s.airports[f.To]
	reg := rights.Of(from.Country, to.Country, s.carriers[f.Carrier].Country)
	owed := rights.DeniedBoarding(rights.Bumped{Regime: reg, KM: f.KM, Intra: rights.Intra(reg, from.Country, to.Country),
		Outbound: reg != rights.US || from.Country == "United States", Fare: fare, Wait: wait, Overnight: overnight,
		Smaller: d.Cause == dcs.OversaleEquipment})
	return owed.Compensation + owed.Care
}

// nextFlight is the wait for the carrier's next flight over the same
// pair after f, in minutes, and whether it is the next morning's.
func (s *Sim) nextFlight(f world.Flight) (wait int, overnight bool) {
	first := -1
	for _, g := range s.Flights[f.Carrier] {
		if g.From != f.From || g.To != f.To {
			continue
		}
		if first < 0 || g.DepMin < first {
			first = g.DepMin
		}
		if g.DepMin > f.DepMin && (wait == 0 || g.DepMin-f.DepMin < wait) {
			wait = g.DepMin - f.DepMin
		}
	}
	if wait > 0 {
		return wait, false
	}
	// No later flight today: the first one tomorrow.
	if first < 0 {
		first = f.DepMin
	}
	return 24*60 - f.DepMin + first, true
}

// owedMisconnect is what the law owes a passenger who missed a
// connection because the inbound flight was late. The journey began
// where the inbound flight did and ends where the missed flight goes.
// The passenger arrives there on the carrier's next flight over the
// same pair, later that day or the next morning, and that wait is the
// delay the law counts: under Regulation 261 the delay at the final
// destination of one booking (Folkerts, C-11/11). The cause is the
// inbound flight's.
func (s *Sim) owedMisconnect(m host.Missed) rights.Owed {
	f, in := m.Flight, m.Inbound
	wait, overnight := s.nextFlight(f)
	from, to := s.airports[in.From], s.airports[f.To]
	reg := rights.Of(from.Country, to.Country, s.carriers[f.Carrier].Country)
	intra := rights.Intra(reg, from.Country, to.Country)
	km := in.KM + f.KM
	if s.tariff != nil {
		if d := s.tariff.Distance(in.From, f.To); d > 0 {
			km = d
		}
	}
	return rights.For(rights.Disruption{Regime: reg, KM: km, Intra: intra, Domestic: intra, Cause: causeOf(s.fate.Of(in)),
		ArrDelay: wait, DepDelay: wait, Overnight: overnight})
}

// causeOf is why a flight was disrupted, as the law weighs it.
func causeOf(fate dayplan.Flight) rights.Cause {
	switch {
	case fate.Cancelled && strings.HasPrefix(fate.Reason, "crew"):
		return rights.Crew
	case fate.Cancelled && (fate.Code == "B" || strings.HasPrefix(fate.Reason, "noc")):
		return rights.Weather
	case fate.Cancelled && fate.Code == "C":
		return rights.ATC
	case fate.Cancelled:
		return rights.Carrier
	case fate.ATFM > 0 && fate.ATFM >= fate.Own:
		if fate.Cause != nil && strings.HasPrefix(fate.Cause.String(), "W") {
			return rights.Weather
		}
		return rights.ATC
	case fate.Reserve:
		return rights.Crew
	}
	return rights.Carrier
}

// findFlight is the carrier's departure by number and boarding point.
func (w seatWorld) findFlight(code, flight, board string) (world.Flight, bool) {
	flight = normaliseFlightNumber(flight)
	for _, f := range w.s.Flights[code] {
		if normaliseFlightNumber(f.Carrier+f.Number) == flight && (board == "" || f.From == strings.ToUpper(board)) {
			return f, true
		}
	}
	return world.Flight{}, false
}

func normaliseFlightNumber(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) < 3 {
		return s
	}
	return s[:2] + strings.TrimLeft(s[2:], "0")
}

// Act implements airline.World: a lever pulled by the seat.
func (w seatWorld) Act(ctx context.Context, carrier string, a airline.Action) (string, error) {
	s := w.s
	code := strings.ToUpper(carrier)
	t, ok := s.Tenants[code]
	if !ok {
		return "", fmt.Errorf("this machine does not run %s", code)
	}
	day := s.BookingDate
	switch a.Kind {
	case "standby":
		if s.Workforce == nil || s.Workforce.Roster(code) == nil {
			return "", fmt.Errorf("this world keeps no rosters")
		}
		n := min(max(a.Count, 1), 10)
		base := strings.ToUpper(a.Board)
		if base == "" {
			base = s.carriers[code].Hub
		}
		got := s.Workforce.AddStandby(code, base, n)
		if got == 0 {
			return "", fmt.Errorf("%s is not one of %s's crew bases", base, code)
		}
		pos, _ := w.Clock()
		s.restaffPending(code, pos)
		return fmt.Sprintf("%d standby crews at %s for the rest of the day", got, base), nil
	case "respond":
		if s.Workforce == nil {
			return "", fmt.Errorf("this world keeps no agreements")
		}
		return s.Workforce.Respond(code, workforce.Unit(strings.ToLower(a.Unit)), strings.ToLower(a.Choice))
	case "noc":
		pol, ok := noc.PolicyByName(strings.ToLower(a.Policy))
		if !ok {
			return "", fmt.Errorf("no such policy %q: absorb, thin 15%%, thin 30%%, standby crews", a.Policy)
		}
		md, err := s.nocModel(code)
		if err != nil {
			return "", err
		}
		pos, _ := w.Clock()
		return s.applyPolicy(ctx, code, md, s.forecast(code, pos, md), pol), nil
	}
	if a.Kind == "channel" {
		return s.setChannel(code, a.Choice, a.Unit, a.Count)
	}
	if a.Kind == "grow" {
		return s.order(code, a.From, a.To, a.Policy, a.Count)
	}
	if a.Kind == "hedge" {
		return s.hedge(code, fuel.Kind(strings.ToLower(a.Policy)), a.Count, a.Minutes)
	}
	if a.Kind == "fares" {
		// A fare filed here is the fare every machine quotes, and the
		// other carriers' desks see it at the next filing.
		if a.From != "" && a.To != "" {
			s.fileFare(code, a.From, a.To, a.Multiplier*s.tariff.Multiplier(code))
			return fmt.Sprintf("%s-%s fares now ×%.2f over the filing, both ways", strings.ToUpper(a.From), strings.ToUpper(a.To), s.tariff.EffectiveMultiplier(code, a.From, a.To)), nil
		}
		s.fileFare(code, "", "", a.Multiplier)
		return fmt.Sprintf("fares now ×%.2f over the filing", s.tariff.Multiplier(code)), nil
	}
	f, ok := w.findFlight(code, a.Flight, a.Board)
	if !ok {
		return "", fmt.Errorf("no departure %s from %s today", a.Flight, a.Board)
	}
	fate := s.fate.Of(f)
	pos, _ := w.Clock()
	switch a.Kind {
	case "cancel":
		if fate.Cancelled {
			return "", fmt.Errorf("%s is already cancelled", a.Flight)
		}
		if pos >= float64(f.DepMin+fate.DepDelay) {
			return "", fmt.Errorf("%s has departed", a.Flight)
		}
		reason := a.Reason
		if reason == "" {
			reason = "cancelled by operations"
		}
		s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, reason, "A" })
		s.announceCancellation(ctx, t, f, day, reason)
		s.replan(f)
		return "cancelled: ASM CNL to distribution, the airport told, the flight plan withdrawn", nil
	case "retime":
		if a.Minutes <= 0 {
			return "", fmt.Errorf("a retime needs minutes")
		}
		if pos >= float64(f.DepMin+fate.DepDelay) {
			return "", fmt.Errorf("%s has departed", a.Flight)
		}
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.DepDelay = max(pf.DepDelay, a.Minutes)
			pf.ArrDelay = max(pf.ArrDelay, a.Minutes)
			pf.Own = max(pf.Own, a.Minutes)
		})
		if err := t.Retime(ctx, f, day, a.Minutes, a.Minutes); err != nil {
			return "", err
		}
		s.replan(f)
		return fmt.Sprintf("retimed to %s: ASM TIM to distribution, bookings moved to TK", hhmm(f.DepMin+a.Minutes)), nil
	case "substitute":
		typ, err := t.Substitute(ctx, f, day)
		if err != nil {
			return "", err
		}
		return "substituted: now a " + typ + "; distribution hears the EQT", nil
	case "class":
		if a.Class == "" {
			return "", fmt.Errorf("a class action needs a class")
		}
		// The lever's word for a closed class is C. The inventory's is the
		// reply a closed class gives, UC: it took a C for a code it did not
		// know and left the class open.
		status := strings.ToUpper(a.Status)
		if status == "C" || status == "CLOSED" {
			status = "UC"
		}
		t.Inventory.SetOverride(f.Carrier, f.Number, strings.ToUpper(day.Format("02Jan")), f.From, strings.ToUpper(a.Class), status)
		t.Touch(f.Number, f.From)
		if status == "" {
			return a.Class + " back on the ladder", nil
		}
		return a.Class + " forced " + status + " on " + f.Carrier + f.Number, nil
	case "ready":
		if fate.CTOT <= 0 {
			return "", fmt.Errorf("%s holds no slot", a.Flight)
		}
		return s.readyForImprovement(ctx, t, f, day)
	case "reserves":
		if !fate.Cancelled || !strings.HasPrefix(fate.Reason, "crew") {
			return "", fmt.Errorf("%s is not a crew cancellation", a.Flight)
		}
		if pos >= float64(f.DepMin-cancelledBefore) {
			return "", fmt.Errorf("too late: the cancellation has been announced")
		}
		if s.Workforce != nil && s.Workforce.Roster(code) != nil {
			return s.callStandbyReserve(f)
		}
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.Cancelled, pf.Reason, pf.Code = false, "", ""
			pf.Reserve = true
			pf.DepDelay += dayplan.ReserveCall
			pf.ArrDelay += dayplan.ReserveCall
		})
		s.replan(f)
		return fmt.Sprintf("reserves called: departs %s", hhmm(f.DepMin+fate.DepDelay+dayplan.ReserveCall)), nil
	}
	return "", fmt.Errorf("no such action %q", a.Kind)
}

// replan works a tail's day again after a seat changed one of its legs,
// and tells the seat what else changed: the flights that are now later
// because the aircraft is, and the ones whose crew now times out.
func (s *Sim) replan(f world.Flight) {
	if f.Tail == "" || s.fate == nil {
		return
	}
	changed := s.fate.Replan(s.Manifest, f.Carrier, f.Tail)
	for _, g := range changed {
		if g.Number == f.Number && g.From == f.From {
			continue
		}
		fate := s.fate.Of(g)
		text := fmt.Sprintf("%s%s %s-%s now leaves %s (+%d) because the aircraft does", g.Carrier, strings.TrimLeft(g.Number, "0"), g.From, g.To, hhmm(g.DepMin+fate.DepDelay), fate.DepDelay)
		if fate.Cancelled {
			text = fmt.Sprintf("%s%s %s-%s: %s -- cancelled unless reserves are called", g.Carrier, strings.TrimLeft(g.Number, "0"), g.From, g.To, fate.Reason)
		}
		if s.Airline != nil {
			s.Airline.Emit(g.Carrier, "incident", text, map[string]any{"flight": g.Carrier + g.Number, "board": g.From, "delay": fate.DepDelay, "cancelled": fate.Cancelled})
		}
	}
}

// callStandbyReserve rescues a crew cancellation with the roster: a
// standby crew comes in at the flight's station, which must be a crew
// base, and the tail is staffed again; the roster's reserve takes the
// duty and the flight waits the callout.
func (s *Sim) callStandbyReserve(f world.Flight) (string, error) {
	r := s.Workforce.Roster(f.Carrier)
	if r == nil || !r.IsBase(f.From) {
		return "", fmt.Errorf("%s is not a crew base: no reserve can reach the aircraft", f.From)
	}
	s.Workforce.AddStandby(f.Carrier, f.From, 1)
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = false, "", "" })
	s.replan(f)
	fate := s.fate.Of(f)
	if fate.Cancelled {
		return "", fmt.Errorf("the standby crew cannot fly it either: %s", fate.Reason)
	}
	return fmt.Sprintf("standby crew called: %s%s departs %s", f.Carrier, strings.TrimLeft(f.Number, "0"), hhmm(f.DepMin+fate.DepDelay)), nil
}

// standbyCost is what a standby crew costs the day, for a decision's quote.
func (s *Sim) standbyCost(f world.Flight) int64 {
	c, ok := s.Workforce.Contract(f.Carrier, workforce.Pilots)
	k, ok2 := s.Workforce.Contract(f.Carrier, workforce.FlightAttendants)
	if !ok || !ok2 {
		return costPerReserveCall
	}
	fl := workforce.FleetOf(f.Seats, false)
	day := c.Rules(fl, true, 8).ReserveDay() + c.Rules(fl, false, 8).ReserveDay() + 2*k.Rules(fl, false, 8).ReserveDay()
	return day * 3 / 2
}

// readyForImprovement is the carrier's REA: the flight is ready and would
// take an earlier slot at short notice. The Network Manager improves it
// only when a slot ahead of it is free, which happens when a flight in
// the regulation has been cancelled or has itself moved up.
func (s *Sim) readyForImprovement(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time) (string, error) {
	fate := s.fate.Of(f)
	if s.ANSP == nil {
		return "", fmt.Errorf("this world has no Network Manager")
	}
	if err := s.ANSP.Ready(ctx, s.carriers[f.Carrier], f, day, fate); err != nil {
		return "", err
	}
	pos := int(s.clock.Pos(time.Now()))
	changes := s.fate.Ready(s.Manifest, f, pos)
	s.publish(worldEvent{Kind: evReady, Carrier: f.Carrier, Number: f.Number, From: f.From, Pos: pos})
	s.slotsImproved(ctx, changes, f.Carrier+strings.TrimLeft(f.Number, "0")+" reported ready", pos)
	now := s.fate.Of(f)
	if now.ATFM >= fate.ATFM {
		return "REA sent; no slot is free ahead of this one in " + fate.Regulation, nil
	}
	if now.CTOT <= 0 {
		return "REA sent; SLC: the flight is no longer held", nil
	}
	return fmt.Sprintf("REA sent; SRM: new CTOT %s", hhmm(now.CTOT)), nil
}

// announceCancellation is everything a cancellation says on the wire: the
// ASM CNL to distribution (and the marketing carrier's for a codeshare),
// the airport's departure control told, the flight plan withdrawn. Once
// per departure however many times the day asks.
func (s *Sim) announceCancellation(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time, reason string) {
	if _, done := s.announced.LoadOrStore(dayplan.Key(f), true); done {
		return
	}
	text := fmt.Sprintf("ASM\nUTC\nCNL\n%s%s/%s\n%s %s", f.Carrier, f.Number, strings.ToUpper(day.Format("02Jan")), f.From, f.To)
	if err := t.SendSchedule(ctx, text); err != nil {
		s.log.Debug("cancellation not sent", "flight", f.Carrier+f.Number, "err", err)
	}
	if mt, ok := s.Tenants[f.Marketing]; ok && f.Marketing != "" && f.Marketing != f.Carrier {
		mtext := fmt.Sprintf("ASM\nUTC\nCNL\n%s%s/%s\n%s %s", f.Marketing, f.MarketingNumber, strings.ToUpper(day.Format("02Jan")), f.From, f.To)
		if err := mt.SendSchedule(ctx, mtext); err != nil {
			s.log.Debug("marketing cancellation not sent", "flight", f.Marketing+f.MarketingNumber, "err", err)
		}
	}
	if err := t.CancelFlight(ctx, f, day, reason); err != nil {
		s.log.Debug("dcs cancellation failed", "flight", f.Carrier+f.Number, "err", err)
	}
	if s.ANSP != nil {
		if err := s.ANSP.Cancellation(ctx, s.carriers[f.Carrier], f, day); err != nil {
			s.log.Debug("flight plan cancellation not sent", "flight", f.Carrier+f.Number, "err", err)
		}
	}
	// The flight plan is withdrawn, so the Network Manager knows: the
	// flight's slots go back to the regulation and the flights behind it
	// move up.
	s.releaseSlots(ctx, f, reason)
	if s.Airline != nil {
		s.Airline.Emit(f.Carrier, "incident", f.Carrier+strings.TrimLeft(f.Number, "0")+" "+f.From+"-"+f.To+" cancelled: "+reason, nil)
	}
}

// The seat's decisions, asked from the day's events where the autopilot
// used to act alone. Each returns what to do; on autopilot that is what
// the autopilot always did.

func (s *Sim) askRetime(ctx context.Context, f world.Flight, dep, arr int) bool {
	if s.Airline == nil {
		return true
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "ops", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s will leave %d minutes late", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To, dep),
		Detail:  fmt.Sprintf("STD %s, now expected %s. Announce: an ASM TIM goes to distribution, the systems that sold the flight move their bookings to the new times and queue the advice. Hold: passengers learn of the delay at the airport.", hhmm(f.DepMin), hhmm(f.DepMin+dep)),
		Options: []airline.Option{{Key: "announce", Label: "announce the delay (ASM TIM)"}, {Key: "hold", Label: "hold the announcement"}}, Default: "announce"})
	return choice == "announce"
}

func (s *Sim) askSubstitute(ctx context.Context, f world.Flight) bool {
	if s.Airline == nil {
		return true
	}
	s.Airline.Emit(f.Carrier, "incident", fmt.Sprintf("%s%s %s-%s: aircraft unserviceable after check-in opened", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To), nil)
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "ops", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s has gone technical", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To),
		Detail:  "The aircraft became unserviceable after check-in opened. Substitute: a smaller type takes the flight, the cabin is re-seated, some passengers may be denied boarding, and distribution receives an ASM EQT. Cancel: every passenger is reprotected.",
		Options: []airline.Option{{Key: "substitute", Label: "substitute a smaller aircraft"}, {Key: "cancel", Label: "cancel the flight", Cost: costPerCancelledPax * int64(f.Seats) / 2}}, Default: "substitute"})
	return choice == "substitute"
}

func (s *Sim) askCrew(ctx context.Context, f world.Flight, fate dayplan.Flight) {
	if s.Airline == nil {
		return
	}
	s.Airline.Emit(f.Carrier, "incident", fmt.Sprintf("%s%s %s-%s: crew timed out (%s)", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To, fate.Reason), nil)
	if s.Workforce != nil && s.Workforce.Roster(f.Carrier) != nil {
		r := s.Workforce.Roster(f.Carrier)
		opts := []airline.Option{{Key: "cancel", Label: "cancel the flight"}}
		detail := fate.Reason + ". No reserve on call at " + f.From + " can take the duty."
		if r.IsBase(f.From) {
			opts = append(opts, airline.Option{Key: "reserves", Label: "bring in a standby crew (+90 min)", Cost: s.standbyCost(f)})
			detail += " Bring in a standby crew: volunteers on a day off, paid half again; the flight leaves 90 minutes later."
		} else {
			detail += " " + f.From + " is no crew base: nobody can reach the aircraft in time."
		}
		choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "crew", Flight: f.Carrier + f.Number, Board: f.From,
			Title:   fmt.Sprintf("%s%s %s-%s: the crew cannot fly it", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To),
			Detail:  detail,
			Options: opts, Default: "cancel"})
		if choice == "reserves" {
			if res, err := s.callStandbyReserve(f); err == nil {
				s.Airline.Emit(f.Carrier, "action", res, nil)
			}
		}
		return
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "crew", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s: the crew has timed out", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To),
		Detail:  fate.Reason + ". Cancel: the default away from the base. Call reserves: the flight leaves 90 minutes later than planned, and the callout is charged to the scorecard.",
		Options: []airline.Option{{Key: "cancel", Label: "cancel the flight"}, {Key: "reserves", Label: "call reserves (+90 min)", Cost: costPerReserveCall}}, Default: "cancel"})
	if choice == "reserves" {
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.Cancelled, pf.Reason, pf.Code = false, "", ""
			pf.Reserve = true
			pf.DepDelay += dayplan.ReserveCall
			pf.ArrDelay += dayplan.ReserveCall
		})
		s.replan(f)
	}
}

func (s *Sim) askSlot(ctx context.Context, t *host.Tenant, f world.Flight, fate dayplan.Flight) {
	if s.Airline == nil {
		return
	}
	s.Airline.Emit(f.Carrier, "incident", fmt.Sprintf("%s%s %s-%s slotted: CTOT %s (+%d) under %s", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To, hhmm(fate.CTOT), fate.ATFM, fate.Regulation), nil)
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "slots", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s has a slot: CTOT %s (+%d)", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To, hhmm(fate.CTOT), fate.ATFM),
		Detail:  fmt.Sprintf("Regulation %s, cause %s. Take the slot, or send REA (ready) to take an earlier one at short notice. An earlier slot exists only when a flight ahead in the regulation has been cancelled.", fate.Regulation, fate.Cause),
		Options: []airline.Option{{Key: "accept", Label: "take the slot"}, {Key: "ready", Label: "send REA, ask for a better one"}}, Default: "accept"})
	if choice == "ready" {
		if res, err := s.readyForImprovement(ctx, t, f, s.BookingDate); err == nil {
			s.Airline.Emit(f.Carrier, "incident", f.Carrier+strings.TrimLeft(f.Number, "0")+": "+res, nil)
		}
	}
}

func (s *Sim) askRush(ctx context.Context, f world.Flight, day time.Time, bags int) bool {
	if s.Airline == nil {
		return true
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "ground", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s left %d bags behind at %s", f.Carrier, strings.TrimLeft(f.Number, "0"), bags, f.From),
		Detail:  "Rush: the bags travel on the next flight over the sector, a BUM goes ahead of each, and the arrival station traces and delivers them. Hold: the bags travel tomorrow and the passengers file claims.",
		Options: []airline.Option{{Key: "rush", Label: "rush on the next flight"}, {Key: "hold", Label: "hold for tomorrow", Cost: costPerMishandledBag * int64(bags)}}, Default: "rush"})
	return choice == "rush"
}

// legRevenue is what a leg was sold for: the distribution systems' word
// where the core feeds it (a region's own books may carry no fare), else
// this machine's ledger.
func (s *Sim) legRevenue(f world.Flight) int64 {
	key := revenue.Key(f.Carrier, f.Number, f.From)
	s.revenueMu.RLock()
	v, ok := s.revenueFeed[key]
	s.revenueMu.RUnlock()
	if ok {
		return v
	}
	return s.Ledger.Sum([]string{key})
}

// legSeats is how many passengers a leg was sold to, from the core's feed
// where it has one, else this machine's ledger.
func (s *Sim) legSeats(f world.Flight) int {
	key := revenue.Key(f.Carrier, f.Number, f.From)
	s.revenueMu.RLock()
	v, ok := s.seatsFeed[key]
	s.revenueMu.RUnlock()
	if ok {
		return v
	}
	return s.Ledger.Seats([]string{key})
}

// accountOf is a carrier's sales account, from the core's feed where it
// has one, else this machine's ledger.
func (s *Sim) accountOf(carrier string) revenue.Account {
	s.revenueMu.RLock()
	a, ok := s.accountFeed[carrier]
	s.revenueMu.RUnlock()
	if ok {
		return a
	}
	return s.Ledger.AccountOf(carrier)
}

// LegFeed is what the distribution systems sold on every leg: money and
// passengers. On a distribution system's machine it is that system's word
// for every leg of the world; the core sums the systems' and hands the
// total to every region.
type LegFeed struct {
	Revenue map[string]int64 `json:"revenue"`
	Seats   map[string]int   `json:"seats"`
	// Accounts is each carrier's sales: what its tickets earned, cost to
	// sell and collected in taxes.
	Accounts map[string]revenue.Account `json:"accounts,omitempty"`
	// Agents is each agency's sales, by its number.
	Agents map[string]revenue.Agent `json:"agents,omitempty"`
}

// RevenueByLeg is this machine's ledger as a feed.
func (s *Sim) RevenueByLeg() LegFeed {
	out := LegFeed{Revenue: map[string]int64{}, Seats: s.Ledger.SeatsByLeg(), Accounts: s.Ledger.Accounts(), Agents: s.Ledger.Agents()}
	for _, fs := range s.Flights {
		for _, f := range fs {
			key := revenue.Key(f.Carrier, f.Number, f.From)
			if v := s.Ledger.Sum([]string{key}); v != 0 {
				out.Revenue[key] = v
			}
		}
	}
	return out
}

// SetRevenueFeed installs the core's federated view of what every leg was
// sold for, which the scorecards then read instead of the local ledger.
func (s *Sim) SetRevenueFeed(f LegFeed) {
	s.revenueMu.Lock()
	s.revenueFeed, s.seatsFeed, s.accountFeed, s.agentFeed = f.Revenue, f.Seats, f.Accounts, f.Agents
	s.revenueMu.Unlock()
	s.scoreMu.Lock()
	s.scoreAt = time.Time{}
	s.scoreMu.Unlock()
}

// serveRevenue is GET /shard/revenue.json and POST /shard/revenue.
func (s *Sim) serveRevenue(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if !s.secretOK(r) {
			http.Error(w, "the revenue feed is the world's own to write", http.StatusForbidden)
			return
		}
		var f LegFeed
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&f); err != nil {
			http.Error(w, "malformed revenue feed", http.StatusBadRequest)
			return
		}
		s.SetRevenueFeed(f)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.RevenueByLeg()) //nolint:errcheck
}

// weatherIncidents tells each carrier when a weather system that touches
// its stations comes into force.
func (s *Sim) weatherIncidents(prev, cur int) {
	if s.Airline == nil || s.fate == nil {
		return
	}
	cells, _, _ := s.fate.Picture()
	for _, c := range cells {
		if !(c.Start > prev && c.Start <= cur) {
			continue
		}
		touched := map[string]int{}
		for code := range s.Tenants {
			for _, f := range s.Flights[code] {
				for _, ap := range c.Airports {
					if f.To == ap && f.ArrMin >= c.Start && f.ArrMin < c.End {
						touched[code]++
					}
				}
			}
		}
		for code, n := range touched {
			s.Airline.Emit(code, "incident", fmt.Sprintf("weather %s in force until %s: arrivals at %s at ×%.2f of the rate; %d of your arrivals fall in it", c.Name, hhmm(c.End), strings.Join(c.Airports, " "), c.Factor, n), nil)
		}
	}
}

// lobbyCache answers the lobby from the last build and rebuilds it in the
// background: the build asks every machine for hundreds of scorecards and
// every joined world for its rows, which is seconds, and a page should not
// wait for it. The first request after boot builds once, synchronously.
type lobbyCache struct {
	inner airline.World
	mu    sync.RWMutex
	rows  []airline.CarrierInfo
	at    time.Time
	busy  bool
}

func (c *lobbyCache) Clock() (float64, int)                     { return c.inner.Clock() }
func (c *lobbyCache) Flights(code string) []airline.FlightState { return c.inner.Flights(code) }
func (c *lobbyCache) Score(code string) airline.Scorecard       { return c.inner.Score(code) }
func (c *lobbyCache) Act(ctx context.Context, code string, a airline.Action) (string, error) {
	return c.inner.Act(ctx, code, a)
}

// Crew, CrewMember, NOC and Project pass the operations centre through.
func (c *lobbyCache) Crew(code string) (any, bool) {
	if o, ok := c.inner.(airline.Operations); ok {
		return o.Crew(code)
	}
	return nil, false
}

func (c *lobbyCache) CrewMember(code, id string) (any, bool) {
	if o, ok := c.inner.(airline.Operations); ok {
		return o.CrewMember(code, id)
	}
	return nil, false
}

func (c *lobbyCache) NOC(code string) (any, bool) {
	if o, ok := c.inner.(airline.Operations); ok {
		return o.NOC(code)
	}
	return nil, false
}

func (c *lobbyCache) Project(ctx context.Context, code string, req json.RawMessage) (any, error) {
	if o, ok := c.inner.(airline.Operations); ok {
		return o.Project(ctx, code, req)
	}
	return nil, fmt.Errorf("this world has no operations centres")
}

// Economy and Turn pass the economy through.
func (c *lobbyCache) Economy(code string) (any, bool) {
	if e, ok := c.inner.(airline.Economics); ok {
		return e.Economy(code)
	}
	return nil, false
}

func (c *lobbyCache) Turn(code, flight, board string) (any, bool) {
	if e, ok := c.inner.(airline.Economics); ok {
		return e.Turn(code, flight, board)
	}
	return nil, false
}

// Finance and Growth pass the carrier's book and projections through.
func (c *lobbyCache) Finance(code string) (any, bool) {
	if g, ok := c.inner.(airline.Growth); ok {
		return g.Finance(code)
	}
	return nil, false
}

func (c *lobbyCache) Growth(code, from, to, aircraft string, daily int) (any, error) {
	if g, ok := c.inner.(airline.Growth); ok {
		return g.Growth(code, from, to, aircraft, daily)
	}
	return nil, fmt.Errorf("this world keeps no economy")
}

// Has answers cheaply from the inner world.
func (c *lobbyCache) Has(code string) bool {
	if h, ok := c.inner.(airline.HasCarrier); ok {
		return h.Has(code)
	}
	for _, r := range c.Carriers() {
		if r.Code == code {
			return true
		}
	}
	return false
}

// Carriers is the lobby as last built, rebuilt if stale.
func (c *lobbyCache) Carriers() []airline.CarrierInfo {
	c.mu.RLock()
	rows, at := c.rows, c.at
	c.mu.RUnlock()
	if rows != nil && time.Since(at) < 30*time.Second {
		return rows
	}
	if rows != nil {
		go c.refresh()
		return rows
	}
	return c.refresh()
}

// OwnCarriers is this world's rows from the same build. Before the
// first build it asks the world for its own rows and nothing else: a
// build asks every joined world for its rows, a joined world asks for
// these to make its own, and two worlds that both started cold would
// each wait on the other until the request timed out.
func (c *lobbyCache) OwnCarriers() []airline.CarrierInfo {
	c.mu.RLock()
	cold := c.rows == nil
	c.mu.RUnlock()
	if own, ok := c.inner.(airline.OwnCarriers); ok && cold {
		return own.OwnCarriers()
	}
	var out []airline.CarrierInfo
	for _, r := range c.Carriers() {
		if r.World == "" {
			out = append(out, r)
		}
	}
	return out
}

// refresh builds the lobby once; a build already running is not doubled.
func (c *lobbyCache) refresh() []airline.CarrierInfo {
	c.mu.Lock()
	if c.busy {
		rows := c.rows
		c.mu.Unlock()
		return rows
	}
	c.busy = true
	c.mu.Unlock()
	rows := c.inner.Carriers()
	c.mu.Lock()
	c.rows, c.at, c.busy = rows, time.Now(), false
	c.mu.Unlock()
	return rows
}

// run keeps the lobby fresh while the world lives.
func (c *lobbyCache) run(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.refresh()
		}
	}
}

// federatedCarriers is the core's lobby: every peer's carriers, merged.
type federatedCarriers struct {
	seatWorld
	peers func() []string
	mu    sync.Mutex
	cache []airline.CarrierInfo
	at    time.Time
}

// Carriers is the core's merged lobby: every peer's carriers, then the
// joined worlds'.
func (f *federatedCarriers) Carriers() []airline.CarrierInfo {
	out := append(f.OwnCarriers(), f.s.joinedCarriers()...)
	airline.RankAgainstTheWorld(out)
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// OwnCarriers is every peer's carriers merged, without the joined worlds'.
func (f *federatedCarriers) OwnCarriers() []airline.CarrierInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Since(f.at) < 5*time.Second {
		return f.cache
	}
	// The peers' rows first; the core's own view of a carrier (an external
	// one it only knows the token of) fills in where no peer listed it.
	byCode := map[string]int{}
	var out []airline.CarrierInfo
	client := &http.Client{Timeout: 15 * time.Second}
	// Every peer at once: the lobby is as slow as the slowest, not the sum.
	urls := f.peers()
	lists := make([][]airline.CarrierInfo, len(urls))
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			// A peer's own rows only: the core adds the joined worlds' once.
			resp, err := client.Get(url + "/carriers.json?own=1")
			if err != nil {
				return
			}
			var body struct {
				Carriers []airline.CarrierInfo `json:"carriers"`
			}
			json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
			resp.Body.Close()
			lists[i] = body.Carriers
		}(i, url)
	}
	wg.Wait()
	for _, list := range lists {
		for _, c := range list {
			if _, seen := byCode[c.Code]; seen || c.World != "" {
				continue
			}
			byCode[c.Code] = len(out)
			out = append(out, c)
		}
	}
	for _, c := range f.seatWorld.OwnCarriers() {
		if _, seen := byCode[c.Code]; !seen {
			byCode[c.Code] = len(out)
			out = append(out, c)
		}
	}
	f.cache, f.at = out, time.Now()
	return out
}
