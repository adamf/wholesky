package sim

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/dcs"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// forSale is what the carrier answers for a seat in Y on a flight.
func forSale(t *testing.T, tn *host.Tenant, f world.Flight, day time.Time) avail.Status {
	t.Helper()
	got := tn.Inventory.Availability([]avail.Key{avail.NewKey(f.Carrier, f.Number, day, f.From, f.To, "Y")}, time.Now())
	if len(got) != 1 {
		t.Fatalf("availability of %s%s: %d answers", f.Carrier, f.Number, len(got))
	}
	return got[0].Status
}

// listed sends a flight's name list and waits for departure control to
// open the flight. It reports false when the flight never opens.
func listed(ctx context.Context, t *testing.T, tn *host.Tenant, f world.Flight, day time.Time) bool {
	t.Helper()
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	for wait := time.Now().Add(5 * time.Second); time.Now().Before(wait); time.Sleep(25 * time.Millisecond) {
		if _, err := tn.DCS.Flight(key); err == nil {
			return true
		}
	}
	return false
}

// flies gives a flight the new day's plan: on time and not cancelled. The
// crews of a new day are drawn again, so the test says what it needs.
func flies(s *Sim, f world.Flight) {
	s.fate.Update(f, func(pf *dayplan.Flight) {
		pf.Cancelled, pf.Reason, pf.Code = false, "", ""
		pf.DepDelay, pf.ArrDelay, pf.CTOT = 0, 0, 0
	})
}

// A cancellation is one day's. A flight due out at 23:00 and in at 01:00
// is cancelled. At midnight the day ends, and the next day's flight of
// the same number is not cancelled: it is for sale, travellers are
// offered it, and its name list opens it at the airport. The hour that
// yesterday's flight would have landed, 01:00, changes none of that.
//
// The cancellation once ended when the flight would have landed. For a
// flight due in after midnight that hour came on the morning before the
// cancellation and never after it, so the flight stayed cancelled and
// closed for sale all the next day, with no check-in.
func TestACancellationEndsWithItsDay(t *testing.T) {
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
	f := lateFlight(t, s, code, 0)
	tn, day := s.Tenants[code], s.BookingDate
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	if got := forSale(t, tn, f, day); got != avail.Open {
		t.Fatalf("before anything the flight is %v for sale", got)
	}
	// The name list has gone and the flight is open at the airport when
	// its crew times out. The cancellation is announced 2 hours out.
	if !listed(ctx, t, tn, f, day) {
		t.Fatal("departure control never opened the flight")
	}
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
	sky := s.aloftAt(day, 20*60)
	s.flyFlights(ctx, day, 20*60, 21*60, sky)
	for wait := time.Now().Add(20 * time.Second); !tn.Cancelled(f, day); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(wait) {
			t.Fatal("the cancellation was never announced")
		}
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Fatalf("the cancelled flight is %v for sale; want closed", got)
	}
	// The evening, in steps. The flight does not leave, and it stays
	// closed for sale until the day ends.
	s.flyFlights(ctx, day, 21*60, 23*60+30, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("the cancelled flight left")
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("at 23:30 the cancelled flight is %v for sale; want closed until the day ends", got)
	}
	s.flyFlights(ctx, day, 23*60+30, 24*60, sky)
	sky.overnight(s, day)
	s.newDay()
	flies(s, f)

	check := func(when string) {
		t.Helper()
		if tn.Cancelled(f, day) {
			t.Errorf("%s: the next day's flight is cancelled by yesterday's cancellation", when)
		}
		if got := forSale(t, tn, f, day); got != avail.Open {
			t.Errorf("%s: the next day's flight is %v for sale; want open", when, got)
		}
		if s.notFlying(f) {
			t.Errorf("%s: travellers are not offered the next day's flight", when)
		}
	}
	check("at 00:00")
	if _, err := tn.DCS.Flight(key); !errors.Is(err, dcs.ErrFlightNotFound) {
		t.Errorf("at 00:00 the airport still holds yesterday's cancelled flight (%v)", err)
	}
	// 01:00, when yesterday's flight would have landed, and on to 02:00.
	s.flyFlights(ctx, day, -1, 2*60, sky)
	check("at 02:00")
	if !listed(ctx, t, tn, f, day) {
		t.Error("the next day's name list did not open the flight: the airport still has it cancelled")
	}
	// It leaves at 23:00.
	s.flyFlights(ctx, day, 2*60, 23*60, sky)
	if sky.air[dayplan.Key(f)] == nil {
		t.Error("the next day's flight did not leave")
	}
}

