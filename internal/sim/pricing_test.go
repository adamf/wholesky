package sim

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/partners"
	"github.com/adamf/wholesky/internal/pricing"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/world"
)

// rivalsIn is the carriers that fly a market, largest first.
func (s *Sim) rivalsIn(a, b string) []string {
	if a > b {
		a, b = b, a
	}
	var out []string
	for code := range s.commerce.seats[a+"/"+b] {
		out = append(out, code)
	}
	return out
}

// The other carriers answer a fare change. Before this a rival's "cut"
// was a number the simulator made up for a seat that priced by hand, and
// a carrier on autopilot never moved a fare whatever the others did. A
// carrier that cuts is matched at the next filing by the carriers its
// cut hurts, in that market and both ways; when it goes back they go
// back; and the filed fare is the fare the booking is priced at.
func TestRivalsAnswerAFareChange(t *testing.T) {
	s := bootWorld(t, Options{GDSCount: 1, DecisionWindow: 5 * time.Second})
	ctx := context.Background()
	w := seatWorld{s}
	tick := 0
	file := func() []pricing.Move { tick++; return s.pricingTick(ctx, tick) }
	if first := file(); len(first) != 0 {
		t.Fatalf("the first filing only looks: %+v", first)
	}
	if all, contested := s.commerce.desk.Markets(); all == 0 || contested == 0 {
		t.Skipf("no market in the small world has two carriers: %d markets", all)
	}
	// Cut market after market until a rival answers: a carrier too small
	// to matter is left alone, and one desk in twenty lets a cut go.
	var mv pricing.Move
	var cutter, a, b string
	for key, by := range s.commerce.seats {
		if len(by) < 2 {
			continue
		}
		p := strings.SplitN(key, "/", 2)
		for code := range by {
			if _, err := w.Act(ctx, code, airline.Action{Kind: "fares", From: p[1], To: p[0], Multiplier: 0.8}); err != nil {
				t.Fatal(err)
			}
			for _, m := range file() {
				if m.Kind == pricing.Match && m.Rival == code {
					mv, cutter, a, b = m, code, p[0], p[1]
				}
			}
			if cutter != "" {
				break
			}
			w.Act(ctx, code, airline.Action{Kind: "fares", From: p[0], To: p[1], Multiplier: 1}) //nolint:errcheck
			file()
		}
		if cutter != "" {
			break
		}
	}
	if cutter == "" {
		t.Fatal("no carrier in any contested market answered a 20% cut")
	}
	t.Logf("%s cut %s-%s to 0.80; %s: %s (to %.2f)", cutter, a, b, mv.Carrier, mv.Why, mv.Mult)
	// The cut was filed one way round and holds both ways, as does the
	// answer; the answer goes no lower than the cut.
	for _, c := range []string{cutter, mv.Carrier} {
		if out, back := s.tariff.EffectiveMultiplier(c, a, b), s.tariff.EffectiveMultiplier(c, b, a); out != back || out >= 1 {
			t.Errorf("%s in %s-%s: ×%.3f out, ×%.3f back", c, a, b, out, back)
		}
	}
	if mv.Mult < 0.8-0.001 && s.commerce.desk != nil && mv.Why == "matched "+cutter+"'s cut" {
		t.Errorf("the matcher undercut: ×%.3f", mv.Mult)
	}
	if s.FareMoves.Load() == 0 {
		t.Error("no fare move counted")
	}
	// The booking is priced at the filed fare: the cutter's full fare in
	// the market is four fifths of its filing.
	s.tariff.SetMarketMultiplier(cutter, a, b, 0)
	filed := s.tariff.Fares(cutter, a, b)
	s.setFare(cutter, a, b, 0.8)
	now := s.tariff.Fares(cutter, a, b)
	if len(filed) == 0 || len(now) != len(filed) || math.Abs(float64(now[0].OneWay.Amount)/float64(filed[0].OneWay.Amount)-0.8) > 0.01 {
		t.Errorf("the cut is not in the fares the distribution system prices: %d fares, %d before", len(now), len(filed))
	}
	// The matcher's seat, had there been one, would have read it on the
	// tape; the cutter's does.
	if _, _, err := s.Airline.Take(cutter, "cutter"); err != nil {
		t.Fatal(err)
	}
	// The cutter goes back, and so does everyone who answered.
	if _, err := w.Act(ctx, cutter, airline.Action{Kind: "fares", From: a, To: b, Multiplier: 1}); err != nil {
		t.Fatal(err)
	}
	back := file()
	restored := false
	for _, m := range back {
		if m.Carrier == mv.Carrier && m.Kind == pricing.Restore {
			restored = true
		}
	}
	if !restored || math.Abs(s.tariff.EffectiveMultiplier(mv.Carrier, a, b)-1) > 0.005 {
		t.Errorf("the cutter went back and %s is at ×%.3f after %+v", mv.Carrier, s.tariff.EffectiveMultiplier(mv.Carrier, a, b), back)
	}
	seen := false
	for _, e := range s.Airline.Tape(cutter) {
		if e.Kind == "pricing" && strings.Contains(e.Text, mv.Carrier) {
			seen = true
		}
	}
	if !seen {
		t.Errorf("the cutter's tape does not say %s moved", mv.Carrier)
	}

	// A carrier somebody prices by hand is asked, and holding holds.
	_, token, err := s.Airline.Take(mv.Carrier, "pricer")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Airline.SetManual(mv.Carrier, token, "pricing", true); err != nil {
		t.Fatal(err)
	}
	var asked []airline.Decision
	for try := 0; try < 20 && len(asked) == 0; try++ {
		w.Act(ctx, cutter, airline.Action{Kind: "fares", From: a, To: b, Multiplier: 0.8}) //nolint:errcheck
		proposals := file()
		for wait := time.Now().Add(2 * time.Second); len(asked) == 0 && len(proposals) > 0 && time.Now().Before(wait); {
			time.Sleep(20 * time.Millisecond)
			asked = s.Airline.Inbox(mv.Carrier)
		}
		if len(asked) == 0 {
			w.Act(ctx, cutter, airline.Action{Kind: "fares", From: a, To: b, Multiplier: 1}) //nolint:errcheck
			file()
		}
	}
	if len(asked) == 0 {
		t.Fatal("the hand-priced carrier was never asked")
	}
	d := asked[0]
	if d.Department != "pricing" || !strings.Contains(d.Title, cutter+" has cut") || d.Default != "act" {
		t.Fatalf("decision %+v", d)
	}
	if got := s.tariff.EffectiveMultiplier(mv.Carrier, a, b); math.Abs(got-1) > 0.005 {
		t.Errorf("the hand-priced carrier was moved before it answered: ×%.3f", got)
	}
	if err := s.Airline.Answer(mv.Carrier, token, d.ID, "hold"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if got := s.tariff.EffectiveMultiplier(mv.Carrier, a, b); math.Abs(got-1) > 0.005 {
		t.Errorf("held, and the fares went to ×%.3f", got)
	}

	// A fare filed on another machine arrives as an event and is the fare
	// here, once.
	body, _ := json.Marshal(fareBody{To: b, Mult: 0.9})
	ev := worldEvent{ID: "test-fare-1", Kind: evFare, Carrier: cutter, From: a, Body: body}
	s.applyEvent(ctx, ev)
	if got := s.tariff.EffectiveMultiplier(cutter, b, a); math.Abs(got-0.9) > 0.001 {
		t.Errorf("a fare from another machine: ×%.3f here", got)
	}
}

// What a carrier's price against its rivals' does to the travellers who
// consider it when the day's advance sales are made: more for the
// cheaper carrier, fewer for the dearer, nothing where it flies alone.
func TestTheDearerCarrierIsConsideredLess(t *testing.T) {
	s := bootWorld(t, Options{GDSCount: 1})
	var contested, alone *worldFlight
	for code := range s.Flights {
		for i := range s.Flights[code] {
			f := &s.Flights[code][i]
			switch n := len(s.rivalsIn(f.From, f.To)); {
			case n >= 2 && contested == nil:
				contested = f
			case n == 1 && alone == nil:
				alone = f
			}
		}
	}
	if contested == nil || alone == nil {
		t.Skip("the small world has no market of each kind")
	}
	if got := s.shareOf(*contested); got != 1 {
		t.Errorf("at the filed fares the share is %.3f", got)
	}
	s.setFare(contested.Carrier, contested.From, contested.To, 1.25)
	dear := s.shareOf(*contested)
	s.setFare(contested.Carrier, contested.From, contested.To, 0.8)
	cheap := s.shareOf(*contested)
	if !(dear < 0.9 && dear > 0.75 && cheap > 1.1 && cheap < 1.3) {
		t.Errorf("a quarter dearer: %.3f of the travellers; a fifth cheaper: %.3f", dear, cheap)
	}
	s.setFare(alone.Carrier, alone.From, alone.To, 1.25)
	if got := s.shareOf(*alone); got != 1 {
		t.Errorf("alone in its market and dearer: %.3f; there is nobody to lose travellers to", got)
	}
}

// A stranded passenger goes to the carrier's own next flight however
// late, then to a joint venture partner, then to the alliance, and to a
// stranger last, whatever the hour each leaves.
func TestStrandedPassengersGoToPartnersFirst(t *testing.T) {
	fl := func(c, n string, dep int) world.Flight {
		return world.Flight{Carrier: c, Number: n, From: "ORD", To: "FRA", DepMin: dep, ArrMin: dep + 500}
	}
	s := &Sim{Flights: map[string][]world.Flight{
		"UA": {fl("UA", "0906", 600), fl("UA", "0944", 1300)},
		"AA": {fl("AA", "0080", 620)},
		"TP": {fl("TP", "0220", 650)},
		"LH": {fl("LH", "0431", 700)},
	}}
	s.commerce = &commerce{partners: partners.New([]string{"UA", "AA", "TP", "LH"}, nil)}
	got, err := s.alternatives(context.Background(), pnr.Segment{Carrier: "UA", FlightNum: "0906", Board: "ORD", Off: "FRA"})
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, c := range got {
		order = append(order, c.Carrier+c.FlightNum)
	}
	if strings.Join(order, " ") != "UA0944 LH0431 TP0220 AA0080" {
		t.Errorf("rebooking order %v", order)
	}
}

// A joint venture's parties share what its routes earn. United and
// Lufthansa both fly Chicago to Frankfurt. United earns 600,000 dollars
// there on 3 flights and Lufthansa 200,000 on 2 of the same aircraft:
// United has three fifths of the seats and three quarters of the money,
// and pays Lufthansa the difference. A flight inside one continent is
// not the venture's, and a carrier in no venture gets nothing.
func TestJointVenturePoolsItsRoutes(t *testing.T) {
	fl := func(c, n, from, to string) world.Flight {
		return world.Flight{Carrier: c, Number: n, From: from, To: to, Seats: 300, KM: 6970}
	}
	s := &Sim{Flights: map[string][]world.Flight{
		"UA": {fl("UA", "0906", "ORD", "FRA"), fl("UA", "0907", "FRA", "ORD"), fl("UA", "0944", "ORD", "FRA"), fl("UA", "0100", "ORD", "DEN")},
		"LH": {fl("LH", "0431", "ORD", "FRA"), fl("LH", "0430", "FRA", "ORD"), fl("LH", "0100", "FRA", "MUC")},
		"AA": {fl("AA", "0080", "ORD", "FRA")},
	}, Ledger: revenue.New()}
	s.airports = map[string]world.Airport{"ORD": {Country: "United States"}, "DEN": {Country: "United States"}, "FRA": {Country: "Germany"}, "MUC": {Country: "Germany"}}
	s.econ = &economy{region: map[string]string{"ORD": refdata.NorthAmerica, "DEN": refdata.NorthAmerica, "FRA": refdata.Europe, "MUC": refdata.Europe}}
	s.commerce = &commerce{partners: partners.New([]string{"UA", "LH", "AA"}, nil)}
	for key, cents := range map[string]int64{
		revenue.Key("UA", "0906", "ORD"): 250_000_00, revenue.Key("UA", "0907", "FRA"): 200_000_00, revenue.Key("UA", "0944", "ORD"): 150_000_00,
		revenue.Key("UA", "0100", "ORD"): 90_000_00, // Chicago to Denver: not the venture's
		revenue.Key("LH", "0431", "ORD"): 120_000_00, revenue.Key("LH", "0430", "FRA"): 80_000_00,
		revenue.Key("LH", "0100", "FRA"): 40_000_00, // Frankfurt to Munich: not the venture's
		revenue.Key("AA", "0080", "ORD"): 300_000_00,
	} {
		s.Ledger.Add(key, cents)
	}
	pool, views := s.pooled()
	// The pool is 800,000 dollars; United's due is three fifths, 480,000,
	// and it earned 600,000.
	if pool["UA"] != -120_000_00 || pool["LH"] != 120_000_00 || pool["AA"] != 0 {
		t.Errorf("what each received: %v", pool)
	}
	if len(views) != 1 || views[0].Name != "Atlantic Plus-Plus" || views[0].Pool != 800_000_00 {
		t.Errorf("the ventures: %+v", views)
	}
}
