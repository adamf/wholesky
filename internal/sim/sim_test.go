package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/ats"
	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/host"

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
		// Worked or pending: the irops engine may already have moved them.
		items, err := s.GDSStore.ListQueue(ctx, store.QueueFilter{Queue: store.QueueScheduleChange, IncludeWorked: true})
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

// The ground story of a departure: the PNL lists what the carrier's own
// store believes it is boarding, the ADL carries exactly the diff after a
// cancellation, and the bag messages tag real parties. All of it crosses
// the switch to the watcher like any other traffic.
func TestGroundStoryFromNameListToLoadsheet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]
	day := s.BookingDate

	var locs []string
	for i := 0; i < 12; i++ {
		res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("GROUND%d", i))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	deadline := time.Now().Add(20 * time.Second)
	for _, l := range locs {
		for {
			if ok, _ := s.Settled(ctx, l); ok {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s never settled", l)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}

	tn := s.Tenants[c.Designator]
	// findAt looks for an inbound message of a kind in a store: the GDS
	// watcher's for what operations sees, the tenant's own for what came
	// back down its circuit to its airport desks.
	findAt := func(st store.Store, kind string) *store.Message {
		msgs, _ := st.ListMessages(ctx, store.MessageFilter{Limit: 10000})
		for _, msg := range msgs {
			if strings.HasPrefix(msg.Kind, kind) && msg.Direction == store.Inbound {
				full, err := st.GetMessage(ctx, msg.ID)
				if err == nil {
					return full
				}
			}
		}
		return nil
	}
	waitAt := func(st store.Store, kind string) *store.Message {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			if m := findAt(st, kind); m != nil {
				return m
			}
			if time.Now().After(deadline) {
				t.Fatalf("no %s message ever arrived", kind)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	waitDCS := func(cond func(*dcs.Flight) bool, what string) *dcs.Flight {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			if fl, err := tn.DCS.Flight(key); err == nil && cond(fl) {
				return fl
			}
			if time.Now().After(deadline) {
				fl, _ := tn.DCS.Flight(key)
				t.Fatalf("departure control never reached %s: %+v", what, fl)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Reservations sends the list to the airport; the switch carries it
	// back down the carrier's own circuit to its check-in address, and the
	// operations watch sees a copy. Departure control opens the flight.
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatalf("SendPNL: %v", err)
	}
	pnlMsg := waitAt(s.GDSStore, "PNL/")
	text := string(pnlMsg.Raw)
	for i := 0; i < 12; i++ {
		if !strings.Contains(text, fmt.Sprintf("GROUND%d", i)) {
			t.Errorf("the PNL is missing GROUND%d:\n%s", i, text)
		}
	}
	if !strings.Contains(text, f.From+"KP"+c.Designator) {
		t.Errorf("the PNL is not addressed to the carrier's check-in at %s:\n%s", f.From, text)
	}
	fl := waitDCS(func(fl *dcs.Flight) bool { return fl.Counts().Listed == 12 }, "twelve listed")
	if fl.Equipment != f.Equipment || fl.Dest != f.To {
		t.Errorf("the flight opened with the wrong aircraft or destination: %+v", fl.Key)
	}

	// One traveller cancels; the ADL must carry exactly that, and departure
	// control must drop them from the list.
	if _, err := s.GDS.Cancel(ctx, locs[0], gateway.CancelOptions{By: "test"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for {
		recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, key.Date, 100)
		if len(recs) == 11 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the carrier still holds %d records after the cancellation", len(recs))
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := tn.SendADL(ctx, f, day); err != nil {
		t.Fatalf("SendADL: %v", err)
	}
	adl := string(waitAt(s.GDSStore, "ADL/").Raw)
	if !strings.Contains(adl, "DEL") || !strings.Contains(adl, "GROUND0") {
		t.Errorf("the ADL does not delete the cancelled party:\n%s", adl)
	}
	if strings.Contains(adl, "ADD") {
		t.Errorf("the ADL invents an addition nobody made:\n%s", adl)
	}
	waitDCS(func(fl *dcs.Flight) bool { return fl.Counts().Listed == 11 }, "eleven listed after the ADL")

	// The counter: everyone who is going to turn up has, by the last wave.
	// Their bags go to the sortation system as BSMs, down the same circuit.
	if err := tn.CheckIn(ctx, f, day, 46); err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	fl, _ = tn.DCS.Flight(key)
	cnt := fl.Counts()
	if cnt.Accepted == 0 || cnt.Accepted+cnt.Listed != 11 {
		t.Fatalf("after the last wave: %+v", cnt)
	}
	if cnt.Bags > 0 {
		bsm := waitAt(tn.Store, "BSM/")
		if !strings.Contains(string(bsm.Raw), ".N/0") || !strings.Contains(string(bsm.Raw), f.From+"KB"+c.Designator) {
			t.Errorf("the BSM carries no licence plate or went to the wrong desk:\n%s", bsm.Raw)
		}
	}
	if err := tn.CloseCheckIn(ctx, f, day); err != nil {
		t.Fatalf("CloseCheckIn: %v", err)
	}
	if err := tn.Board(ctx, f, day, 14); err != nil {
		t.Fatalf("Board: %v", err)
	}
	if err := tn.ReportBags(ctx, f, day); err != nil {
		t.Fatalf("ReportBags: %v", err)
	}
	if err := tn.Close(ctx, f, day); err != nil {
		t.Fatalf("Close: %v", err)
	}
	fl, _ = tn.DCS.Flight(key)
	if fl.State != dcs.StateClosed || fl.Loadsheet == "" || fl.Load == nil {
		t.Fatalf("the flight did not close with a loadsheet: %s", fl.State)
	}
	cnt = fl.Counts()
	if cnt.Boarded == 0 || cnt.Boarded+cnt.NoShow+cnt.Offload != 11 {
		t.Errorf("close accounts for %+v of eleven", cnt)
	}
	if cnt.Bags > 0 {
		for _, p := range fl.Passengers {
			for _, b := range p.Bags {
				if p.Status == dcs.StatusBoarded && !b.Loaded {
					t.Errorf("the sortation system never reported %s loaded", b.Tag)
				}
			}
		}
	}

	// The closure's messages reach their desks: final sales home to
	// reservations, the load to the arrival station and the watch.
	pfs := waitAt(tn.Store, "PFS/")
	if !strings.Contains(string(pfs.Raw), "ENDPFS") {
		t.Errorf("PFS:\n%s", pfs.Raw)
	}
	ldm := waitAt(s.GDSStore, "LDM/")
	if !strings.Contains(string(ldm.Raw), f.To+"KL"+c.Designator) || !strings.Contains(string(ldm.Raw), fmt.Sprintf(".PAX/%d", cnt.Boarded)) {
		t.Errorf("the LDM did not report %d boarded to the arrival station:\n%s", cnt.Boarded, ldm.Raw)
	}
	// A no-show is written back onto the booking, which is the point of
	// telling reservations at all.
	if cnt.NoShow > 0 {
		deadline = time.Now().Add(10 * time.Second)
		for {
			marked := 0
			recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, key.Date, 100)
			for _, r := range recs {
				for _, rm := range r.Remarks {
					if strings.HasPrefix(rm.Text, "NOSHO") {
						marked++
					}
				}
			}
			if marked == cnt.NoShow {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%d no-shows, %d records marked", cnt.NoShow, marked)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// The movement message carries the boarded count, not a guess.
	if err := tn.Depart(ctx, f, day, "SKY001", 0); err != nil {
		t.Fatalf("Depart: %v", err)
	}
	mvt := waitAt(s.GDSStore, "MVT/")
	if !strings.Contains(string(mvt.Raw), fmt.Sprintf("PX%d", cnt.Boarded)) {
		t.Errorf("the MVT does not carry the boarded count %d:\n%s", cnt.Boarded, mvt.Raw)
	}
	// The globe's drill-through sees the same story, and everything else the
	// station holds: the seat map by cabin, the manifest itself, the load
	// the closure produced, and the ops desk's side of the flight.
	sum, ok := tn.Summarise(f.Carrier + strings.TrimLeft(f.Number, "0"))
	if !ok || sum.Counts.Boarded != cnt.Boarded {
		t.Fatalf("Summarise: %+v %v", sum, ok)
	}
	if len(sum.Cabins) == 0 || sum.Cabins[0].Seats == 0 {
		t.Errorf("summary has no seat map: %+v", sum.Cabins)
	}
	// Total counts every name the list ever carried, deleted ones included.
	if sum.Total < cnt.Listed+cnt.Accepted+cnt.Boarded+cnt.Standby+cnt.NoShow+cnt.Offload || len(sum.Passengers) != sum.Total {
		t.Errorf("summary manifest: total %d rows %d counts %+v", sum.Total, len(sum.Passengers), cnt)
	}
	if sum.Load == nil || sum.Loadsheet == "" || sum.ClosedAt == nil {
		t.Errorf("a closed flight's summary should carry the load and loadsheet: load=%v sheet=%d", sum.Load != nil, len(sum.Loadsheet))
	}
	if sum.Ops.Callsign == "" {
		t.Errorf("summary has no callsign: %+v", sum.Ops)
	}
}

// The globe's drill-through: an aircraft resolves to the bookings riding on
// it, straight from whichever channel sold them.
func TestFlightDrillThroughFindsTheBookings(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 2, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]
	res, err := s.GDSes[1].GW.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: "DRILLTHRU", Given: "ANN", Title: "MS"}},
		Segments: []gateway.BookingSegment{{
			Carrier: f.Carrier, FlightNum: f.Number, Class: "Y",
			Date:  strings.ToUpper(s.BookingDate.Format("02Jan")),
			Board: f.From, Off: f.To, Seats: 1}},
		Agent: "test", Channel: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	recs := s.flightRecords(f.Carrier + f.Number)
	found := false
	for _, r := range recs {
		if r.Locator == res.PNR.RecordLocator {
			found = true
			if r.Surname != "DRILLTHRU" || r.GDS != s.GDSes[1].Designator {
				t.Errorf("record listed wrong: %+v", r)
			}
		}
	}
	if !found {
		t.Errorf("the drill-through never found %s among %d records",
			res.PNR.RecordLocator, len(recs))
	}
}

// The clock re-anchors on every warp change, so time never jumps -- it just
// starts passing at the new rate. Zero holds the day exactly where it is.
func TestSimClockRepacesWithoutJumping(t *testing.T) {
	c := newSimClock(60)
	t0 := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	c.SetWarp(t0, 60) // anchor at a known instant
	p0 := c.Pos(t0)

	// One wall minute at warp 60 is one sim hour.
	if got := c.Pos(t0.Add(time.Minute)); math.Abs(got-math.Mod(p0+60, 1440)) > 0.01 {
		t.Errorf("after one minute at warp 60: pos = %.2f, want %.2f", got, math.Mod(p0+60, 1440))
	}

	// Speeding up mid-flight continues from the current position.
	t1 := t0.Add(30 * time.Second)
	before := c.Pos(t1)
	c.SetWarp(t1, 600)
	if got := c.Pos(t1); math.Abs(got-before) > 0.01 {
		t.Fatalf("changing warp jumped the clock: %.2f -> %.2f", before, got)
	}
	if got := c.Pos(t1.Add(time.Minute)); math.Abs(got-math.Mod(before+600, 1440)) > 0.01 {
		t.Errorf("after one minute at warp 600: pos = %.2f, want %.2f", got, math.Mod(before+600, 1440))
	}

	// Pause holds; resume continues from the held position.
	t2 := t1.Add(2 * time.Minute)
	held := c.Pos(t2)
	c.SetWarp(t2, 0)
	if got := c.Pos(t2.Add(time.Hour)); math.Abs(got-held) > 0.01 {
		t.Errorf("an hour into the pause the day moved: %.2f -> %.2f", held, got)
	}
	t3 := t2.Add(2 * time.Hour)
	c.SetWarp(t3, 60)
	if got := c.Pos(t3.Add(time.Minute)); math.Abs(got-math.Mod(held+60, 1440)) > 0.01 {
		t.Errorf("resume did not continue from the held position: %.2f, want %.2f",
			got, math.Mod(held+60, 1440))
	}
}

// The control clamps to sane rates.
func TestSetWarpBounds(t *testing.T) {
	s := &Sim{clock: newSimClock(60), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := s.SetWarp(601); err == nil {
		t.Error("warp 601 accepted")
	}
	if err := s.SetWarp(-1); err == nil {
		t.Error("negative warp accepted")
	}
	if err := s.SetWarp(0); err != nil {
		t.Errorf("pause refused: %v", err)
	}
	if got := s.clock.Warp(); got != 0 {
		t.Errorf("warp = %d after pause", got)
	}
}

// The end of a simulated day clears the tenants' books of record: what was
// booked for it has flown or not, and the next day starts clean.
func TestEndOfDayPurgeClearsTenantBooks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]
	res, err := s.Book(ctx, f, "Y", 0, "PURGEME")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if ok, _ := s.Settled(ctx, res.PNR.RecordLocator); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never settled")
		}
		time.Sleep(25 * time.Millisecond)
	}
	tn := s.Tenants[c.Designator]
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	if recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, wire, 100); len(recs) != 1 {
		t.Fatalf("the carrier holds %d records before the purge", len(recs))
	}
	// A purge cut before the booking leaves it; one cut after takes it.
	s.purgeTenants(ctx, time.Now().Add(-time.Hour))
	if recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, wire, 100); len(recs) != 1 {
		t.Fatalf("a purge cut in the past removed a fresh record")
	}
	s.purgeTenants(ctx, time.Now().Add(time.Second))
	if recs, _ := tn.Store.FindPNRsByFlight(ctx, f.Carrier+f.Number, wire, 100); len(recs) != 0 {
		t.Fatalf("the carrier still holds %d records after the day ended", len(recs))
	}
	// The distribution system's book is its own; the tenant's day ending
	// does not touch it.
	if _, err := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator); err != nil {
		t.Errorf("the GDS lost the record: %v", err)
	}
}

