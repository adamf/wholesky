package sim

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/growth"
	"github.com/adamf/wholesky/internal/world"
)

// A carrier's day is closed into its book when it ends: what it earned
// and spent goes to its cash, and the book says how the carrier is
// doing in the units an airline counts in.
func TestADayIsClosedIntoTheBook(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s, err := Boot(ctx, smallWorld(t), Options{GDSCount: 1, Fill: 0.8, FillSeed: 3, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.fillDay(ctx)
	s.rebuildLedger(ctx)
	// The end of the day: everything has flown.
	s.clock.Sync(time.Now(), 23*60+59, s.clock.Warp())
	w := seatWorld{s}
	var code string
	for c := range s.Tenants {
		if sc := w.score(c); sc.Flown > 10 && (code == "" || c < code) {
			code = c
		}
	}
	if code == "" {
		t.Fatal("no carrier flew anything")
	}
	v, ok := w.Finance(code)
	if !ok {
		t.Fatal("no book")
	}
	before := v.(financeView)
	if before.Cash != before.Opening || before.Opening <= 0 || len(before.Days) != 0 {
		t.Fatalf("a new book: %+v", before)
	}
	sc := w.score(code)
	s.closeDay()
	v, _ = w.Finance(code)
	after := v.(financeView)
	if len(after.Days) != 1 {
		t.Fatalf("%d days in the book", len(after.Days))
	}
	d := after.Days[0]
	t.Logf("%s: revenue %d, cost %d, profit %d; %d flown, load factor %.2f, RASK %.1f, CASK %.1f cents; opening cash %d", code,
		d.Revenue/100, d.Cost/100, d.Profit/100, d.Flown, after.All.LoadFactor, after.All.RASK, after.All.CASK, after.Opening/100)
	// The fleet's rent runs with the clock, so two readings a moment
	// apart differ by cents.
	about := func(a, b int64) bool { return a-b < 1000 && b-a < 1000 }
	if d.Revenue != sc.Revenue || !about(d.Cost, sc.Cost) || d.Profit != d.Revenue-d.Cost || d.Flown != sc.Flown || after.Cash != after.Opening+d.Profit {
		t.Errorf("the day in the book %+v is not the scorecard's %d and %d, or the cash %d is not the opening %d and the profit", d, sc.Revenue, sc.Cost, after.Cash, after.Opening)
	}
	if after.All.LoadFactor < 0.5 || after.All.LoadFactor > 0.98 || after.All.RASK <= 0 || after.All.CASK <= 0 || d.ASK <= d.RPK {
		t.Errorf("the units: %+v", after.All)
	}
	// The opening cash is 18% of a year at about this day's revenue.
	if share := float64(after.Opening) / (float64(d.Revenue) * 365); share < 0.10 || share > 0.30 {
		t.Errorf("opening cash is %.2f of a year of this day's revenue", share)
	}
	// The same day closed again, after a restart say, counts once.
	s.closeDay()
	v, _ = w.Finance(code)
	again := v.(financeView)
	if len(again.Days) != 1 || !about(again.Cash, after.Cash) {
		t.Errorf("the day closed twice: %d days, cash %d then %d", len(again.Days), after.Cash, again.Cash)
	}
	// And the book is in the state.
	st := s.snapshot()
	if st.Economy == nil || st.Economy.Books[code] == nil || st.Economy.Books[code].Cash != again.Cash || len(st.Economy.Books[code].Days) != 1 {
		t.Error("the book is not in the state")
	}
}

// A carrier asks what a service would do, orders it, and flies it from
// the first start of the world after it comes due.
func TestAServiceIsProjectedOrderedAndFlown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: log, AllowPrivatePeers: true, StateFile: path, LinkSecret: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	w := seatWorld{s}
	// A carrier and a market from its hub that another carrier flies too:
	// the service adds flights where there is somebody to take
	// travellers from.
	var code, from, to string
	for _, c := range m.Carriers {
		for _, f := range s.Flights[c.Designator] {
			if f.From == c.Hub && f.KM > 300 && len(s.rivalsIn(f.From, f.To)) >= 2 {
				code, from, to = c.Designator, f.From, f.To
				break
			}
		}
		if code != "" {
			break
		}
	}
	if code == "" {
		t.Skip("no carrier in the small world shares a market from its hub")
	}
	v, err := w.Growth(code, from, to, "320", 3)
	if err != nil {
		t.Fatal(err)
	}
	proj := v.(map[string]any)["projection"].(growth.Projection)
	t.Logf("%s %s-%s, 3 daily A320: market %.0f a day to %.0f, share %.2f, load %.2f at %d cents, contribution %d a day, break-even %.2f, start %d, payback %d days", code, from, to,
		proj.MarketNow, proj.MarketAfter, proj.Share, proj.LoadFactor, proj.Fare, proj.Contribution/100, proj.BreakEven, proj.Start/100, proj.Payback)
	if proj.Flights != 6 || proj.Seats != 6*180 || proj.Aircraft < 1 || proj.Fare <= 0 || proj.Share <= 0 || proj.Share > 1 || proj.Cost <= 0 || proj.Start <= 0 || len(proj.Notes) == 0 {
		t.Fatalf("projection %+v", proj)
	}
	for _, line := range []string{"fuel", "maintenance", "airports", "navigation", "ground handling", "crew", "aircraft rent", "selling"} {
		if proj.Lines[line] <= 0 {
			t.Errorf("no %s in the projection's cost: %v", line, proj.Lines)
		}
	}
	// A seat-kilometre of the service costs what a seat-kilometre of an
	// airline costs: 4 to 14 US cents.
	if cask := float64(proj.Cost) / (float64(proj.Seats) * float64(s.tariff.Distance(from, to))); cask < 4 || cask > 14 {
		t.Errorf("the service costs %.1f cents a seat-kilometre", cask)
	}
	for _, bad := range [][3]string{{"ZZZ", to, "320"}, {from, from, "320"}, {from, to, "XXX"}} {
		if _, err := w.Growth(code, bad[0], bad[1], bad[2], 3); err == nil {
			t.Errorf("a service %v was projected", bad)
		}
	}

	// The order: the cash to start leaves the book, and a carrier cannot
	// order what it cannot pay to start.
	s.econ.mu.Lock()
	cash := s.bookOf(code).Cash
	s.bookOf(code).Cash = proj.Start - 1
	s.econ.mu.Unlock()
	if _, err := w.Act(ctx, code, airline.Action{Kind: "grow", From: from, To: to, Policy: "320", Count: 3}); err == nil {
		t.Fatal("an order the carrier could not pay for was placed")
	}
	s.econ.mu.Lock()
	s.bookOf(code).Cash = cash
	s.econ.mu.Unlock()
	said, err := w.Act(ctx, code, airline.Action{Kind: "grow", From: from, To: to, Policy: "320", Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	fv, _ := w.Finance(code)
	book := fv.(financeView)
	if len(book.Orders) != 1 || book.Cash != cash-proj.Start || !strings.Contains(said, "ordered") {
		t.Fatalf("after the order: %s; cash %d (was %d, start %d); orders %+v", said, book.Cash, cash, proj.Start, book.Orders)
	}
	o := book.Orders[0]
	if o.Due != o.Placed+leadDays || o.Aircraft != proj.Aircraft || o.Flying {
		t.Errorf("order %+v", o)
	}
	flightsBefore := len(s.Flights[code])
	// Before it is due a restart changes nothing; after, the service flies.
	early, _ := json.Marshal(s.snapshot())
	if n := GrowManifest(smallWorld(t), early); n != 0 {
		t.Errorf("%d flights added before the order came due", n)
	}
	s.econ.day.Store(int64(o.Due))
	if err := s.writeState(); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if b, err := os.ReadFile(path); err != nil || !strings.Contains(string(b), o.ID) {
		t.Fatalf("the order is not in the state file: %v", err)
	}

	m2 := smallWorld(t)
	s2, err := Boot(ctx, m2, Options{Log: log, AllowPrivatePeers: true, StateFile: path, LinkSecret: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Stop()
	var added []world.Flight
	for _, f := range s2.Flights[code] {
		if strings.HasPrefix(f.Tail, "G"+code) {
			added = append(added, f)
		}
	}
	if len(added) != 6 || len(s2.Flights[code]) != flightsBefore+6 {
		t.Fatalf("%d flights of the order in the schedule, %d in all against %d before", len(added), len(s2.Flights[code]), flightsBefore)
	}
	out, back := 0, 0
	for _, f := range added {
		if f.Equipment != "320" || f.Seats != 180 || f.KM != s.tariff.Distance(from, to) {
			t.Errorf("flight %+v", f)
		}
		if f.From == from && f.To == to {
			out++
		}
		if f.From == to && f.To == from {
			back++
		}
		// The world it was built into knows it: an aircraft in the fleet,
		// a fare in the tariff, a market at the pricing desk.
		if _, ok := s2.econ.fleets[code][f.Tail]; !ok {
			t.Errorf("tail %s is not in the fleet", f.Tail)
		}
		if len(s2.tariff.Fares(code, f.From, f.To)) == 0 {
			t.Errorf("no fare filed for %s-%s", f.From, f.To)
		}
	}
	if out != 3 || back != 3 || len(s2.rivalsIn(from, to)) == 0 {
		t.Errorf("%d out, %d back, rivals %v", out, back, s2.rivalsIn(from, to))
	}
	fv, _ = (seatWorld{s2}).Finance(code)
	if got := fv.(financeView); len(got.Orders) != 1 || !got.Orders[0].Flying || got.Cash != book.Cash {
		t.Errorf("the book after the restart: %+v", got)
	}
}

// A running world takes a schedule change overnight. A carrier orders a
// service of 2 round trips a day. At the end of a day before the service
// is due the world carries on. At the end of the day it comes due the
// world stops and starts again: the carrier has 4 more flights, the
// calendar and the carrier's cash are where they were, and the order is
// marked as flying. The world does not stop again for the same order.
func TestARunningWorldTakesAScheduleChangeOvernight(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	before := len(m.Flights)
	worlds := make(chan *Sim, 4)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, m, Options{Log: log, AllowPrivatePeers: true, LinkSecret: "keep"},
			func(_ context.Context, s *Sim) { worlds <- s })
	}()
	next := func() *Sim {
		select {
		case s := <-worlds:
			return s
		case err := <-done:
			t.Fatalf("the world ended: %v", err)
		case <-ctx.Done():
			t.Fatal("no world started")
		}
		return nil
	}
	s := next()
	var code string
	var f world.Flight
	for _, c := range m.Carriers {
		for _, g := range s.Flights[c.Designator] {
			if g.From == c.Hub && g.KM > 300 {
				code, f = c.Designator, g
				break
			}
		}
		if code != "" {
			break
		}
	}
	if code == "" {
		t.Skip("no carrier in the small world flies 300 km from its hub")
	}
	had := len(s.Flights[code])
	w := seatWorld{s}
	if _, err := w.Act(ctx, code, airline.Action{Kind: "grow", From: f.From, To: f.To, Policy: "320", Count: 2}); err != nil {
		t.Fatal(err)
	}
	day := s.econ.day.Load()

	// The service is weeks from due: the day ends and the world goes on.
	if s.wrapDay(ctx) {
		t.Fatal("the world stopped for a service that is not due")
	}
	select {
	case <-s.Rescheduled():
		t.Fatal("the world says it stopped")
	default:
	}
	// The day before it is due.
	s.econ.mu.Lock()
	due := s.bookOf(code).Orders[0].Due
	s.econ.mu.Unlock()
	s.econ.day.Store(int64(due) - 1)
	if !s.wrapDay(ctx) {
		t.Fatal("the service came due and the world went on without it")
	}
	// What the carrier had when the world stopped, after 2 days closed.
	s.econ.mu.Lock()
	cash := s.bookOf(code).Cash
	s.econ.mu.Unlock()
	s2 := next()
	if s2 == s {
		t.Fatal("the same world came back")
	}
	if got := len(s2.Flights[code]); got != had+4 {
		t.Errorf("%s flew %d flights and now flies %d; 2 round trips are 4 more", code, had, got)
	}
	added := 0
	for _, g := range s2.Flights[code] {
		if (g.From == f.From && g.To == f.To) || (g.From == f.To && g.To == f.From) {
			added++
		}
	}
	if added < 4 {
		t.Errorf("%d flights between %s and %s after the order", added, f.From, f.To)
	}
	if got := s2.econ.day.Load(); got != int64(due) || got <= day {
		t.Errorf("the calendar is at day %d; the service was due on day %d", got, due)
	}
	s2.econ.mu.Lock()
	b := s2.bookOf(code)
	flying, cash2 := len(b.Orders) == 1 && b.Orders[0].Flying, b.Cash
	s2.econ.mu.Unlock()
	if !flying {
		t.Error("the order is not marked as flying in the new world")
	}
	if cash2 != cash {
		t.Errorf("the carrier's cash was %d when the world stopped and is %d", cash, cash2)
	}
	if len(m.Flights) != before {
		t.Errorf("the manifest the world came from had %d flights and has %d", before, len(m.Flights))
	}
	// The next day ends as any day does.
	if s2.wrapDay(ctx) {
		t.Error("the world stopped again for a service it already flies")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the world ended with %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Error("the world did not end with its context")
	}
}

// An incumbent answers an entrant with flights. A rival begins 2 round
// trips a day on a route that a carrier flies from its hub. The rival is
// losing money and has cash for a few days of it, so the carrier orders
// half as many flights again as it flies there, and no more than 4. It
// answers once. It does not answer a rival with the cash to last, and a
// carrier whose network planning is in a seat's hands is left to decide.
func TestAnIncumbentAnswersAWeakEntrant(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: log, AllowPrivatePeers: true, LinkSecret: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	// An incumbent, a route from its hub that it flies, and the flights
	// it would add: they must earn what they cost to fly.
	var inc, hub, far, typ string
	own, add := 0, 0
	for _, c := range m.Carriers {
		routes := map[string]int{}
		types := map[string]string{}
		for _, f := range s.Flights[c.Designator] {
			if f.From == c.Hub && f.KM > 300 {
				routes[f.To]++
				types[f.To] = f.Equipment
			}
		}
		for to, n := range routes {
			want := growth.Answer(n, growth.Entrant{Daily: 2, Aircraft: 1})
			p, mk, costs, err := s.proposal(c.Designator, c.Hub, to, types[to], want)
			if err == nil && growth.Project(p, mk, costs).Covers() && (inc == "" || to < far) {
				inc, hub, far, typ, own, add = c.Designator, c.Hub, to, types[to], n, want
			}
		}
		if inc != "" {
			break
		}
	}
	if inc == "" {
		t.Skip("no route in the small world would cover the cost of more flights")
	}
	var rival string
	for _, c := range m.Carriers {
		if c.Designator != inc && s.Tenants[c.Designator] != nil && c.Hub != hub && c.Hub != far {
			rival = c.Designator
			break
		}
	}
	if rival == "" {
		t.Skip("no second carrier in the small world")
	}
	day := int(s.econ.day.Load())
	enter := func(id string, cash int64) {
		s.econ.mu.Lock()
		b := s.bookOf(rival)
		b.Days, b.Cash = nil, cash
		b.Orders = append(b.Orders, growth.Order{ID: id, From: far, To: hub, Type: "320", Daily: 2, Aircraft: 1, Placed: day - leadDays, Due: day, Flying: true})
		s.econ.mu.Unlock()
	}
	answers := func(code string) []growth.Order {
		s.econ.mu.Lock()
		defer s.econ.mu.Unlock()
		var out []growth.Order
		for _, o := range s.bookOf(code).Orders {
			if o.Answers != "" {
				out = append(out, o)
			}
		}
		return out
	}
	// A rival with 10,000 dollars: the day it closes with no flights
	// flown is a loss, and it cannot pay for a second.
	enter("R-1", 10_000_00)
	s.closeDay()
	got := answers(inc)
	if len(got) != 1 {
		t.Fatalf("%s flies %d a day %s-%s and a weak rival began 2: its answers are %+v", inc, own, hub, far, got)
	}
	o := got[0]
	if o.Answers != "R-1" || o.From != hub || o.To != far || o.Daily != add || o.Type != typ || o.Due != day+leadDays || o.Paid <= 0 || o.Flying {
		t.Errorf("the answer %+v; want %d more a day %s-%s on %s, due on day %d", o, add, hub, far, typ, day+leadDays)
	}
	if add != min((own+1)/2, 4) {
		t.Errorf("%d a day answered with %d", own, add)
	}
	// The next day it has answered already.
	s.closeDay()
	if got := answers(inc); len(got) != 1 {
		t.Errorf("a second day, and the answers are %+v", got)
	}
	// An answer is not an entry: the rival does not add flights against
	// the incumbent's answer.
	if got := answers(rival); len(got) != 0 {
		t.Errorf("the rival answered the answer: %+v", got)
	}
	// A rival with the cash to last: no flights are added against it,
	// unless its fleet is small.
	s.econ.mu.Lock()
	fleet := len(s.econ.fleets[rival])
	s.econ.mu.Unlock()
	t.Logf("%s flies %d a day %s-%s on the %s and adds %d; the rival %s has %d aircraft", inc, own, hub, far, typ, add, rival, fleet)
	enter("R-2", 1_000_000_000_000_00)
	s.closeDay()
	if got := answers(inc); fleet >= 10 && len(got) != 1 {
		t.Errorf("a rival with %d aircraft and cash to last was answered: %+v", fleet, got)
	}
	// A seat that plans its own network is not answered for.
	_, token, err := s.Airline.Take(inc, "planner")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Airline.SetManual(inc, token, "network", true); err != nil {
		t.Fatal(err)
	}
	enter("R-3", 10_000_00)
	s.closeDay()
	for _, o := range answers(inc) {
		if o.Answers == "R-3" {
			t.Errorf("network planning is in a seat's hands and the carrier answered for it: %+v", o)
		}
	}
}
