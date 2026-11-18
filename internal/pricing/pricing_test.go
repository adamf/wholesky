package pricing

import (
	"fmt"
	"math"
	"testing"

	"github.com/adamf/wholesky/internal/equipment"
)

// board is a world's fares for a test: every carrier's multiplier in
// every market, with the desks' moves applied as they are filed.
type board struct {
	t    *testing.T
	d    *Desk
	mult map[key]float64
	tick int
	hand map[string]bool
	log  []Move
}

func newBoard(t *testing.T, seed int64, carriers []Carrier, markets []Market) *board {
	b := &board{t: t, d: New(seed, carriers, markets), mult: map[key]float64{}, hand: map[string]bool{}}
	b.file(Conditions{}) // the first filing only looks
	return b
}

func (b *board) get(c, x, y string) float64 {
	if x > y {
		x, y = y, x
	}
	if v, ok := b.mult[key{c, x, y}]; ok {
		return v
	}
	return 1
}

func (b *board) set(c, x, y string, m float64) {
	if x > y {
		x, y = y, x
	}
	b.mult[key{c, x, y}] = m
}

// file runs one filing and applies what the desks filed.
func (b *board) file(c Conditions) []Move {
	c.Tick = b.tick
	b.tick++
	moves := b.d.Tick(c, b.get, func(code string) bool { return b.hand[code] })
	for _, m := range moves {
		if !m.Proposal {
			b.set(m.Carrier, m.A, m.B, m.Mult)
		}
	}
	b.log = append(b.log, moves...)
	return moves
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.002 }

var (
	aa = Carrier{Code: "AA", Model: equipment.Network, Hub: "DFW", Level: 1.06}
	ua = Carrier{Code: "UA", Model: equipment.Network, Hub: "ORD", Level: 1.05}
	wn = Carrier{Code: "WN", Model: equipment.LowCost, Hub: "DAL", Level: 0.80}
	nk = Carrier{Code: "NK", Model: equipment.UltraLowCost, Hub: "FLL", Level: 0.62}
)

// A cut by a carrier of the same kind is matched at the next filing and
// not undercut, and when the cutter goes back so does the matcher. Ninety
// five times in a hundred: Delta declined to match in April 2016.
func TestAPeersCutIsMatchedAndRestored(t *testing.T) {
	matched := 0
	const worlds = 400
	for seed := int64(0); seed < worlds; seed++ {
		b := newBoard(t, seed, []Carrier{aa, ua}, []Market{{A: "LAX", B: "JFK", Seats: map[string]int{"AA": 900, "UA": 900}}})
		b.set("UA", "LAX", "JFK", 0.85)
		moves := b.file(Conditions{})
		if len(moves) == 0 {
			if again := b.file(Conditions{}); len(again) != 0 {
				t.Fatalf("a carrier that let a cut go answered it a filing later: %+v", again)
			}
			continue
		}
		matched++
		if len(moves) != 1 || moves[0].Carrier != "AA" || moves[0].Kind != Match || moves[0].Rival != "UA" || !near(b.get("AA", "LAX", "JFK"), 0.85) {
			t.Fatalf("seed %d: %+v", seed, moves)
		}
		if again := b.file(Conditions{}); len(again) != 0 {
			t.Fatalf("the cutter does not answer the match, and the matcher does not go lower: %+v", again)
		}
		b.set("UA", "LAX", "JFK", 1)
		back := b.file(Conditions{})
		if len(back) != 1 || back[0].Kind != Restore || !near(b.get("AA", "LAX", "JFK"), 1) {
			t.Fatalf("seed %d: the cutter went back and the matcher did %+v", seed, back)
		}
		if inc, fights := b.d.Open(); inc != 0 || fights != 0 {
			t.Fatalf("seed %d: %d increases and %d fights left open", seed, inc, fights)
		}
	}
	if rate := float64(matched) / worlds; rate < 0.92 || rate > 0.98 {
		t.Errorf("a peer's cut was matched %.3f of the time", rate)
	}
}