// A peer syncs to the core's clock: position and rate, from now.
func TestSimClockSyncFollowsAnotherClock(t *testing.T) {
	core := newSimClock(6)
	peer := newSimClock(60)
	now := time.Now()
	if math.Abs(core.Pos(now)-peer.Pos(now)) < 1 {
		t.Skip("the two boot anchors happen to agree; nothing to sync")
	}
	peer.Sync(now, core.Pos(now), core.Warp())
	later := now.Add(90 * time.Second)
	if d := math.Abs(core.Pos(later) - peer.Pos(later)); d > 0.01 {
		t.Fatalf("after sync the clocks differ by %.3f sim-minutes", d)
	}
	if peer.Warp() != 6 {
		t.Fatalf("warp %d", peer.Warp())
	}
}

// A cancelled flight's passenger is moved onto the next flight over the
// same city pair: the engine at the distribution system does the desk's
// work, and the record ends up holding a live segment on another flight.
func TestIROPSRebooksOffACancelledFlight(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, AVSInterval: 2 * time.Second, IROPSInterval: 500 * time.Millisecond,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	// A city pair this world flies more than once, so there is somewhere to go.
	var dead, alt world.Flight
	found := false
	for _, fs := range s.Flights {
		for i, a := range fs {
			for _, b := range fs[i+1:] {
				if a.From == b.From && a.To == b.To && a.DepMin < b.DepMin {
					dead, alt, found = a, b, true
				}
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("the small world has no city pair flown twice by one carrier")
	}
	res, err := s.Book(ctx, dead, "Y", 0, "REROUTE")
	if err != nil {
		t.Fatal(err)
	}
	loc := res.PNR.RecordLocator
	deadline := time.Now().Add(20 * time.Second)
	for {
		if ok, _ := s.Settled(ctx, loc); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("never settled")
		}
		time.Sleep(25 * time.Millisecond)
	}
	// The alternative must be on free sale at the GDS before the engine
	// looks, which is what the availability broadcast does.
	altKey := avail.NewKey(alt.Carrier, alt.Number, s.BookingDate, alt.From, alt.To, "Y")
	deadline = time.Now().Add(20 * time.Second)
	for {
		if _, ok, fresh := s.GDS.Avail.Lookup(altKey); ok && fresh {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the GDS never heard availability for %s%s", alt.Carrier, alt.Number)
		}
		time.Sleep(50 * time.Millisecond)
	}

	tn := s.Tenants[dead.Carrier]
	date := strings.ToUpper(s.BookingDate.Format("02Jan"))
	text := fmt.Sprintf("ASM\nUTC\nCNL\n%s%s/%s\n%s %s", dead.Carrier, dead.Number, date, dead.From, dead.To)
	if err := tn.SendSchedule(ctx, text); err != nil {
		t.Fatal(err)
	}
	if err := tn.CancelFlight(ctx, dead, s.BookingDate, "test"); err != nil {
		t.Fatal(err)
	}

	deadline = time.Now().Add(30 * time.Second)
	for {
		rec, err := s.GDSStore.GetPNR(ctx, loc)
		if err != nil {
			t.Fatal(err)
		}
		// Own metal first: the new leg is one of the carrier's own later
		// flights over the pair, whichever the schedule ranked nearest.
		var onAlt, onDead *pnr.Segment
		for i := range rec.Segments {
			seg := &rec.Segments[i]
			switch {
			case seg.FlightNum == dead.Number:
				onDead = seg
			case seg.Carrier == dead.Carrier && seg.Board == dead.From && seg.Off == dead.To && seg.Status == "HK":
				onAlt = seg
			}
		}
		if onAlt != nil && onDead != nil && onDead.Status == "XX" {
			if s.Rebooked.Load() < 1 {
				t.Errorf("rebooked counter %d", s.Rebooked.Load())
			}
			items, _ := s.GDSStore.ListQueue(ctx, store.QueueFilter{Queue: store.QueueScheduleChange, IncludeWorked: true})
			worked := false
			for _, it := range items {
				if it.Locator == loc && !it.Pending() && strings.Contains(it.Note, onAlt.Carrier+onAlt.FlightNum) {
					worked = true
				}
			}
			if !worked {
				t.Errorf("the queue item was not worked with the new flight: %+v", items)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("never rebooked: segments %+v", rec.Segments)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// The aircraft talks and the tower talks: an OOOI report from the datalink
// provider becomes the airline's MVT, the airline's flight plan reaches
// air traffic services, and the tower's DEP reaches the airline.
func TestAircraftReportsAndATSDriveTheDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if s.DSP == nil || s.ANSP == nil {
		t.Fatal("the world has no datalink provider or ANSP")
	}
	c := m.Carriers[0]
	f := s.Flights[c.Designator][0]
	tn := s.Tenants[c.Designator]
	day := s.BookingDate

	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("never saw %s", what)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	// The day is flying, so many MVTs arrive; match the one this flight's
	// aircraft produced by a substring of its raw text.
	inbound := func(st store.Store, kind, contains string) *store.Message {
		msgs, _ := st.ListMessages(ctx, store.MessageFilter{Limit: 5000})
		for _, msg := range msgs {
			if strings.HasPrefix(msg.Kind, kind) && msg.Direction == store.Inbound {
				full, _ := st.GetMessage(ctx, msg.ID)
				if full != nil && (contains == "" || strings.Contains(string(full.Raw), contains)) {
					return full
				}
			}
		}
		return nil
	}

	// Operations files the plan; the ANSP receives it.
	if err := tn.FileFlightPlan(ctx, f, day); err != nil {
		t.Fatalf("FileFlightPlan: %v", err)
	}
	waitFor("the flight plan at the ANSP", func() bool { return s.ANSP.FlightPlansFiled() >= 1 })
	fpl := inbound(s.ANSP.Store, "ATS/FPL/", host.Callsign(c, f))
	if fpl == nil || !strings.Contains(string(fpl.Raw), "(FPL-"+host.Callsign(c, f)) || !strings.Contains(string(fpl.Raw), "ZPZX") {
		t.Errorf("the ANSP holds no readable flight plan: %v", fpl)
	}

	// The aircraft departs: the provider reports, the airline derives the
	// MVT, the watcher sees it with the report's times.
	s.departure(ctx, tn, f, day, "SKY777", 5)
	waitFor("the MVT at the watcher", func() bool { return inbound(s.GDSStore, "MVT/", "SKY777") != nil })
	mvt := string(inbound(s.GDSStore, "MVT/", "SKY777").Raw)
	dep := day.Add(time.Duration(f.DepMin+5) * time.Minute)
	wantAD := "AD" + dep.Format("1504") + "/" + dep.Add(12*time.Minute).Format("1504")
	if !strings.Contains(mvt, wantAD) || !strings.Contains(mvt, "SKY777") {
		t.Errorf("the MVT does not carry the aircraft's OUT/OFF (%s) and registration:\n%s", wantAD, mvt)
	}
	if !strings.Contains(mvt, "DL") {
		t.Errorf("a five-minute delay should be coded:\n%s", mvt)
	}
	if inbound(tn.Store, "ACARS/DEP/", "") == nil {
		t.Error("the airline never received the aircraft's report")
	}
	waitFor("the tower's DEP at the airline", func() bool { return tn.ATSMessages()[ats.TypeDEP] >= 1 })

	// And the landing.
	s.arrival(ctx, tn, f, day, "SKY777", 0)
	waitFor("the arrival MVT", func() bool {
		msgs, _ := s.GDSStore.ListMessages(ctx, store.MessageFilter{Limit: 5000})
		for _, msg := range msgs {
			if strings.HasPrefix(msg.Kind, "MVT/") {
				full, _ := s.GDSStore.GetMessage(ctx, msg.ID)
				if full != nil && strings.Contains(string(full.Raw), "SKY777") && strings.Contains(string(full.Raw), "\nAA") {
					return true
				}
			}
		}
		return false
	})
	waitFor("the tower's ARR at the airline", func() bool { return tn.ATSMessages()[ats.TypeARR] >= 1 })
}
