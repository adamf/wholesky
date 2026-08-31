package sim

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/world"
)

// The invariant suite: laws the world must obey no matter what the traffic
// did. Each test boots a real world -- real sockets, real messages -- drives
// load through the public surfaces, waits for quiet, and then audits the
// stores. Nothing here inspects internals: every assertion is against what
// the systems themselves recorded.

func bootWorld(t *testing.T, opts Options) *Sim {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	if opts.Log == nil {
		opts.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s, err := Boot(ctx, smallWorld(t), opts)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(s.Stop)
	return s
}

func settleAll(t *testing.T, s *Sim, locators []string) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for _, l := range locators {
		for {
			if ok, _ := s.Settled(ctx, l); ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("booking %s never settled", l)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
}

// quiesce waits until every switch message older than a grace period has
// reached a terminal state. The world itself never goes silent -- tenants
// rebroadcast availability on a timer, which is the point of a living world
// -- so conservation is stated as freshness: transients may exist, stale
// transients may not.
func quiesce(t *testing.T, s *Sim) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for {
		msgs, err := s.Switch.Store.ListMessages(ctx, store.MessageFilter{Limit: 10000})
		if err != nil {
			t.Fatal(err)
		}
		cutoff := time.Now().Add(-5 * time.Second)
		stale := 0
		for _, m := range msgs {
			if m.At.After(cutoff) {
				continue
			}
			if m.Status == store.StatusReceived || m.Status == store.StatusDecoded {
				stale++
			}
		}
		if stale == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d messages older than the grace period are still transient", stale)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// confirmedSeats sums the seats a store holds at HK for one flight and class.
func confirmedSeats(t *testing.T, st store.Store, carrier, number, class string) int {
	t.Helper()
	recs, err := st.ListPNRs(context.Background(), 10000)
	if err != nil {
		t.Fatal(err)
	}
	seats := 0
	for _, r := range recs {
		for _, sg := range r.Segments {
			if sg.Carrier == carrier && sg.FlightNum == number &&
				sg.Class == class && sg.Status == "HK" {
				seats += sg.Seats
			}
		}
	}
	return seats
}

// No flight may confirm more seats than its capacity, no matter how many
// bookings arrive or which path they take. The carrier's inventory is the
// only authority, and both its own store and the GDS's copies must agree
// with it after the dust settles.
func TestInvariantNoOversell(t *testing.T) {
	s := bootWorld(t, Options{Capacity: 2})
	ctx := context.Background()

	c := s.Manifest.Carriers[0]
	f := s.Flights[c.Designator][0]

	var locators []string
	for i := 0; i < 6; i++ {
		res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("OVERSELL%d", i))
		if err != nil {
			t.Fatalf("Book %d: %v", i, err)
		}
		locators = append(locators, res.PNR.RecordLocator)
	}
	settleAll(t, s, locators)
	quiesce(t, s)

	carrier := confirmedSeats(t, s.Tenants[c.Designator].Store, c.Designator, f.Number, "Y")
	gds := confirmedSeats(t, s.GDSStore, c.Designator, f.Number, "Y")
	if carrier > 2 {
		t.Errorf("the carrier's own store confirms %d seats on %s%s Y against capacity 2",
			carrier, c.Designator, f.Number)
	}
	if gds > 2 {
		t.Errorf("the distribution store confirms %d seats on %s%s Y against capacity 2",
			gds, c.Designator, f.Number)
	}
	if carrier != gds {
		t.Errorf("the carrier confirms %d seats, the GDS believes %d: the copies diverged",
			carrier, gds)
	}
	// Six bookings against two seats: the pressure must have been real.
	statuses := map[string]int{}
	for _, l := range locators {
		rec, err := s.GDSStore.GetPNR(ctx, l)
		if err != nil {
			t.Fatal(err)
		}
		for _, sg := range rec.Segments {
			statuses[sg.Status]++
		}
	}
	if statuses["HK"] == 6 {
		t.Errorf("all six bookings confirmed; capacity was never enforced: %v", statuses)
	}
	t.Logf("outcome under pressure: %v", statuses)
}

// Every message the switch accepted must reach a terminal state: relayed on,
// applied, or -- when a link is down -- undeliverable. Nothing may sit in a
// transient state after the world goes quiet, and with every link healthy
// nothing may be undeliverable or dead-lettered at the switch.
func TestInvariantSwitchConservation(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()

	var locators []string
	for i, c := range s.Manifest.Carriers {
		fs := s.Flights[c.Designator]
		if len(fs) == 0 {
			continue
		}
		for j := 0; j < 3; j++ {
			f := fs[j%len(fs)]
			res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("CONSERVE%dX%d", i, j))
			if err != nil {
				t.Fatalf("Book: %v", err)
			}
			locators = append(locators, res.PNR.RecordLocator)
		}
	}
	settleAll(t, s, locators)
	quiesce(t, s)

	msgs, err := s.Switch.Store.ListMessages(ctx, store.MessageFilter{Limit: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) == 0 {
		t.Fatal("no messages crossed the switch")
	}
	counts := map[store.Status]int{}
	for _, m := range msgs {
		counts[m.Status]++
	}
	t.Logf("switch ledger: %d messages, %v", len(msgs), counts)
	if counts[store.StatusUndeliverable] > 0 {
		t.Errorf("%d undeliverable messages with every link healthy", counts[store.StatusUndeliverable])
	}
	if counts[store.StatusDLQ] > 0 {
		t.Errorf("%d messages dead-lettered with every link healthy", counts[store.StatusDLQ])
	}
}

// A settled interline booking exists at every carrier that operates a leg of
// it, with that carrier's own leg confirmed in its own store. The messages
// are the only thing connecting these systems; this is the law that says the
// messages suffice.
func TestInvariantInterlineConvergence(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()

	// Find a legal cross-carrier connection in the compiled schedule.
	var f1, f2 world.Flight
	found := false
	for _, c := range s.Manifest.Carriers {
		for _, a := range s.Flights[c.Designator] {
			for _, b := range s.flightsByOrigin[a.To] {
				if b.Carrier != a.Carrier && b.DepMin >= a.ArrMin+40 && b.To != a.From {
					f1, f2, found = a, b, true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("the trimmed world offers no cross-carrier connection")
	}

	date := strings.ToUpper(s.BookingDate.Format("02Jan"))
	surname := "CONVERGE1"
	res, err := s.GDS.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: surname, Given: "INV", Title: "MR"}},
		Segments: []gateway.BookingSegment{
			{Carrier: f1.Carrier, FlightNum: f1.Number, Class: "Y", Date: date,
				Board: f1.From, Off: f1.To, Seats: 1},
			{Carrier: f2.Carrier, FlightNum: f2.Number, Class: "Y", Date: date,
				Board: f2.From, Off: f2.To, Seats: 1},
		},
		Agent: "invariants", Channel: "test",
	})
	if err != nil {
		t.Fatalf("interline Book: %v", err)
	}
	settleAll(t, s, []string{res.PNR.RecordLocator})
	quiesce(t, s)

	rec, err := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator)
	if err != nil {
		t.Fatal(err)
	}
	for _, leg := range []world.Flight{f1, f2} {
		st := s.Tenants[leg.Carrier].Store
		recs, err := st.ListPNRs(ctx, 10000)
		if err != nil {
			t.Fatal(err)
		}
		foundCopy := false
		for _, r := range recs {
			match := false
			for _, p := range r.Passengers {
				if p.Surname == surname {
					match = true
				}
			}
			if !match {
				continue
			}
			for _, sg := range r.Segments {
				if sg.Carrier == leg.Carrier && sg.FlightNum == leg.Number && sg.Status == "HK" {
					foundCopy = true
				}
			}
		}
		if !foundCopy {
			t.Errorf("carrier %s holds no confirmed copy of its leg %s%s for %s; gds record: %v",
				leg.Carrier, leg.Carrier, leg.Number, surname, rec.Segments)
		}
	}
}

// Cancelling a flight's operations must reach every booking on it: each
// settled record touching the flight lands on the schedule-change queue,
// where an agent would rebook it. One lost booking is one stranded party.
func TestInvariantCancelledFlightQueuesEveryBooking(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()

	c := s.Manifest.Carriers[0]
	f := s.Flights[c.Designator][0]

	var locators []string
	for i := 0; i < 3; i++ {
		res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("STRAND%d", i))
		if err != nil {
			t.Fatalf("Book: %v", err)
		}
		locators = append(locators, res.PNR.RecordLocator)
	}
	settleAll(t, s, locators)

	if err := s.chaos("close", f.From); err != nil {
		t.Fatalf("close %s: %v", f.From, err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		items, err := s.GDSStore.ListQueue(ctx, store.QueueFilter{Queue: store.QueueScheduleChange})
		if err != nil {
			t.Fatal(err)
		}
		queued := map[string]bool{}
		for _, it := range items {
			queued[it.Locator] = true
		}
		missing := 0
		for _, l := range locators {
			if !queued[l] {
				missing++
			}
		}
		if missing == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d bookings on the cancelled flight never reached the queue (%d items)",
				missing, len(locators), len(items))
		}
		time.Sleep(100 * time.Millisecond)
	}
}
