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
