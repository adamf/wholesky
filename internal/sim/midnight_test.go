package sim

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// lateFlight makes one of a carrier's flights leave at 23:00 and land at
// 01:00, on time in the day's plan, and returns it.
func lateFlight(t *testing.T, s *Sim, code string, depDelay int) world.Flight {
	t.Helper()
	return flightAt(t, s, code, 23*60, depDelay)
}

// flightAt makes one of a carrier's flights leave at a minute of the day
// and land 2 hours later, with a delay in the day's plan, and returns it.
func flightAt(t *testing.T, s *Sim, code string, depMin, depDelay int) world.Flight {
	t.Helper()
	fs := s.Flights[code]
	if len(fs) == 0 {
		t.Fatalf("%s has no flights", code)
	}
	f := &fs[0]
	f.DepMin, f.ArrMin, f.BlockMin = depMin, depMin+120, 120
	s.fate.Update(*f, func(pf *dayplan.Flight) {
		pf.Cancelled, pf.Reason, pf.Code = false, "", ""
		pf.DepDelay, pf.ArrDelay, pf.CTOT = depDelay, depDelay, 0
	})
	if d, a := s.flightDelays(*f, s.BookingDate); d != depDelay || a != depDelay {
		t.Fatalf("the plan gives the flight %d and %d minutes; the test set %d", d, a, depDelay)
	}
	return *f
}

// What leaves lands, also after midnight. A flight leaves at 23:00 and
// lands at 01:00. Its departure control record is open from its name
// list. The clock goes through midnight, and at 01:00 the flight lands:
// the record is let go, so that tomorrow's flight of the same number can
// open. A flight that landed after midnight once never landed, and every
// later day's name list was refused.
func TestAFlightLandsAfterMidnight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil // the aircraft's own reports are another test's; here the world tells the carrier
	code := m.Carriers[0].Designator
	f := lateFlight(t, s, code, 0)
	tn, day := s.Tenants[code], s.BookingDate
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	open := func() bool { _, err := tn.DCS.Flight(key); return err == nil }
	for wait := time.Now().Add(20 * time.Second); !open(); {
		if time.Now().After(wait) {
			t.Fatal("departure control never opened the flight")
		}
		time.Sleep(50 * time.Millisecond)
	}

	sky := s.aloftAt(day, 22*60)
	if _, in := sky.air[dayplan.Key(f)]; in {
		t.Fatal("at 22:00 a 23:00 flight is in the air")
	}
	left := s.Departures.Load()
	s.flyFlights(ctx, day, 22*60, 23*60-1, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("the flight left before 23:00")
	}
	s.flyFlights(ctx, day, 23*60-1, 23*60, sky)
	if sky.air[dayplan.Key(f)] == nil || s.Departures.Load() <= left {
		t.Fatal("at 23:00 the flight did not leave")
	}
	// The last minutes of the day, midnight, and the first hour.
	s.flyFlights(ctx, day, 23*60, 24*60, sky)
	if !open() {
		t.Fatal("the flight landed before midnight")
	}
	sky.overnight(s, day)
	s.flyFlights(ctx, day, -1, 59, sky)
	if !open() || sky.air[dayplan.Key(f)] == nil {
		t.Fatal("the flight landed before 01:00")
	}
	s.flyFlights(ctx, day, 59, 60, sky)
	if _, err := tn.DCS.Flight(key); !errors.Is(err, dcs.ErrFlightNotFound) {
		t.Errorf("at 01:00 the flight has not landed: its departure control record is still held (%v)", err)
	}
	if sky.air[dayplan.Key(f)] != nil {
		t.Error("the flight is still in the air after it landed")
	}
	// Every flight that leaves during the day and is due in before the
	// evening has landed by then: nothing is left in the air that should
	// be on the ground.
	s.flyFlights(ctx, day, 60, 22*60, sky)
	for key, x := range sky.air {
		if x.due <= 22*60 {
			t.Errorf("%s was due in at minute %d and is still in the air at 22:00", key, x.due)
		}
	}

	// A world that starts at 00:30 finds the flight in the air, and lands
	// it at 01:00.
	fresh := s.aloftAt(day, 30)
	if x := fresh.air[dayplan.Key(f)]; x == nil || x.due != 60 {
		t.Errorf("a world that starts at 00:30 has the flight %+v; want in the air, due at minute 60", x)
	}
	// A clock that jumps over 01:00 still lands it: what is overdue lands.
	s.flyFlights(ctx, day, 200, 201, fresh)
	if fresh.air[dayplan.Key(f)] != nil {
		t.Error("a flight whose landing minute the clock jumped over never landed")
	}
	if x := s.aloftAt(day, 23*60+30).air[dayplan.Key(f)]; x == nil || x.due != 25*60 {
		t.Errorf("a world that starts at 23:30 has the flight %+v; want in the air, due at minute 1500", x)
	}
}

