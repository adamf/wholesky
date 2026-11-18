package sim

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/pricing"
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