// The hour that yesterday's cancelled flight would have landed is no
// flight's. Yesterday's 23:00 flight was cancelled, and it would have
// landed at 01:00. At 00:10 the carrier cancels today's flight of the
// same number. At 01:00 today's flight is still cancelled at the
// airport: the hour belongs to a cancellation that ended at midnight.
func TestYesterdaysCancelledFlightLeavesTodaysAlone(t *testing.T) {
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
	f := lateFlight(t, s, code, 0)
	tn, day := s.Tenants[code], s.BookingDate
	why := func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" }
	s.fate.Update(f, why)
	sky := s.aloftAt(day, 20*60)
	s.flyFlights(ctx, day, 20*60, 24*60, sky)
	for wait := time.Now().Add(20 * time.Second); !tn.Cancelled(f, day); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(wait) {
			t.Fatal("the cancellation was never announced")
		}
	}
	if x := sky.gone[dayplan.Key(f)]; x == nil || x.due != 25*60 {
		t.Fatalf("at midnight the cancelled flight is %+v; want to be let go at minute 1500", x)
	}
	sky.overnight(s, day)
	if len(sky.gone) != 0 {
		t.Errorf("after midnight %d of yesterday's cancelled flights still wait for the hour they would have landed", len(sky.gone))
	}
	s.newDay()
	flies(s, f)
	s.clock.Sync(time.Now(), 10, s.clock.Warp())
	s.fate.Update(f, why)
	s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
	if !tn.Cancelled(f, day) {
		t.Fatal("today's flight is not cancelled at the airport")
	}
	s.flyFlights(ctx, day, -1, 2*60, sky)
	if !tn.Cancelled(f, day) {
		t.Error("the hour yesterday's flight would have landed took today's cancellation")
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("today's cancelled flight is %v for sale; want closed", got)
	}
}

// A flight in the first hours of a day has its name list sent the
// evening before. Today's flight, due out at 00:30, is cancelled. At
// 02:30, when it would have landed, the airport lets it go, and it stays
// closed for sale. In the evening the name list of tomorrow's flight
// opens tomorrow's flight at the airport. At midnight that record is
// still there: the day's end gives the flight back for sale, and does
// not take what is the next flight's.
func TestACancelledEarlyFlightDoesNotStopTheNextOne(t *testing.T) {
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
	f := flightAt(t, s, code, 30, 0)
	tn, day := s.Tenants[code], s.BookingDate
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	if !listed(ctx, t, tn, f, day) {
		t.Fatal("departure control never opened the flight")
	}
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
	sky := s.aloftAt(day, 0)
	s.flyFlights(ctx, day, -1, 10, sky)
	for wait := time.Now().Add(20 * time.Second); !tn.Cancelled(f, day); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(wait) {
			t.Fatal("the cancellation was never announced")
		}
	}
	s.flyFlights(ctx, day, 10, 149, sky)
	if _, err := tn.DCS.Flight(key); err != nil || !tn.Cancelled(f, day) {
		t.Fatalf("before 02:30 the airport has let the cancelled flight go: record %v, cancelled %v", err, tn.Cancelled(f, day))
	}
	s.flyFlights(ctx, day, 149, 150, sky)
	if _, err := tn.DCS.Flight(key); !errors.Is(err, dcs.ErrFlightNotFound) {
		t.Errorf("at 02:30 the airport still holds the cancelled flight (%v)", err)
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("at 02:30 the cancelled flight is %v for sale; want closed until the day ends", got)
	}
	// 21:30: the name list of tomorrow's flight.
	s.flyFlights(ctx, day, 150, 21*60, sky)
	if !listed(ctx, t, tn, f, day) {
		t.Fatal("the name list of tomorrow's flight did not open it: the airport still has today's cancellation")
	}
	s.flyFlights(ctx, day, 21*60, 24*60, sky)
	sky.overnight(s, day)
	s.newDay()
	flies(s, f)
	if _, err := tn.DCS.Flight(key); err != nil {
		t.Errorf("midnight took the record of the flight that leaves at 00:30: %v", err)
	}
	if got := forSale(t, tn, f, day); got != avail.Open {
		t.Errorf("the new day's flight is %v for sale; want open", got)
	}
	if tn.Cancelled(f, day) {
		t.Error("the new day's flight is cancelled by yesterday's cancellation")
	}
}