// A flight that its delay pushes past midnight leaves in the morning,
// with the delay its own day gave it, and then lands. It once never left:
// its fares were counted and no flight was flown.
func TestAFlightDelayedPastMidnightLeaves(t *testing.T) {
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
	f := lateFlight(t, s, code, 120) // due out at 01:00 and in at 03:00
	day := s.BookingDate
	sky := s.aloftAt(day, 23*60)
	s.flyFlights(ctx, day, 23*60, 24*60, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("the flight left before midnight")
	}
	sky.overnight(s, day)
	if x := sky.late[dayplan.Key(f)]; x == nil || x.due != 60 || x.depDelay != 120 {
		t.Fatalf("at midnight the flight is %+v; want still to leave, at minute 60, 120 late", x)
	}
	// The new day's plan has the flight on time. Yesterday's flight keeps
	// yesterday's delay.
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.DepDelay, pf.ArrDelay = 0, 0 })
	s.flyFlights(ctx, day, -1, 59, sky)
	if sky.late[dayplan.Key(f)] == nil || sky.air[dayplan.Key(f)] != nil {
		t.Fatal("the flight left before 01:00")
	}
	at := s.Departures.Load()
	s.flyFlights(ctx, day, 59, 60, sky)
	x := sky.air[dayplan.Key(f)]
	if x == nil || sky.late[dayplan.Key(f)] != nil || s.Departures.Load() <= at {
		t.Fatalf("at 01:00 the flight did not leave: in the air %v", x != nil)
	}
	if x.due != 3*60 {
		t.Errorf("the flight is due in at minute %d, want 180", x.due)
	}
	s.flyFlights(ctx, day, 60, 3*60, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Error("at 03:00 the flight has not landed")
	}
}

// The clock at midnight, and the clock set back. A fall of more than half
// a day is midnight. A smaller fall is the clock being set, by a peer's
// sync or by the wall clock, and is not a new day: it once closed the day
// in the middle of the afternoon and began another.
func TestTheTurnOfTheDay(t *testing.T) {
	for _, c := range []struct {
		prev, cur      int
		wraps, stepped bool
	}{
		{1437, 1, true, false},
		{1439, 0, true, false},
		{900, 896, false, true},
		{900, 300, false, true},
		{10, 20, false, false},
		{720, 720, false, false},
		{-1, 0, false, false},
	} {
		if got := wrapped(c.prev, c.cur); got != c.wraps {
			t.Errorf("from %d to %d: midnight %v, want %v", c.prev, c.cur, got, c.wraps)
		}
		if got := steppedBack(c.prev, c.cur); got != c.stepped {
			t.Errorf("from %d to %d: stepped back %v, want %v", c.prev, c.cur, got, c.stepped)
		}
	}
}

