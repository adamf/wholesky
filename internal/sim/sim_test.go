package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/world"
)

// smallWorld compiles a real world from the vendored data and trims it to a
// handful of carriers, forcing at least one onto each wire format so the test
// cannot quietly become a Type B-only test again.
func smallWorld(t *testing.T) *world.Manifest {
	t.Helper()
	m, err := world.Compile(world.CompileOptions{
		DataDir: "../../data", Seed: 1,
		Countries:   []string{"United Kingdom", "France", "Germany"},
		MaxCarriers: 4,
	})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(m.Carriers) < 2 {
		t.Fatalf("the compiled world holds %d carriers; the vendored data is broken", len(m.Carriers))
	}
	m.Carriers[0].Format = "typeb"
	m.Carriers[1].Format = "edifact"
	return m
}

// The wholesky boot test: a compiled world, the real topology on real
// sockets, bookings settled in both dialects, and a flight day that reaches
// the watcher. This is the claim the README makes, run on every go test.
func TestBootBookAndFly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	defer s.Stop()

	// One booking per carrier, which covers both wire formats by
	// construction. Each sell crosses the switch, is answered by the
	// tenant's inventory, and the reply crosses back.
	for i, c := range m.Carriers {
		fs := s.Flights[c.Designator]
		if len(fs) == 0 {
			continue
		}
		res, err := s.Book(ctx, fs[0], "Y", 0, fmt.Sprintf("BOOT%02d", i))
		if err != nil {
			t.Fatalf("book with %s (%s): %v", c.Designator, c.Format, err)
		}
		loc := res.PNR.RecordLocator
		deadline := time.Now().Add(15 * time.Second)
		for {
			ok, err := s.Settled(ctx, loc)
			if err != nil {
				t.Fatalf("settled %s: %v", loc, err)
			}
			if ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("booking %s with %s (%s) never settled: the %s reply did not "+
					"make it back through the switch", loc, c.Designator, c.Format, c.Format)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}

	// The flight day: a departure emitted by a tenant must cross the switch
	// and register at the watcher as a movement event.
	var tenant string
	for code, fs := range s.Flights {
		if len(fs) > 0 {
			tenant = code
			break
		}
	}
	before := s.Movements.Load()
	f := s.Flights[tenant][0]
	if err := s.Tenants[tenant].Depart(ctx, f, time.Now().UTC().Truncate(24*time.Hour), "SKY001", 0); err != nil {
		t.Fatalf("depart: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for s.Movements.Load() == before {
		if time.Now().After(deadline) {
			t.Fatalf("a departure was emitted and no movement event arrived at the watcher")
		}
		time.Sleep(25 * time.Millisecond)
	}

	// And the Eye saw it: the movement message became an aircraft on the map.
	mux := http.NewServeMux()
	s.Eye.Routes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/eye/planes.json", nil))
	var planes []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &planes); err != nil {
		t.Fatalf("planes.json: %v", err)
	}
	found := false
	for _, p := range planes {
		if p["from"] == f.From && p["to"] == f.To {
			found = true
		}
	}
	if !found {
		t.Fatalf("the departure of %s%s never became a plane; the Eye holds %v",
			f.Carrier, f.Number, planes)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/eye/world.json", nil))
	var w map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &w); err != nil || len(w["airports"].([]any)) == 0 {
		t.Fatalf("world.json is not a map anyone could draw: err=%v", err)
	}
}

// The compiler is deterministic: same data, same seed, same world.
func TestCompileIsDeterministic(t *testing.T) {
	opts := world.CompileOptions{
		DataDir: "../../data", Seed: 7,
		Countries: []string{"United Kingdom"}, MaxCarriers: 6,
	}
	a, err := world.Compile(opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := world.Compile(opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.Stats() != b.Stats() {
		t.Fatalf("two compiles of the same world differ: %q vs %q", a.Stats(), b.Stats())
	}
	if len(a.Flights) != len(b.Flights) {
		t.Fatalf("flight counts differ")
	}
	for i := range a.Flights {
		if a.Flights[i] != b.Flights[i] {
			t.Fatalf("flight %d differs: %+v vs %+v", i, a.Flights[i], b.Flights[i])
		}
	}
}

// The chaos path, end to end: closing an airport makes the operating carriers
// send real ASM cancellations, the GDS ingests them through applySchedule,
// and every booking it holds on an affected flight lands on the
// schedule-change queue -- which is what the map's halo counts.
func TestChaosCloseAirportCascades(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	// A booking to be disrupted, settled first so the record is live.
	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]
	res, err := s.Book(ctx, f, "Y", 0, "CHAOS1")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if ok, _ := s.Settled(ctx, res.PNR.RecordLocator); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the booking never settled")
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := s.chaos("close", f.From); err != nil {
		t.Fatalf("close %s: %v", f.From, err)
	}

	// The cascade must reach the queue: at least the booking above.
	deadline = time.Now().Add(20 * time.Second)
	for {
		items, err := s.GDSStore.ListQueue(ctx, store.QueueFilter{Queue: store.QueueScheduleChange})
		if err != nil {
			t.Fatalf("ListQueue: %v", err)
		}
		found := false
		for _, it := range items {
			if it.Locator == res.PNR.RecordLocator {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("closing %s cancelled its flights but the booking on %s%s never "+
				"reached the schedule-change queue (%d items)", f.From, f.Carrier, f.Number, len(items))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// And the demand and flight day must refuse the closed airport.
	if !s.isClosed(f.From, "XXX") {
		t.Error("the airport does not read as closed")
	}
	if err := s.chaos("reopen", f.From); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if s.isClosed(f.From, "XXX") {
		t.Error("the airport is still closed after reopening")
	}
}