// Yesterday's flight lands, and today's flight of the same number stays
// cancelled. A flight leaves at 23:00 and lands at 01:00. At 00:10 the
// carrier cancels today's flight of that number, while yesterday's is in
// the air. At 01:00 yesterday's flight lands and its departure control
// record is let go. Today's flight is still cancelled and still closed
// for sale, and it does not leave at 23:00.
//
// The 2 flights have the same key, because the world flies the same date
// each day. The landing once took everything under the key with it, the
// cancellation also, and today's flight went back on sale.
func TestALandingLeavesTodaysCancellationAlone(t *testing.T) {
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
	f := lateFlight(t, s, code, 0)
	tn, day := s.Tenants[code], s.BookingDate
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	if !listed(ctx, t, tn, f, day) {
		t.Fatal("departure control never opened the flight")
	}
	sky := s.aloftAt(day, 22*60)
	s.flyFlights(ctx, day, 22*60, 24*60, sky)
	if sky.air[dayplan.Key(f)] == nil {
		t.Fatal("the flight did not leave at 23:00")
	}
	sky.overnight(s, day)
	s.newDay()
	flies(s, f)
	if sky.air[dayplan.Key(f)] == nil {
		t.Fatal("at midnight yesterday's flight is not in the air")
	}

	// 00:10: the carrier cancels today's flight.
	s.clock.Sync(time.Now(), 10, s.clock.Warp())
	s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" })
	s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
	if !tn.Cancelled(f, day) || forSale(t, tn, f, day) != avail.Closed {
		t.Fatalf("today's flight is not cancelled and closed: cancelled %v, for sale %v", tn.Cancelled(f, day), forSale(t, tn, f, day))
	}
	if _, err := tn.DCS.Flight(key); err != nil {
		t.Fatalf("the cancellation took the record of the flight that is in the air: %v", err)
	}

	// 01:00: yesterday's flight lands.
	s.flyFlights(ctx, day, -1, 59, sky)
	if sky.air[dayplan.Key(f)] == nil {
		t.Fatal("yesterday's flight landed before 01:00")
	}
	s.flyFlights(ctx, day, 59, 60, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("yesterday's flight did not land at 01:00")
	}
	if _, err := tn.DCS.Flight(key); !errors.Is(err, dcs.ErrFlightNotFound) {
		t.Errorf("yesterday's flight landed and its departure control record is still held (%v)", err)
	}
	if !tn.Cancelled(f, day) {
		t.Error("yesterday's landing took today's cancellation with it")
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("after yesterday's flight landed, today's cancelled flight is %v for sale; want closed", got)
	}
	if !s.notFlying(f) {
		t.Error("travellers are offered today's cancelled flight")
	}
	// Today's flight does not leave, and nothing lands for it.
	s.flyFlights(ctx, day, 60, 23*60+1, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Error("today's cancelled flight left")
	}
}

// closeAirport closes an airport at a minute of the day, and waits for
// the cancellations to go out.
func closeAirport(s *Sim, iata string, minute float64) {
	s.clock.Sync(time.Now(), minute, s.clock.Warp())
	s.closedMu.Lock()
	s.closed[iata] = true
	s.closedMu.Unlock()
	s.cancelFlightsTouching(iata)
}

func reopenAirport(s *Sim, iata string) {
	s.closedMu.Lock()
	delete(s.closed, iata)
	s.closedMu.Unlock()
}

// eveningFlightAt finds an airport and a flight that touches it and
// leaves after 14:00, on a carrier this process flies.
func eveningFlightAt(t *testing.T, s *Sim) (string, world.Flight) {
	t.Helper()
	for _, code := range s.Manifest.Carriers {
		for _, f := range s.Flights[code.Designator] {
			if f.Marketing == "" && f.DepMin > 14*60 && f.ArrMin < 23*60 && !s.fate.Of(f).Cancelled {
				return f.From, f
			}
		}
	}
	t.Fatal("no evening flight")
	return "", world.Flight{}
}

