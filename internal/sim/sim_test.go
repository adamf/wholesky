package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// The first carrier also dials in over MATIP, so every suite that boots
	// this world exercises the airline transport end to end.
	m.Carriers[0].Transport = "matip"
	m.Carriers[1].Format = "edifact"
	m.Carriers[1].Transport = "tcp"
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

	// Every embedded node serves its own full console under /node/{code}/,
	// with the prefix stripped so the page's relative paths resolve.
	nmux := http.NewServeMux()
	nmux.HandleFunc("/node/", s.serveNodeConsole)
	first := m.Carriers[0].Designator
	for _, path := range []string{"/node/" + first + "/", "/node/1G/"} {
		rec := httptest.NewRecorder()
		nmux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Jetway") {
			t.Fatalf("%s did not serve a console: %d", path, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	nmux.ServeHTTP(rec, httptest.NewRequest("GET", "/node/"+first+"/api/status", nil))
	var st map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("a tenant console's api/status did not answer: %v", err)
	}
	if id, _ := st["identity"].(map[string]any); id == nil || id["designator"] != first {
		t.Fatalf("the console at /node/%s/ answers as %v; each node must wear its own identity", first, st["identity"])
	}

	// The logical web must be derivable from the wire: within an AVS cycle
	// the Eye should hold carrier-to-GDS conversations and -- because
	// availability goes to interline partners too -- at least one
	// carrier-to-carrier edge. The switch never appears: it is plumbing.
	deadline = time.Now().Add(20 * time.Second)
	for {
		edges := s.Eye.LogicalEdges()
		toGDS, c2c, hasSwitch := false, false, false
		for k := range edges {
			src, dst, _ := strings.Cut(k, ">")
			if src == "1X" || dst == "1X" {
				hasSwitch = true
			}
			if dst == "1G" && src != "1X" {
				toGDS = true
			}
			if src != "1G" && dst != "1G" && src != "1X" && dst != "1X" {
				c2c = true
			}
		}
		if hasSwitch {
			t.Fatalf("the logical web contains the switch; it must show conversations, not plumbing: %v", edges)
		}
		if toGDS && c2c {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the logical web never formed (toGDS=%v carrier-to-carrier=%v): %v", toGDS, c2c, edges)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// And the instrument panel serves series a chart could draw.
	smux := http.NewServeMux()
	s.Stats.Routes(smux)
	rec = httptest.NewRecorder()
	smux.ServeHTTP(rec, httptest.NewRequest("GET", "/stats/data.json", nil))
	var sd map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &sd); err != nil || sd["series"] == nil {
		t.Fatalf("stats/data.json unusable: %v", err)
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

// A closed airport turns away what is already flying toward it: the operating
// carrier sends a real DIV naming the alternate, the watcher reroutes the
// aircraft when the message arrives, and nothing lands where nothing can.
func TestChaosDivertsAirborneAircraft(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	// Put an aircraft in the air toward a destination we will close.
	var code string
	for c := range s.Tenants {
		if len(s.Flights[c]) > 0 {
			code = c
			break
		}
	}
	f := s.Flights[code][0]
	day := time.Now().UTC().Truncate(24 * time.Hour)
	if err := s.Tenants[code].Depart(ctx, f, day, "SKY777", 0); err != nil {
		t.Fatalf("depart: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for len(s.Eye.PlanesTo(f.To)) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the departure never became an airborne aircraft")
		}
		time.Sleep(25 * time.Millisecond)
	}

	if err := s.chaos("close", f.To); err != nil {
		t.Fatalf("close %s: %v", f.To, err)
	}

	// The aircraft must leave the closed destination for a real alternate,
	// marked diverted -- and only because the DIV crossed the wire.
	deadline = time.Now().Add(15 * time.Second)
	for {
		var diverted *Planeish
		for _, p := range planesOf(s) {
			if p.Flight == f.Carrier+trimZeros(f.Number) && p.Diverted {
				diverted = &p
			}
		}
		if diverted != nil {
			if diverted.To == f.To {
				t.Fatalf("the aircraft is marked diverted but still bound for the closed %s", f.To)
			}
			if _, ok := s.airports[diverted.To]; !ok {
				t.Fatalf("diverted to %q, which is not an airport in this world", diverted.To)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the aircraft bound for %s was never diverted", f.To)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// And the wire carries the evidence: a DIV at the watcher.
	msgs, err := s.GDSStore.ListMessages(ctx, store.MessageFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	for _, mm := range msgs {
		if strings.HasPrefix(mm.Kind, "DIV/") {
			return
		}
	}
	t.Fatal("the aircraft turned but no DIV message reached the watcher; the Eye must not know things the wire did not carry")
}

// Planeish and helpers keep the test readable without exporting Eye internals.
type Planeish = struct {
	Flight   string
	To       string
	Diverted bool
}

func planesOf(s *Sim) []Planeish {
	var out []Planeish
	for _, apt := range s.Manifest.Airports {
		for _, p := range s.Eye.PlanesTo(apt.IATA) {
			out = append(out, Planeish{Flight: p.Flight, To: p.To, Diverted: p.Diverted})
		}
	}
	return out
}

func trimZeros(n string) string { return strings.TrimLeft(n, "0") }

// Chaos can cut one carrier's circuit and the world degrades exactly as a
// real network does: the switch loses the session, sells routed at the dark
// carrier go undeliverable instead of settling, and a restore brings the
// same carrier back through its own reconnect path -- after which bookings
// settle again. The dashboards' link dots and the undeliverable series are
// fed by what this test asserts from the stores.
func TestChaosSeverAndRestoreLink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]

	livePeer := func(code string) bool {
		for _, p := range s.Switch.LivePeers() {
			if p == code {
				return true
			}
		}
		return false
	}
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	if !livePeer(c.Designator) {
		t.Fatalf("%s is not on the switch before the test starts", c.Designator)
	}

	if err := s.LinkControl(c.Designator, "sever"); err != nil {
		t.Fatalf("sever: %v", err)
	}
	waitFor("the switch to lose the session", func() bool { return !livePeer(c.Designator) })
	if !s.Tenants[c.Designator].Severed() {
		t.Error("the tenant does not read as severed")
	}

	// A sell routed at the dark carrier must not settle; the switch's copy
	// dies on the wire and says so.
	res, err := s.Book(ctx, f, "Y", 0, "DARKLINK")
	if err != nil {
		t.Fatalf("Book during sever: %v", err)
	}
	time.Sleep(2 * time.Second)
	if ok, _ := s.Settled(ctx, res.PNR.RecordLocator); ok {
		t.Error("a booking settled while the carrier's link was severed")
	}
	waitFor("an undeliverable message at the switch", func() bool {
		msgs, err := s.Switch.Store.ListMessages(ctx,
			store.MessageFilter{Status: store.StatusUndeliverable, Limit: 5})
		return err == nil && len(msgs) > 0
	})

	if err := s.LinkControl(c.Designator, "restore"); err != nil {
		t.Fatalf("restore: %v", err)
	}
	waitFor("the carrier to dial back in", func() bool { return livePeer(c.Designator) })

	// After the repair the same carrier answers again.
	res2, err := s.Book(ctx, f, "Y", 1, "REPAIRED")
	if err != nil {
		t.Fatalf("Book after restore: %v", err)
	}
	waitFor("the post-restore booking to settle", func() bool {
		ok, _ := s.Settled(ctx, res2.PNR.RecordLocator)
		return ok
	})

	if err := s.LinkControl("ZZ", "sever"); err == nil {
		t.Error("severing an unknown carrier did not error")
	}
	if err := s.LinkControl(c.Designator, "detonate"); err == nil {
		t.Error("an unknown action did not error")
	}
}

// The delay model is deterministic noise with the familiar shape of a
// network's day: the same flight on the same day is always exactly as late,
// most flights go out on time, and nothing exceeds the four-hour tail.
func TestDelaysAreDeterministicAndShaped(t *testing.T) {
	m := smallWorld(t)
	day := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	onTime, total, worst := 0, 0, 0
	for _, f := range m.Flights {
		d1, a1 := delayFor(f, day)
		d2, a2 := delayFor(f, day)
		if d1 != d2 || a1 != a2 {
			t.Fatalf("%s%s delays twice differently: %d/%d then %d/%d",
				f.Carrier, f.Number, d1, a1, d2, a2)
		}
		if a1 > d1 {
			t.Errorf("%s%s arrives later than it departed late: dep+%d arr+%d",
				f.Carrier, f.Number, d1, a1)
		}
		total++
		if d1 == 0 {
			onTime++
		}
		if d1 > worst {
			worst = d1
		}
	}
	if total == 0 {
		t.Fatal("no flights")
	}
	if float64(onTime)/float64(total) < 0.45 {
		t.Errorf("only %d of %d flights on time; the model is a strike, not a day", onTime, total)
	}
	if worst > 240 {
		t.Errorf("worst delay is %d minutes; the tail is capped at four hours", worst)
	}
	if worst == 0 {
		t.Error("nothing is ever late; the model is a fantasy, not a day")
	}
	// A different day rolls different dice.
	changed := false
	other := day.AddDate(0, 0, 1)
	for _, f := range m.Flights {
		d1, _ := delayFor(f, day)
		d2, _ := delayFor(f, other)
		if d1 != d2 {
			changed = true
			break
		}
	}
	if !changed {
		t.Error("every flight is exactly as late tomorrow; the day is not in the dice")
	}
}