// A network carrier leaves a low-cost carrier with a few seats alone,
// goes half way to one with a sixth of the market, and matches one with
// a third: Northwest left Spirit's one Detroit to Philadelphia flight
// alone for six months and matched it on every seat when it added a
// second.
func TestTheSmallLowCostCarrierIsLeftAlone(t *testing.T) {
	for _, c := range []struct {
		seats int
		want  float64
	}{{50, 1}, {180, 0.90}, {500, 0.80}} {
		answered := 0
		for seed := int64(0); seed < 40; seed++ {
			b := newBoard(t, seed, []Carrier{aa, nk}, []Market{{A: "DTW", B: "PHL", Seats: map[string]int{"AA": 1000, "NK": c.seats}}})
			b.set("NK", "DTW", "PHL", 0.80)
			b.file(Conditions{})
			got := b.get("AA", "DTW", "PHL")
			if near(got, c.want) && c.want != 1 {
				answered++
			} else if !near(got, 1) {
				t.Fatalf("with %d seats against 1000 the network carrier went to %.3f", c.seats, got)
			}
		}
		if c.want == 1 && answered != 0 || c.want != 1 && answered < 30 {
			t.Errorf("a low-cost carrier with %d seats against 1000: answered in %d worlds of 40, want fares at %.2f", c.seats, answered, c.want)
		}
	}
}

// A low-cost carrier prices off its own costs. A network carrier's cut
// that still leaves it the cheaper is nothing to it; one that undercuts
// it is answered by going back under.
func TestTheLowCostCarrierStaysUnder(t *testing.T) {
	under, tried := 0, 0
	for seed := int64(0); seed < 60; seed++ {
		b := newBoard(t, seed, []Carrier{aa, wn}, []Market{{A: "DAL", B: "HOU", Seats: map[string]int{"AA": 500, "WN": 900}}})
		b.set("AA", "DAL", "HOU", 0.90) // 0.954 of the market against 0.80: still dearer
		if moves := b.file(Conditions{}); len(moves) != 0 {
			t.Fatalf("the low-cost carrier answered a cut that left it cheaper: %+v", moves)
		}
		b.set("AA", "DAL", "HOU", 0.70) // 0.742: under it
		tried++
		if moves := b.file(Conditions{}); len(moves) == 1 {
			under++
			if got := wn.Level * b.get("WN", "DAL", "HOU"); got >= aa.Level*0.70 || got < aa.Level*0.70*0.95 {
				t.Fatalf("the low-cost carrier went to %.3f of the market against %.3f", got, aa.Level*0.70)
			}
		}
	}
	if rate := float64(under) / float64(tried); rate < 0.80 {
		t.Errorf("undercut, the low-cost carrier went back under %.2f of the time", rate)
	}
}

// An increase is an offer. Network carriers follow most of the time, a
// low-cost carrier less than half of it and an ultra-low-cost carrier
// hardly ever; when a carrier that matters has not followed after two
// days everyone comes back down. About half of all increases stuck from
// 2007 to 2015, and seven in ten when fuel was climbing.
func TestIncreasesStickWhenTheyAreFollowed(t *testing.T) {
	try := func(rival Carrier, seats int, fuel float64) (stuck float64) {
		const worlds = 400
		n := 0
		for seed := int64(0); seed < worlds; seed++ {
			b := newBoard(t, seed, []Carrier{aa, rival}, []Market{{A: "LAX", B: "JFK", Seats: map[string]int{"AA": 1000, rival.Code: seats}}})
			b.hand["AA"] = true // somebody raises American's fares by hand
			b.set("AA", "LAX", "JFK", 1.03)
			first := b.file(Conditions{FuelChange: fuel})
			followed := len(first) == 1 && first[0].Kind == Follow && near(b.get(rival.Code, "LAX", "JFK"), 1.03)
			if len(first) > 1 || len(first) == 1 && !followed {
				t.Fatalf("%+v", first)
			}
			var last []Move
			for i := 0; i < patience+1; i++ {
				if mv := b.file(Conditions{FuelChange: fuel}); len(mv) > 0 {
					last = mv
				}
			}
			small := float64(seats)/float64(1000+seats) < minorShare
			switch {
			case followed || small:
				// It stuck: nobody comes down, and the hand that raised the
				// fares is not told to lower them.
				n++
				if len(last) != 0 || !near(b.get("AA", "LAX", "JFK"), 1.03) {
					t.Fatalf("seed %d: an increase that stuck was taken back: %+v", seed, last)
				}
			default:
				// Refused by a carrier that matters: the desk says to come
				// down, and American is priced by hand so it is a proposal.
				if len(last) != 1 || last[0].Kind != Rollback || !last[0].Proposal || last[0].Carrier != "AA" || !near(last[0].Mult, 1) {
					t.Fatalf("seed %d: a refused increase: %+v", seed, last)
				}
			}
			if inc, _ := b.d.Open(); inc != 0 {
				t.Fatalf("seed %d: the increase is still open after its deadline", seed)
			}
		}
		return float64(n) / worlds
	}
	for _, c := range []struct {
		name     string
		rival    Carrier
		seats    int
		fuel     float64
		min, max float64
	}{
		{"against a network carrier", ua, 1000, 0, 0.80, 0.90},
		{"against a low-cost carrier", wn, 1000, 0, 0.39, 0.51},
		{"against an ultra-low-cost carrier", nk, 1000, 0, 0.06, 0.15},
		{"against an ultra-low-cost carrier too small to matter", nk, 80, 0, 1, 1},
		{"against a low-cost carrier with fuel up eight per cent", wn, 1000, 0.08, 0.61, 0.73},
	} {
		if got := try(c.rival, c.seats, c.fuel); got < c.min || got > c.max {
			t.Errorf("%s an increase stuck %.3f of the time, want %.2f to %.2f", c.name, got, c.min, c.max)
		}
	}
}