// A closure's cancellations are the day's also. An airport closes at
// noon, and a flight due out of it in the evening is cancelled. The
// airport is still closed at the hour the flight would have landed. It
// opens again at 23:00. The next day the flight is in the plan, it is
// not cancelled at the airport, it is for sale, and it leaves.
//
// The world here flies with no crew model, so its plan is the same each
// day. The closure once left the flight cancelled in that plan for every
// later day. The carrier also kept it cancelled: the hour it would have
// landed was passed over while the airport was closed.
func TestAClosuresCancellationsEndWithTheDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, NoCrew: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	iata, f := eveningFlightAt(t, s)
	tn, day := s.Tenants[f.Carrier], s.BookingDate
	before := s.fate.Of(f)

	closeAirport(s, iata, 12*60)
	if !s.fate.Of(f).Cancelled || !tn.Cancelled(f, day) || forSale(t, tn, f, day) != avail.Closed {
		t.Fatalf("the closure did not cancel %s%s: plan %v, airport %v, for sale %v", f.Carrier, f.Number, s.fate.Of(f).Cancelled, tn.Cancelled(f, day), forSale(t, tn, f, day))
	}
	sky := s.aloftAt(day, 12*60)
	s.flyFlights(ctx, day, 12*60, 23*60, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("a flight left a closed airport")
	}
	if tn.Cancelled(f, day) {
		t.Error("at a closed airport the cancelled flight was not let go at the hour it would have landed")
	}
	reopenAirport(s, iata)
	s.flyFlights(ctx, day, 23*60, 24*60, sky)
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("the airport opened again and the cancelled flight is %v for sale the same day; want closed", got)
	}
	sky.overnight(s, day)
	s.newDay()

	after := s.fate.Of(f)
	if after.Cancelled {
		t.Errorf("the next day's plan still has %s%s cancelled: %q", f.Carrier, f.Number, after.Reason)
	}
	if after.DepDelay != before.DepDelay {
		t.Errorf("the next day's plan gives the flight %d minutes of delay; the first day gave it %d", after.DepDelay, before.DepDelay)
	}
	if tn.Cancelled(f, day) {
		t.Error("the next day the airport still has the flight cancelled")
	}
	if got := forSale(t, tn, f, day); got != avail.Open {
		t.Errorf("the next day the flight is %v for sale; want open", got)
	}
	if s.notFlying(f) {
		t.Error("the next day travellers are not offered the flight")
	}
	dep, _ := s.flightDelays(f, day)
	s.flyFlights(ctx, day, -1, f.DepMin+dep, sky)
	if sky.air[dayplan.Key(f)] == nil {
		t.Error("the next day the flight did not leave")
	}
}

// An airport that is closed over midnight cancels the next day's flights
// too. The new day's plan has them flying, so the closure cancels them
// again when the day begins: through the plan, announced, and closed for
// sale. The day after the airport opens, they fly.
//
// Left to the plan alone, the new day's flights at a closed airport were
// neither flown nor cancelled, and nobody told their passengers.
func TestAnAirportClosedOverMidnightCancelsTheNextDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, NoCrew: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	iata, f := eveningFlightAt(t, s)
	tn, day := s.Tenants[f.Carrier], s.BookingDate

	closeAirport(s, iata, 20*60)
	sky := s.aloftAt(day, 20*60)
	s.flyFlights(ctx, day, 20*60, 24*60, sky)
	sky.overnight(s, day)
	s.clock.Sync(time.Now(), 0, s.clock.Warp())
	s.newDay()
	cancelled := func() bool {
		_, told := s.announced.Load(dayplan.Key(f))
		return told && s.fate.Of(f).Cancelled && tn.Cancelled(f, day)
	}
	for wait := time.Now().Add(20 * time.Second); !cancelled(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(wait) {
			_, told := s.announced.Load(dayplan.Key(f))
			t.Fatalf("the airport is still closed, and the new day's flight is not cancelled: plan %v, airport %v, announced %v", s.fate.Of(f).Cancelled, tn.Cancelled(f, day), told)
		}
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("the new day's flight at a closed airport is %v for sale; want closed", got)
	}
	dep, _ := s.flightDelays(f, day)
	s.flyFlights(ctx, day, -1, 10*60, sky)
	// The airport opens at 10:00. The flight stays cancelled for the day.
	reopenAirport(s, iata)
	s.flyFlights(ctx, day, 10*60, 24*60, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Error("a flight cancelled in the morning left in the evening")
	}
	sky.overnight(s, day)
	s.newDay()
	if s.fate.Of(f).Cancelled || tn.Cancelled(f, day) || forSale(t, tn, f, day) != avail.Open {
		t.Errorf("the day after the airport opened, the flight is cancelled or closed: plan %v, airport %v, for sale %v", s.fate.Of(f).Cancelled, tn.Cancelled(f, day), forSale(t, tn, f, day))
	}
	s.flyFlights(ctx, day, -1, f.DepMin+dep, sky)
	if sky.air[dayplan.Key(f)] == nil {
		t.Error("the day after the airport opened, the flight did not leave")
	}
}

