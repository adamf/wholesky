package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/world"
)

// The seats' view of the world: what someone running a carrier sees and
// can pull. Everything here is wired to the same tenant, plan and ledger
// the autopilot uses, so a seat's action is the world's action -- an ASM on
// the wire, a fate changed in the plan -- and not a display.

// The cost shape. An airline's cost base per block minute grows with the
// aircraft; a delay past fifteen minutes costs compensation and crew; a
// cancellation costs every booked passenger's reprotection; a reserve
// crew is a callout; a mishandled bag is a claim. These are the world's
// numbers, labelled as such; the comparison between seats is the point.
const (
	costPerBlockMinBase  = 2500  // cents per block minute, any aircraft
	costPerBlockMinSeat  = 45    // cents per block minute per seat
	costPerDelayMin      = 7500  // cents per departure minute past fifteen
	costPerCancelledPax  = 30000 // cents per booked passenger on a cancelled flight
	costPerReserveCall   = 800000
	costPerMishandledBag = 15000
	slotImprovementMin   = 10
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
	scores := w.s.scores()
	var out []airline.CarrierInfo
	for code := range w.s.Tenants {
		c := w.s.carriers[code]
		sc, ok := scores[code]
		if !ok {
			sc = w.score(code)
		}
		out = append(out, airline.CarrierInfo{Code: code, Name: c.Name, Hub: c.Hub, Flights: len(w.s.Flights[code]), Score: sc})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
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
	s.scoreCache, s.scoreAt = out, time.Now()
	return out
}

// flightStatus is where a flight is in its day.
func (w seatWorld) flightStatus(f world.Flight, fate dayplan.Flight, pos float64) string {
	t := w.s.Tenants[f.Carrier]
	if t != nil && t.Cancelled(f, w.s.BookingDate) {
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
	if t == nil || t.Inventory == nil {
		return 0, f.Seats
	}
	wire := strings.ToUpper(w.s.BookingDate.Format("02Jan"))
	for comp, n := range host.Cabins(f, w.s.capacity) {
		seats += n
		for _, sold := range t.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, comp) {
			booked += sold
		}
	}
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
			Revenue: w.s.Ledger.Sum([]string{revenue.Key(f.Carrier, f.Number, f.From)})}
		st.Delay, st.Crew = fateLines(fate)
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
	code := strings.ToUpper(carrier)
	pos, _ := w.Clock()
	sc := airline.Scorecard{Carrier: code, Costs: map[string]int64{}}
	t := w.s.Tenants[code]
	for _, f := range w.s.Flights[code] {
		fate := w.s.fate.Of(f)
		sc.Flights++
		booked, seats := w.booked(f)
		sc.Revenue += w.s.Ledger.Sum([]string{revenue.Key(f.Carrier, f.Number, f.From)})
		status := w.flightStatus(f, fate, pos)
		if fate.ATFM > 0 {
			sc.Slots++
		}
		if fate.Reserve && (status == "departed" || status == "landed") {
			sc.Reserves++
			sc.Costs["reserves"] += costPerReserveCall
		}
		switch status {
		case "cancelled":
			sc.Cancelled++
			sc.Costs["cancellations"] += int64(booked) * costPerCancelledPax
		case "departed", "landed":
			sc.Flown++
			sc.Passengers += booked
			sc.Seats += seats
			block := f.BlockMin
			if block <= 0 {
				block = f.ArrMin - f.DepMin
			}
			sc.Costs["block hours"] += int64(block) * (costPerBlockMinBase + costPerBlockMinSeat*int64(seats))
			if fate.DepDelay <= 15 {
				sc.OnTime++
			} else {
				sc.DelayMin += fate.DepDelay
				sc.Costs["delays"] += int64(fate.DepDelay-15) * costPerDelayMin
			}
		default:
			sc.Remaining++
		}
	}
	if t != nil {
		tr := t.Tracing()
		sc.Bags.Mishandled = tr.AHL
		sc.Costs["bags"] += int64(tr.AHL) * costPerMishandledBag
	}
	for _, v := range sc.Costs {
		sc.Cost += v
	}
	if seat, ok := w.s.Airline.Seat(code); ok {
		sc.Decisions, sc.Defaulted = seat.Answered, seat.Defaulted
	}
	sc.Rank()
	return sc
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
	if a.Kind == "fares" {
		s.tariff.SetMultiplier(code, a.Multiplier)
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
		t.Inventory.SetOverride(f.Carrier, f.Number, strings.ToUpper(day.Format("02Jan")), f.From, strings.ToUpper(a.Class), strings.ToUpper(a.Status))
		if a.Status == "" {
			return a.Class + " back on the ladder", nil
		}
		return a.Class + " forced " + a.Status + " on " + f.Carrier + f.Number, nil
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
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.Cancelled, pf.Reason, pf.Code = false, "", ""
			pf.Reserve = true
			pf.DepDelay += dayplan.ReserveCall
			pf.ArrDelay += dayplan.ReserveCall
		})
		return fmt.Sprintf("reserves called: departs %s", hhmm(f.DepMin+fate.DepDelay+dayplan.ReserveCall)), nil
	}
	return "", fmt.Errorf("no such action %q", a.Kind)
}