// A follower that went up comes back down with the leader when a third
// carrier refuses.
func TestARefusalBringsEveryoneDown(t *testing.T) {
	seen := false
	for seed := int64(0); seed < 200 && !seen; seed++ {
		b := newBoard(t, seed, []Carrier{aa, ua, wn}, []Market{{A: "LAX", B: "JFK", Seats: map[string]int{"AA": 1000, "UA": 900, "WN": 700}}})
		b.d.attempts = nil
		// The desk's own increase: American is the largest.
		var led []Move
		for i := 0; i < 400 && len(led) == 0; i++ {
			led = b.file(Conditions{LeadDay: true})
		}
		if len(led) == 0 || led[0].Kind != Increase || led[0].Carrier != "AA" {
			t.Fatalf("seed %d: the largest carrier never tried an increase: %+v", seed, led)
		}
		next := b.file(Conditions{})
		ual, swa := near(b.get("UA", "LAX", "JFK"), 1.02), near(b.get("WN", "LAX", "JFK"), 1.02)
		for i := 0; i < patience; i++ {
			next = append(next, b.file(Conditions{})...)
		}
		if ual && !swa {
			seen = true
			if !near(b.get("AA", "LAX", "JFK"), 1) || !near(b.get("UA", "LAX", "JFK"), 1) || !near(b.get("WN", "LAX", "JFK"), 1) {
				t.Fatalf("seed %d: Southwest refused and the fares are AA %.3f UA %.3f WN %.3f after %+v", seed,
					b.get("AA", "LAX", "JFK"), b.get("UA", "LAX", "JFK"), b.get("WN", "LAX", "JFK"), next)
			}
		}
		if ual && swa && !near(b.get("AA", "LAX", "JFK"), 1.02) {
			t.Fatalf("seed %d: everyone followed and the increase did not hold", seed)
		}
	}
	if !seen {
		t.Error("no world had one carrier follow and another refuse")
	}
}

// Attacked at its own hub by a carrier with a hub of its own, a network
// carrier matches there and files the same cut in the attacker's best
// hub markets that it also flies. When the attacker retreats, both
// answers go. This is the exchange the Justice Department's complaint
// of 1992 describes, fifty times over.
func TestRetaliationInTheAttackersMarkets(t *testing.T) {
	markets := []Market{
		{A: "DFW", B: "LAX", Seats: map[string]int{"AA": 2000, "UA": 400}},
		{A: "ORD", B: "LAX", Seats: map[string]int{"UA": 2200, "AA": 600}},
		{A: "ORD", B: "JFK", Seats: map[string]int{"UA": 1800, "AA": 500}},
		{A: "ORD", B: "DEN", Seats: map[string]int{"UA": 2500, "AA": 300}},
		{A: "ORD", B: "SFO", Seats: map[string]int{"UA": 1500, "AA": 200}},
		{A: "ORD", B: "MSP", Seats: map[string]int{"UA": 900}}, // American does not fly it
		{A: "DFW", B: "ORD", Seats: map[string]int{"AA": 1500, "UA": 1400}},
	}
	b := newBoard(t, 3, []Carrier{aa, ua}, markets)
	b.hand["UA"] = true
	b.set("UA", "DFW", "LAX", 0.80)
	moves := b.file(Conditions{})
	kinds := map[string][]string{}
	for _, m := range moves {
		kinds[m.Kind] = append(kinds[m.Kind], m.A+"-"+m.B)
		if m.Carrier != "AA" || m.Rival != "UA" || m.Proposal {
			t.Fatalf("%+v", m)
		}
	}
	if len(kinds[Match]) != 1 || kinds[Match][0] != "DFW-LAX" || !near(b.get("AA", "DFW", "LAX"), 0.80) {
		t.Fatalf("the match: %+v", moves)
	}
	// United's three largest Chicago markets that American flies; DFW-ORD
	// is larger than Chicago to San Francisco and touches the hub.
	if fmt.Sprint(kinds[Retaliate]) != "[DEN-ORD LAX-ORD JFK-ORD]" {
		t.Fatalf("the retaliation: %v", kinds[Retaliate])
	}
	for _, m := range []string{"DEN", "LAX", "JFK"} {
		if !near(b.get("AA", "ORD", m), 0.80) {
			t.Errorf("American in Chicago to %s is at %.3f", m, b.get("AA", "ORD", m))
		}
	}
	if !near(b.get("AA", "ORD", "SFO"), 1) || !near(b.get("AA", "DFW", "ORD"), 1) {
		t.Error("the retaliation went past three markets")
	}
	// United, priced by hand, is not moved; and it retreats.
	if got := b.file(Conditions{}); len(got) != 0 {
		t.Fatalf("a filing with nothing new: %+v", got)
	}
	b.set("UA", "DFW", "LAX", 1)
	back := b.file(Conditions{})
	if len(back) != 4 {
		t.Fatalf("United retreated and American filed %+v", back)
	}
	for _, m := range markets {
		if !near(b.get("AA", m.A, m.B), 1) {
			t.Errorf("American in %s-%s is still at %.3f", m.A, m.B, b.get("AA", m.A, m.B))
		}
	}
	if _, fights := b.d.Open(); fights != 0 {
		t.Errorf("%d fights still open", fights)
	}
}