// A flight that comes due at a closed airport is cancelled there and
// then. The closure's own cancellations go out one flight at a time, and
// they stop if the airport opens again. A flight that they have not
// reached when its departure minute comes was once passed over: it did
// not leave and it was not cancelled.
func TestAFlightDueAtAClosedAirportIsCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, NoCrew: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	iata, f := eveningFlightAt(t, s)
	tn, day := s.Tenants[f.Carrier], s.BookingDate
	dep, _ := s.flightDelays(f, day)
	// The airport is closed, and no cancellation has gone out.
	s.closedMu.Lock()
	s.closed[iata] = true
	s.closedMu.Unlock()
	sky := s.aloftAt(day, f.DepMin+dep-5)
	s.flyFlights(ctx, day, f.DepMin+dep-5, f.DepMin+dep-1, sky)
	if s.fate.Of(f).Cancelled {
		t.Fatal("the flight was cancelled before its departure minute, with no cancellation sent")
	}
	s.flyFlights(ctx, day, f.DepMin+dep-1, f.DepMin+dep, sky)
	if sky.air[dayplan.Key(f)] != nil {
		t.Fatal("a flight left a closed airport")
	}
	fate := s.fate.Of(f)
	if !fate.Cancelled || fate.Code != "B" || !strings.Contains(fate.Reason, iata) {
		t.Fatalf("a flight due out of a closed airport is not cancelled in the plan: %+v", fate)
	}
	for wait := time.Now().Add(20 * time.Second); !tn.Cancelled(f, day); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(wait) {
			t.Fatal("the cancellation was never announced")
		}
	}
	if got := forSale(t, tn, f, day); got != avail.Closed {
		t.Errorf("the flight is %v for sale; want closed", got)
	}
}

// A cancellation is announced within one day, and only while the day's
// plan holds it.
//
//   - A flight that the plan flies is not announced as cancelled.
//   - While the day ends, an announcement waits. It then reads the new
//     day's plan.
//   - While an announcement is on its way, the day's end waits for it.
//
// An announcement begun in the last minute of a day could once mark the
// flight cancelled at the airport after midnight had let the day's
// cancellations go. The mark then stood all the next day.
func TestACancellationIsAnnouncedWithinItsDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, NoCrew: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	_, f := eveningFlightAt(t, s)
	tn, day := s.Tenants[f.Carrier], s.BookingDate
	told := func() bool { _, ok := s.announced.Load(dayplan.Key(f)); return ok }
	why := func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" }

	// The plan flies the flight.
	s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
	if told() || tn.Cancelled(f, day) || forSale(t, tn, f, day) != avail.Open {
		t.Fatalf("a flight that the plan flies was announced cancelled: announced %v, airport %v, for sale %v", told(), tn.Cancelled(f, day), forSale(t, tn, f, day))
	}

	// The day is ending when the announcement starts.
	s.fate.Update(f, why)
	s.turn.Lock()
	done := make(chan struct{})
	go func() {
		s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	if told() || tn.Cancelled(f, day) {
		t.Error("a cancellation was announced while the day was ending")
	}
	s.turn.Unlock()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the announcement never went out")
	}
	if !told() || !tn.Cancelled(f, day) {
		t.Fatalf("after the day's end the cancellation was not announced: announced %v, airport %v", told(), tn.Cancelled(f, day))
	}

	// An announcement is on its way when the day ends.
	s.turn.RLock()
	turned := make(chan struct{})
	go func() {
		s.newDay()
		close(turned)
	}()
	time.Sleep(200 * time.Millisecond)
	if !tn.Cancelled(f, day) || !told() {
		t.Error("the day ended while an announcement was on its way")
	}
	s.turn.RUnlock()
	select {
	case <-turned:
	case <-time.After(20 * time.Second):
		t.Fatal("the day never ended")
	}
	if tn.Cancelled(f, day) || told() || s.fate.Of(f).Cancelled {
		t.Errorf("the new day has yesterday's cancellation: airport %v, announced %v, plan %v", tn.Cancelled(f, day), told(), s.fate.Of(f).Cancelled)
	}
	// An announcement that yesterday began and that starts only now reads
	// the new day's plan, which flies the flight.
	s.announceCancellation(ctx, tn, f, day, "cancelled by operations")
	if told() || tn.Cancelled(f, day) {
		t.Error("yesterday's cancellation was announced in the new day")
	}
}