// readyForImprovement is the carrier's REA: the Network Manager improves
// the slot when the regulation has room, which it does half the time.
func (s *Sim) readyForImprovement(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time) (string, error) {
	fate := s.fate.Of(f)
	if s.ANSP == nil {
		return "", fmt.Errorf("this world has no Network Manager")
	}
	if err := s.ANSP.Ready(ctx, s.carriers[f.Carrier], f, day, fate); err != nil {
		return "", err
	}
	if aogHash(f.Carrier+f.Number+f.From+"rea")%2 != 0 || fate.ATFM <= slotImprovementMin {
		return "REA sent; no improvement available in " + fate.Regulation, nil
	}
	s.fate.Update(f, func(pf *dayplan.Flight) {
		pf.ATFM -= slotImprovementMin
		pf.CTOT -= slotImprovementMin
		pf.DepDelay = max(pf.Own, pf.ATFM, pf.DepDelay-slotImprovementMin)
		pf.ArrDelay = max(0, pf.ArrDelay-slotImprovementMin)
	})
	fate = s.fate.Of(f)
	if err := s.ANSP.Revise(ctx, s.carriers[f.Carrier], f, day, fate); err != nil {
		return "", err
	}
	return fmt.Sprintf("REA sent; SRM: new CTOT %s", hhmm(fate.CTOT)), nil
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
		Detail:  fmt.Sprintf("STD %s, now expected %s. Announce it to distribution as an ASM TIM and the systems that sold it move their bookings to the new times and queue the advice; hold it and they find out at the airport.", hhmm(f.DepMin), hhmm(f.DepMin+dep)),
		Options: []airline.Option{{Key: "announce", Label: "announce the delay (ASM TIM)"}, {Key: "hold", Label: "hold the announcement"}}, Default: "announce"})
	return choice == "announce"
}

func (s *Sim) askSubstitute(ctx context.Context, f world.Flight) bool {
	if s.Airline == nil {
		return true
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "ops", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s has gone technical", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To),
		Detail:  "The aircraft is unserviceable after check-in opened. A smaller type is available: the cabin is re-seated, some passengers may be denied boarding, distribution hears the EQT. Or cancel and reprotect everyone.",
		Options: []airline.Option{{Key: "substitute", Label: "substitute a smaller aircraft"}, {Key: "cancel", Label: "cancel the flight", Cost: costPerCancelledPax * int64(f.Seats) / 2}}, Default: "substitute"})
	return choice == "substitute"
}

func (s *Sim) askCrew(ctx context.Context, f world.Flight, fate dayplan.Flight) {
	if s.Airline == nil {
		return
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "crew", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s: the crew has timed out", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To),
		Detail:  fate.Reason + ". Cancel (the default away from the base), or call a reserve crew: the flight leaves ninety minutes later than it would have, and the callout is paid for.",
		Options: []airline.Option{{Key: "cancel", Label: "cancel the flight"}, {Key: "reserves", Label: "call reserves (+90 min)", Cost: costPerReserveCall}}, Default: "cancel"})
	if choice == "reserves" {
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.Cancelled, pf.Reason, pf.Code = false, "", ""
			pf.Reserve = true
			pf.DepDelay += dayplan.ReserveCall
			pf.ArrDelay += dayplan.ReserveCall
		})
	}
}

func (s *Sim) askSlot(ctx context.Context, t *host.Tenant, f world.Flight, fate dayplan.Flight) {
	if s.Airline == nil {
		return
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: f.Carrier, Department: "slots", Flight: f.Carrier + f.Number, Board: f.From,
		Title:   fmt.Sprintf("%s%s %s-%s has a slot: CTOT %s (+%d)", f.Carrier, strings.TrimLeft(f.Number, "0"), f.From, f.To, hhmm(fate.CTOT), fate.ATFM),
		Detail:  fmt.Sprintf("Regulation %s, cause %s. Take it, or send REA -- ready -- and ask the Network Manager for an improvement; there is one about half the time.", fate.Regulation, fate.Cause),
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
		Detail:  "Rush them on the next flight over the sector (a BUM ahead of each; the arrival station traces them and delivers), or hold them for tomorrow and let the passengers file.",
		Options: []airline.Option{{Key: "rush", Label: "rush on the next flight"}, {Key: "hold", Label: "hold for tomorrow", Cost: costPerMishandledBag * int64(bags)}}, Default: "rush"})
	return choice == "rush"
}

// federatedCarriers is the core's lobby: every peer's carriers, merged.
type federatedCarriers struct {
	seatWorld
	peers func() []string
	mu    sync.Mutex
	cache []airline.CarrierInfo
	at    time.Time
}

func (f *federatedCarriers) Carriers() []airline.CarrierInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	if time.Since(f.at) < 5*time.Second {
		return f.cache
	}
	out := f.seatWorld.Carriers()
	client := &http.Client{Timeout: 8 * time.Second}
	for _, url := range f.peers() {
		resp, err := client.Get(url + "/carriers.json")
		if err != nil {
			continue
		}
		var body struct {
			Carriers []airline.CarrierInfo `json:"carriers"`
		}
		json.NewDecoder(resp.Body).Decode(&body) //nolint:errcheck
		resp.Body.Close()
		out = append(out, body.Carriers...)
	}
	f.cache, f.at = out, time.Now()
	return out
}