// A carrier somebody prices by hand is told what its desk would do and
// is not moved. If the hand holds, the desk does not take the hold for a
// move; if the hand matches later, the desk does not take the match for
// an attack.
func TestTheHandPricedCarrierIsAsked(t *testing.T) {
	// A world in which American's desk would match: one in twenty would
	// let the cut go.
	var b *board
	var moves []Move
	for seed := int64(0); seed < 20 && len(moves) == 0; seed++ {
		b = newBoard(t, seed, []Carrier{aa, ua}, []Market{{A: "LAX", B: "JFK", Seats: map[string]int{"AA": 900, "UA": 900}}})
		b.hand["AA"], b.hand["UA"] = true, true
		b.set("UA", "LAX", "JFK", 0.85)
		moves = b.file(Conditions{})
	}
	if len(moves) != 1 || !moves[0].Proposal || moves[0].Carrier != "AA" || moves[0].Kind != Match || !near(moves[0].Mult, 0.85) || !near(b.get("AA", "LAX", "JFK"), 1) {
		t.Fatalf("%+v, American at %.3f", moves, b.get("AA", "LAX", "JFK"))
	}
	if again := b.file(Conditions{}); len(again) != 0 {
		t.Fatalf("holding is not a move: %+v", again)
	}
	b.set("AA", "LAX", "JFK", 0.85) // the hand matches, a day late
	if again := b.file(Conditions{}); len(again) != 0 {
		t.Fatalf("a late match is not an attack: %+v", again)
	}
	b.set("UA", "LAX", "JFK", 1)
	back := b.file(Conditions{})
	if len(back) != 1 || back[0].Kind != Restore || !back[0].Proposal || !near(back[0].Mult, 1) {
		t.Fatalf("the cutter went back: %+v", back)
	}
}

// The same world makes the same decisions, and a market with one carrier
// has nobody to answer.
func TestTheDesksAreDeterministic(t *testing.T) {
	run := func() string {
		b := newBoard(t, 9, []Carrier{aa, ua, wn, nk}, []Market{
			{A: "LAX", B: "JFK", Seats: map[string]int{"AA": 900, "UA": 900, "WN": 300}},
			{A: "DAL", B: "HOU", Seats: map[string]int{"WN": 900, "NK": 200}},
			{A: "FLL", B: "ATL", Seats: map[string]int{"NK": 400}},
		})
		b.set("UA", "LAX", "JFK", 0.9)
		b.set("NK", "FLL", "ATL", 0.8)
		for i := 0; i < 30; i++ {
			b.file(Conditions{LeadDay: i%7 == 4, FuelChange: 0.06})
		}
		return fmt.Sprintf("%+v", b.log)
	}
	first := run()
	if first != run() {
		t.Error("two runs of one world filed different fares")
	}
	b := newBoard(t, 9, []Carrier{nk}, []Market{{A: "FLL", B: "ATL", Seats: map[string]int{"NK": 400}}})
	b.set("NK", "FLL", "ATL", 0.8)
	if moves := b.file(Conditions{}); len(moves) != 0 {
		t.Errorf("a carrier alone in a market was answered: %+v", moves)
	}
	if all, contested := b.d.Markets(); all != 1 || contested != 0 {
		t.Errorf("markets %d, contested %d", all, contested)
	}
}
