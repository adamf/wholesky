package sim

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// What a carrier holds about one departure is that departure's. The
// world flies the same date again tomorrow, so the next departure of a
// flight has the same key.
//
//   - A cancelled flight is closed for sale at once, and no traveller is
//     offered it. When the departure is forgotten, at the hour it would
//     have landed, tomorrow's flight is for sale again and is not
//     cancelled.
//   - A flight number that flies 2 legs: cancelling one leg does not
//     cancel the other.
//   - A flight retimed today can be retimed again tomorrow, and a smaller
//     aircraft put on a flight today is not on it tomorrow.
func TestADeparturesStateIsItsOwn(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	code := m.Carriers[0].Designator
	tn, day := s.Tenants[code], s.BookingDate
	var f, h world.Flight
	for _, g := range s.Flights[code] {
		if s.fate.Of(g).Cancelled || g.Marketing != "" || g.DepMin < 8*60 || g.DepMin > 18*60 {
			continue
		}
		if f.Number == "" {
			f = g
		} else if h.Number == "" && g.Number != f.Number && host.SmallerType(g.Equipment) != "" {
			h = g // a smaller aircraft exists for it
		}
	}
	if h.Number == "" {
		t.Fatal("no 2 flights")
	}
	open := func(g world.Flight) avail.Status {
		t.Helper()
		got := tn.Inventory.Availability([]avail.Key{avail.NewKey(g.Carrier, g.Number, day, g.From, g.To, "Y")}, time.Now())
		if len(got) != 1 {
			t.Fatalf("availability of %s%s: %d answers", g.Carrier, g.Number, len(got))
		}
		return got[0].Status
	}
	if open(f) != avail.Open || s.notFlying(f) {
		t.Fatalf("before anything, %s%s is %v", f.Carrier, f.Number, open(f))
	}

	// The seat cancels the flight in the morning.
	s.clock.Sync(time.Now(), 7*60, s.clock.Warp())
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" })
	s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
	if !tn.Cancelled(f, day) {
		t.Fatal("the airport does not have the flight cancelled")
	}
	if got := open(f); got != avail.Closed {
		t.Errorf("a cancelled flight is %v for sale; want closed", got)
	}
	if !s.notFlying(f) {
		t.Error("a traveller is still offered the cancelled flight")
	}
	// The other leg of the same number, from where this one lands.
	sibling := f
	sibling.From, sibling.To = f.To, f.From
	if tn.Cancelled(sibling, day) {
		t.Errorf("cancelling %s%s from %s cancelled its leg from %s", f.Carrier, f.Number, f.From, sibling.From)
	}
	if open(h) != avail.Open || tn.Cancelled(h, day) {
		t.Error("another flight was closed or cancelled with it")
	}
	// The hour it would have landed: the departure is forgotten.
	sky := s.aloftAt(day, f.ArrMin-1)
	s.flyFlights(ctx, day, f.ArrMin-1, f.ArrMin, sky)
	if tn.Cancelled(f, day) {
		t.Error("tomorrow's flight is cancelled by today's cancellation")
	}
	if got := open(f); got != avail.Open {
		t.Errorf("tomorrow's flight is %v for sale; want open", got)
	}

	// A retime and a smaller aircraft, on the second flight.
	key := dcs.Key{Flight: h.Carrier + h.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: h.From}
	list := func() {
		t.Helper()
		if err := tn.SendPNL(ctx, h, day); err != nil {
			t.Fatal(err)
		}
		for wait := time.Now().Add(20 * time.Second); ; {
			if _, err := tn.DCS.Flight(key); err == nil {
				return
			}
			if time.Now().After(wait) {
				t.Fatal("departure control never opened the flight")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	list()
	if err := tn.Retime(ctx, h, day, 50, 50); err != nil {
		t.Fatal(err)
	}
	if sum, ok := tn.Summarise(h.Carrier+h.Number, h.From); !ok || sum.Retimed == "" {
		t.Fatalf("the flight was not retimed: %+v", sum)
	}
	if _, err := tn.Substitute(ctx, h, day); err != nil {
		t.Fatalf("Substitute: %v", err)
	}
	if !tn.Downgauged(h.Number, h.From) {
		t.Fatal("the smaller aircraft is not on the flight")
	}
	tn.Forget(ctx, h, day)
	if tn.Downgauged(h.Number, h.From) {
		t.Error("the smaller aircraft is still on tomorrow's flight")
	}
	list()
	if sum, ok := tn.Summarise(h.Carrier+h.Number, h.From); !ok || sum.Retimed != "" || sum.Substituted != "" {
		t.Errorf("tomorrow's flight begins retimed or substituted: %+v", sum)
	}
	if err := tn.Retime(ctx, h, day, 55, 55); err != nil {
		t.Fatal(err)
	}
	if sum, _ := tn.Summarise(h.Carrier+h.Number, h.From); sum == nil || !strings.Contains(sum.Retimed, "+55") {
		t.Errorf("tomorrow's delay was not announced: %+v", sum)
	}
}

// Reprotection offers flights that a passenger can still board. A flight
// is cancelled at noon. Of the later flights over its city pair, one the
// day has also cancelled is not offered, and one that has already left is
// offered for tomorrow and not for today.
func TestReprotectionOffersFlightsThatStillFly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	// A city pair with a morning flight and at least 2 later ones.
	type pair struct{ from, to string }
	by := map[pair][]world.Flight{}
	for code := range s.Tenants {
		for _, f := range s.Flights[code] {
			if !s.fate.Of(f).Cancelled {
				by[pair{f.From, f.To}] = append(by[pair{f.From, f.To}], f)
			}
		}
	}
	var dead, second, third world.Flight
	for _, fs := range by {
		var early world.Flight
		var later []world.Flight
		for _, f := range fs {
			if f.DepMin < 9*60 && (early.Number == "" || f.DepMin < early.DepMin) {
				early = f
			}
		}
		for _, f := range fs {
			if early.Number != "" && f.DepMin > early.DepMin+60 && f.Number != early.Number {
				later = append(later, f)
			}
		}
		if len(later) >= 2 {
			dead, second, third = early, later[0], later[1]
			if second.DepMin > third.DepMin {
				second, third = third, second
			}
			if second.DepMin != third.DepMin {
				break
			}
		}
	}
	if dead.Number == "" || second.DepMin == third.DepMin {
		t.Fatal("no city pair with a morning flight and 2 later ones")
	}
	day := s.BookingDate
	seg := pnr.Segment{Type: pnr.SegmentAir, Carrier: dead.Carrier, FlightNum: dead.Number, Board: dead.From, Off: dead.To, Depart: day}
	offered := func() map[string]int {
		t.Helper()
		cands, err := s.alternatives(ctx, seg)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		for _, c := range cands {
			days := int(c.Depart.Sub(day).Hours() / 24)
			out[c.Carrier+strings.TrimLeft(c.FlightNum, "0")] = days + 1 // 1 today, 2 tomorrow
		}
		return out
	}
	name := func(f world.Flight) string { return f.Carrier + strings.TrimLeft(f.Number, "0") }
	// Just after the dead flight's time, both later flights are offered
	// for today.
	s.clock.Sync(time.Now(), float64(dead.DepMin+1), s.clock.Warp())
	got := offered()
	if got[name(second)] != 1 || got[name(third)] != 1 {
		t.Fatalf("at %s the later flights are offered as %v; want %s and %s today", hhmm(dead.DepMin+1), got, name(second), name(third))
	}
	// The day cancels the third.
	s.fate.Update(third, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
	if got := offered(); got[name(third)] != 0 || got[name(second)] != 1 {
		t.Errorf("with %s cancelled the offer is %v; want it gone and %s today", name(third), got, name(second))
	}
	// The second leaves. It is tomorrow's flight now.
	dep, _ := s.flightDelays(second, day)
	s.clock.Sync(time.Now(), float64(second.DepMin+dep+5), s.clock.Warp())
	if got := offered(); got[name(second)] != 2 {
		t.Errorf("after %s left the offer is %v; want it for tomorrow", name(second), got)
	}
	// An airport that is closed takes its flights out of the offer.
	s.fate.Update(third, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = false, "", "" })
	s.clock.Sync(time.Now(), float64(dead.DepMin+1), s.clock.Warp())
	s.closedMu.Lock()
	s.closed[dead.To] = true
	s.closedMu.Unlock()
	if got := offered(); len(got) != 0 {
		t.Errorf("with %s closed the offer is %v; want nothing", dead.To, got)
	}
}

// A ticket's stock code names its carrier. Every carrier of a world has
// its own code, and a ticket sold before the day is on the same stock as
// one sold during it. The codes once came from a sum of the designator
// alone: FR and G3 both made 552, and the tickets written before the day
// carried codes from another sum that settlement did not know.
func TestEachCarrierHasItsOwnTicketStock(t *testing.T) {
	s := &Sim{carriers: map[string]world.Carrier{}}
	for _, code := range []string{"FR", "G3", "UA", "DL", "AA", "U2", "BA", "AF", "LH", "WN", "B6", "F9"} {
		s.carriers[code] = world.Carrier{Designator: code}
	}
	if stockSum("FR") != stockSum("G3") {
		t.Fatalf("the test's 2 carriers no longer start from the same code: %d and %d", stockSum("FR"), stockSum("G3"))
	}
	seen := map[string]string{}
	for code := range s.carriers {
		acct := s.accountingCode(code)
		if len(acct) != 3 || acct < "100" || acct > "999" {
			t.Errorf("%s has stock code %q", code, acct)
		}
		if other, taken := seen[acct]; taken {
			t.Errorf("%s and %s share stock code %s", code, other, acct)
		}
		seen[acct] = code
		if again := s.accountingCode(code); again != acct {
			t.Errorf("%s has code %s and then %s", code, acct, again)
		}
	}
	// FR comes first by designator and keeps the code; G3 takes the next.
	if s.accountingCode("FR") != "552" || s.accountingCode("G3") != "553" {
		t.Errorf("FR has %s and G3 %s; want 552 and 553", s.accountingCode("FR"), s.accountingCode("G3"))
	}
	// The seats sold before the day are ticketed on the same stock.
	opts := s.fillOptions(0)
	if opts.Accounting == nil {
		t.Fatal("the fill makes its own stock codes")
	}
	for code := range s.carriers {
		if got := opts.Accounting(code); got != s.accountingCode(code) {
			t.Errorf("%s: the fill tickets on %s and the day on %s", code, got, s.accountingCode(code))
		}
	}
}

// A booking made on the day stays in the books until its flight has
// left. A traveller books an evening flight. At 05:00 and at noon the
// carrier and the seller still hold the record. 3 hours after the flight
// has left, they let it go. The booking has a date and no time, and it
// was once dropped at 03:00, before the name list was made.
func TestABookingMadeOnTheDayIsKeptUntilItsFlightLeaves(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	code := m.Carriers[0].Designator
	var f world.Flight
	for _, g := range s.Flights[code] {
		if dep, _ := s.flightDelays(g, s.BookingDate); !s.fate.Of(g).Cancelled && g.Marketing == "" && g.DepMin+dep > 17*60 && g.DepMin+dep < 20*60 {
			f = g
			break
		}
	}
	if f.Number == "" {
		t.Fatal("no evening flight")
	}
	s.clock.Sync(time.Now(), 2*60, s.clock.Warp())
	res, err := s.Book(ctx, f, "Y", 0, "MORNING")
	if err != nil {
		t.Fatal(err)
	}
	loc := res.PNR.RecordLocator
	for wait := time.Now().Add(20 * time.Second); ; {
		if ok, _ := s.Settled(ctx, loc); ok {
			break
		}
		if time.Now().After(wait) {
			t.Fatal("the booking never settled")
		}
		time.Sleep(25 * time.Millisecond)
	}
	tn := s.Tenants[code]
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	held := func() (carrier, seller bool) {
		recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, wire, 100)
		for _, r := range recs {
			for _, p := range r.Passengers {
				if p.Surname == "MORNING" {
					carrier = true
				}
			}
		}
		_, err := s.GDSStore.GetPNR(ctx, loc)
		return carrier, err == nil
	}
	if c, g := held(); !c || !g {
		t.Fatalf("after the sale the carrier holds the record: %v, the seller: %v", c, g)
	}
	for _, at := range []float64{5 * 60, 12 * 60} {
		s.clock.Sync(time.Now(), at, s.clock.Warp())
		s.retire(ctx)
		if c, g := held(); !c || !g {
			t.Fatalf("at %s the booking for the %s flight is gone: carrier holds it %v, seller %v", hhmm(int(at)), hhmm(f.DepMin), c, g)
		}
	}
	dep, _ := s.flightDelays(f, s.BookingDate)
	s.clock.Sync(time.Now(), float64(f.DepMin+dep+181), s.clock.Warp())
	s.retire(ctx)
	if c, g := held(); c || g {
		t.Errorf("3 hours after the flight left the record is still held: carrier %v, seller %v", c, g)
	}
}