// A cancellation is announced when it is decided. The plan cancels a
// flight 1 hour before its departure: a crew timed out. The timetable's
// moment for the announcement, 2 hours out, has passed. The flight is
// announced at the next tick all the same: distribution has the ASM CNL,
// departure control has it cancelled, and it does not depart. Left
// unannounced it kept its flight plan and its slots, and its passengers
// stayed checked in.
func TestACancellationDecidedLateIsAnnounced(t *testing.T) {
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
	var f world.Flight
	for _, g := range s.Flights[code] {
		if fate := s.fate.Of(g); !fate.Cancelled && g.DepMin > 6*60 && g.DepMin < 20*60 && g.Marketing == "" {
			f = g
			break
		}
	}
	if f.Number == "" {
		t.Fatal("no flight to cancel")
	}
	tn, day := s.Tenants[code], s.BookingDate
	dep, _ := s.flightDelays(f, day)
	sky := s.aloftAt(day, f.DepMin-90)
	// 90 minutes out: nothing is wrong yet.
	s.flyFlights(ctx, day, f.DepMin-90, f.DepMin-61, sky)
	if _, done := s.announced.Load(dayplan.Key(f)); done {
		t.Fatal("a flight that is not cancelled was announced cancelled")
	}
	// 60 minutes out the crew times out.
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
	s.flyFlights(ctx, day, f.DepMin-61, f.DepMin-60, sky)
	for wait := time.Now().Add(20 * time.Second); !tn.Cancelled(f, day); {
		if time.Now().After(wait) {
			t.Fatal("a flight cancelled 1 hour before departure was never announced: departure control still has it")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, done := s.announced.Load(dayplan.Key(f)); !done {
		t.Error("the cancellation is not on the day's list of announcements")
	}
	// It does not leave.
	s.flyFlights(ctx, day, f.DepMin-60, f.DepMin+dep+1, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Error("a cancelled flight left")
	}
	// A flight the plan cancels after its departure time is not
	// announced: it has left.
	var g world.Flight
	for _, x := range s.Flights[code] {
		if x.Number != f.Number && !s.fate.Of(x).Cancelled && x.DepMin > 6*60 && x.DepMin < 20*60 {
			g = x
			break
		}
	}
	gdep, _ := s.flightDelays(g, day)
	sky2 := s.aloftAt(day, g.DepMin+gdep-1)
	s.flyFlights(ctx, day, g.DepMin+gdep-1, g.DepMin+gdep, sky2)
	if sky2.air[dayplan.Key(g)] == nil {
		t.Fatal("the second flight did not leave")
	}
	s.fate.Update(g, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
	s.flyFlights(ctx, day, g.DepMin+gdep, g.DepMin+gdep+5, sky2)
	time.Sleep(200 * time.Millisecond)
	if _, done := s.announced.Load(dayplan.Key(g)); done || tn.Cancelled(g, day) {
		t.Error("a flight that had left was announced cancelled")
	}
	if sky2.air[dayplan.Key(g)] == nil {
		t.Error("a flight in the air was taken out of it by a cancellation")
	}
}

// An airport closes at noon. A flight that left it in the morning is not
// cancelled: it flew. A flight still to leave is cancelled through the
// day's plan, so it stays cancelled when the airport opens again, and it
// is announced once. The closure once cancelled every flight of the day
// that touched the airport, also those that had landed, and left the plan
// as it was, so the flights after the reopening departed although their
// cancellations had gone out.
func TestAClosedAirportCancelsWhatHasNotLeft(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	// The airport with a morning departure and an evening one.
	var gone, later world.Flight
	for code := range s.Tenants {
		for _, f := range s.Flights[code] {
			if s.fate.Of(f).Cancelled {
				continue
			}
			dep, _ := s.flightDelays(f, s.BookingDate)
			if f.DepMin+dep < 11*60 && gone.Number == "" {
				gone = f
			}
		}
	}
	for code := range s.Tenants {
		for _, f := range s.Flights[code] {
			if (f.From == gone.From || f.To == gone.From) && f.DepMin > 14*60 && !s.fate.Of(f).Cancelled {
				later = f
			}
		}
	}
	if gone.Number == "" || later.Number == "" {
		t.Fatal("no airport with a morning and an evening flight")
	}
	iata := gone.From
	s.clock.Sync(time.Now(), 12*60, s.clock.Warp())
	s.closedMu.Lock()
	s.closed[iata] = true
	s.closedMu.Unlock()
	s.cancelFlightsTouching(iata)
	day := s.BookingDate
	if s.fate.Of(gone).Cancelled || s.Tenants[gone.Carrier].Cancelled(gone, day) {
		t.Errorf("%s%s left %s at %s and was cancelled by a closure at 12:00", gone.Carrier, gone.Number, iata, hhmm(gone.DepMin))
	}
	if st := (seatWorld{s}).flightStatus(gone, s.fate.Of(gone), 12*60); st == "cancelled" {
		t.Errorf("the scorecard has the morning flight as %s", st)
	}
	fate := s.fate.Of(later)
	if !fate.Cancelled || fate.Code != "B" || !s.Tenants[later.Carrier].Cancelled(later, day) {
		t.Errorf("%s%s at %s was not cancelled through the plan: %+v", later.Carrier, later.Number, hhmm(later.DepMin), fate)
	}
	if _, done := s.announced.Load(dayplan.Key(later)); !done {
		t.Error("the cancellation was not announced")
	}
	// The airport opens again. The flight stays cancelled and does not
	// leave.
	s.closedMu.Lock()
	delete(s.closed, iata)
	s.closedMu.Unlock()
	dep, _ := s.flightDelays(later, day)
	sky := s.aloftAt(day, later.DepMin+dep-1)
	s.flyFlights(ctx, day, later.DepMin+dep-1, later.DepMin+dep, sky)
	if sky.air[dayplan.Key(later)] != nil {
		t.Error("after the airport opened again, a flight that had been cancelled left")
	}
}