// No carrier holds a cancellation that the day's plan does not. A day is
// flown with 1 flight in 7 cancelled, at every hour, and with an airport
// closed for the afternoon. The day ends. At 00:00 no flight is cancelled
// at its airport and every flight is for sale. The new day is flown to
// its end, and at each step a flight is cancelled at its airport, or
// closed for sale, only if the new day's plan cancels it.
func TestNoCancellationOutlivesItsDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.DSP = nil
	day := s.BookingDate
	var all []world.Flight
	for code := range s.Tenants {
		for _, f := range s.Flights[code] {
			if f.Marketing == "" {
				all = append(all, f)
			}
		}
	}
	// stale counts the flights a carrier has cancelled, or closed for
	// sale, that the plan flies.
	stale := func() (marked, closed, planned int, first string) {
		for _, f := range all {
			tn := s.Tenants[f.Carrier]
			plan := s.fate.Of(f).Cancelled
			if plan {
				planned++
			}
			if tn.Cancelled(f, day) && !plan {
				marked++
				first = f.Carrier + f.Number + " from " + f.From
			}
			if forSale(t, tn, f, day) == avail.Closed && !plan {
				closed++
				first = f.Carrier + f.Number + " from " + f.From
			}
		}
		return
	}
	settle := func() {
		// The announcements go out beside the day's loop.
		for wait, last := time.Now().Add(10*time.Second), -1; time.Now().Before(wait); time.Sleep(100 * time.Millisecond) {
			n := 0
			s.announced.Range(func(_, _ any) bool { n++; return true })
			if n == last {
				return
			}
			last = n
		}
	}

	// Day 1: 1 flight in 7 is cancelled, and an airport closes from 15:00
	// to 18:00.
	cancelledDay1 := 0
	for i, f := range all {
		if i%7 == 0 {
			s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "crew: no legal crew", "A" })
			cancelledDay1++
		}
	}
	iata := all[0].From
	sky := s.aloftAt(day, 0)
	prev := -1
	fly := func(to int) {
		for prev < to {
			next := min(prev+30, to)
			s.clock.Sync(time.Now(), float64(next), s.clock.Warp())
			s.flyFlights(ctx, day, prev, next, sky)
			prev = next
		}
	}
	fly(15 * 60)
	s.closedMu.Lock()
	s.closed[iata] = true
	s.closedMu.Unlock()
	s.cancelFlightsTouching(iata)
	fly(18 * 60)
	reopenAirport(s, iata)
	fly(24 * 60)
	settle()
	held := 0
	for _, f := range all {
		if forSale(t, s.Tenants[f.Carrier], f, day) == avail.Closed {
			held++
		}
	}
	if held < cancelledDay1 {
		t.Fatalf("day 1 cancelled %d flights and more at %s, and %d are closed for sale at its end", cancelledDay1, iata, held)
	}

	// Midnight.
	sky.overnight(s, day)
	s.clock.Sync(time.Now(), 0, s.clock.Warp())
	s.newDay()
	for _, f := range all {
		tn := s.Tenants[f.Carrier]
		if tn.Cancelled(f, day) || forSale(t, tn, f, day) != avail.Open {
			t.Fatalf("at 00:00 %s%s from %s is cancelled at its airport (%v) or not for sale (%v)", f.Carrier, f.Number, f.From, tn.Cancelled(f, day), forSale(t, tn, f, day))
		}
	}

	// Day 2, with the plan the world made for it.
	prev = -1
	for _, to := range []int{2 * 60, 6 * 60, 12 * 60, 18 * 60, 24 * 60} {
		fly(to)
		settle()
		marked, closed, planned, first := stale()
		if marked > 0 || closed > 0 {
			t.Fatalf("day 2 at %s: %d flights are cancelled at their airports and %d are closed for sale that the plan flies, the first %s (the plan cancels %d)", hhmm(to), marked, closed, first, planned)
		}
	}
	t.Logf("%d flights; day 1 cancelled %d and the flights at %s; %d were closed for sale at its end; day 2 has none that its plan does not cancel", len(all), cancelledDay1, iata, held)
}
