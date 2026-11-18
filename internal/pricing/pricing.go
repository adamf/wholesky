// Package pricing is the pricing desks of the carriers nobody is running
// by hand: what each does when a rival moves a fare.
//
// The rules are the ones the public record shows airlines following.
// Fares are filed a few times a day and every carrier sees every other
// carrier's by the next filing, so a move is answered within hours
// (United States v. Airline Tariff Publishing Co., the Justice
// Department's competitive impact statement of 1994, describes the
// cycle). A carrier matches a cut by a carrier of its own kind almost
// always and does not undercut it. A network carrier ignores a low-cost
// carrier too small to hurt it and answers one that is not. An increase
// is an offer: the others follow it or they do not, and when a carrier
// that matters has not followed by the deadline everyone who went up
// comes back down. Network carriers follow most of the time and low-cost
// carriers less than half of it, which is why about half of all
// increases stick (FareCompare's tallies for 2007 to 2015). A carrier
// attacked at its own hub answers there and also in the attacker's best
// markets, and takes the second answer away when the attacker retreats
// (the complaint in the same case sets out fifty such exchanges). When
// the move that started a fight is undone, so is everything that
// answered it.
//
// The probabilities are estimates. The orderings between kinds of
// carrier are what the evidence supports. The levels are tuned so that
// about half of the desks' own increases hold.
package pricing

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"

	"github.com/adamf/wholesky/internal/equipment"
)

// Carrier is one carrier as the desks see it.
type Carrier struct {
	Code  string
	Model equipment.Model
	Hub   string
	// Level is where the carrier's fares sit at a multiplier of one, as a
	// share of the market's average: the tariff's figure.
	Level float64
}

// Market is one pair of airports and the seats each carrier flies
// between them in a day, both directions together.
type Market struct {
	A, B  string
	Seats map[string]int
}

func (m Market) total() int {
	n := 0
	for _, s := range m.Seats {
		n += s
	}
	return n
}

func (m Market) share(c string) float64 {
	if t := m.total(); t > 0 {
		return float64(m.Seats[c]) / float64(t)
	}
	return 0
}

// Kinds of move.
const (
	Match     = "match"     // answering a rival's cut
	Follow    = "follow"    // going up after a rival's increase
	Increase  = "increase"  // a desk's own attempt to raise fares
	Rollback  = "rollback"  // an increase withdrawn because it was not followed
	Retaliate = "retaliate" // a cut in the attacker's own market
	Restore   = "restore"   // back to where the fares were before a fight
)

// Move is one fare change a desk makes, or would make.
type Move struct {
	Carrier string
	A, B    string
	// Mult is the carrier's new multiplier in the market.
	Mult float64
	Kind string
	// Rival is the carrier whose move this answers.
	Rival string
	Why   string
	// Proposal says the carrier is priced by hand: the desk has not filed
	// this, and it is for the caller to put to whoever runs the carrier.
	Proposal bool
}

// Conditions is the state of the world at a filing.
type Conditions struct {
	// Tick counts filings from the start; there are four in a day.
	Tick int
	// LeadDay says today is a day increases are tried on: a Thursday or a
	// Friday, so that the weekend shows who followed.
	LeadDay bool
	// FuelChange is how much the fuel index moved in the last month, as
	// a share: 0.05 is five per cent up.
	FuelChange float64
}

// The estimates.
const (
	// A move of under one per cent is not a move.
	notice = 0.01
	// A carrier with less than this share of a market's seats is not
	// answered by a network carrier, and its refusal does not stop an
	// increase.
	minorShare = 0.10
	// Under this share a low-cost carrier's cut is half matched.
	majorShare = 0.25
	// An increase has this many filings to be followed: two days.
	patience = 8
	// The size of a desk's own increase: two per cent, or three when fuel
	// has risen five per cent in a month.
	step, fuelStep = 0.02, 0.03
	// How often the largest carrier in a market tries one on a lead day's
	// filing.
	tryRate = 0.06
	// No desk prices under this share of its filed fares, or over this.
	floor, ceiling = 0.70, 1.60
	// A carrier attacked at its hub answers in this many of the
	// attacker's markets.
	punishMarkets = 3
)

// matchRate is how often a carrier of one kind answers a cut by another.
func matchRate(own, cutter equipment.Model) float64 {
	switch own {
	case equipment.Network:
		if cutter == equipment.Network {
			return 0.95
		}
		return 0.90
	case equipment.LowCost:
		return 0.90
	}
	return 0.50
}

// followRate is how often a carrier of a kind follows an increase.
func followRate(m equipment.Model, fuel float64) float64 {
	p := 0.10
	switch m {
	case equipment.Network:
		p = 0.85
	case equipment.LowCost:
		p = 0.45
	}
	// When fuel is rising everyone needs the money: in the spike of 2007
	// and 2008 about seven increases in ten held.
	if fuel > 0.05 {
		p += (1 - p) * 0.4
	}
	return p
}

type key struct{ carrier, a, b string }

// fight is what a carrier did in answer to a rival and what undoes it.
type fight struct {
	rival string
	// was is the carrier's multiplier before it answered; rivalWas the
	// rival's before it moved.
	was, rivalWas float64
	// trigger, for a retaliation, is the market the rival attacked.
	trigger *key
}

// attempt is an increase waiting to be followed.
type attempt struct {
	leader   string
	a, b     string
	was      float64 // the leader's multiplier before
	ratio    float64 // how much it went up
	deadline int
	asked    map[string]bool    // carriers that have decided
	went     map[string]float64 // followers and where they were before
}

// Desk is every autopilot carrier's pricing desk.
type Desk struct {
	Seed     int64
	carriers map[string]Carrier
	markets  []Market
	by       map[string]int // "A/B" to index
	last     map[key]float64
	fights   map[key]*fight
	attempts []*attempt
	started  bool
}

// New builds the desks for a world's carriers and markets.
func New(seed int64, carriers []Carrier, markets []Market) *Desk {
	d := &Desk{Seed: seed, carriers: map[string]Carrier{}, by: map[string]int{}, last: map[key]float64{}, fights: map[key]*fight{}}
	for _, c := range carriers {
		d.carriers[c.Code] = c
	}
	for _, m := range markets {
		if m.A > m.B {
			m.A, m.B = m.B, m.A
		}
		if _, dup := d.by[m.A+"/"+m.B]; dup || len(m.Seats) == 0 {
			continue
		}
		d.by[m.A+"/"+m.B] = len(d.markets)
		d.markets = append(d.markets, m)
	}
	return d
}

// Markets is how many markets the desks watch, and how many of them
// have more than one carrier.
func (d *Desk) Markets() (all, contested int) {
	for _, m := range d.markets {
		if len(m.Seats) > 1 {
			contested++
		}
	}
	return len(d.markets), contested
}

// roll is a number from zero to one that depends on the seed, the
// filing and what is being decided, and on nothing else: the same world
// makes the same decisions.
func (d *Desk) roll(tick int, parts ...string) float64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%d/%d", d.Seed, tick)
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return float64(h.Sum64()>>11) / float64(1<<53)
}

func clamp(m float64) float64 { return math.Max(floor, math.Min(ceiling, m)) }

// Tick is one filing. mult is each carrier's multiplier in a market as
// it stands now, with every change made since the last filing in it,
// whoever made it. The moves returned are what the desks file in
// answer; the caller applies them, or for a carrier somebody prices by
// hand puts them to that somebody. manual says which carriers those are:
// their own moves are seen and answered, and the answers the desk would
// have made for them come back as moves for the caller to offer.
func (d *Desk) Tick(c Conditions, mult func(carrier, a, b string) float64, manual func(carrier string) bool) []Move {
	cur := map[key]float64{}
	for _, m := range d.markets {
		for code := range m.Seats {
			cur[key{code, m.A, m.B}] = mult(code, m.A, m.B)
		}
	}
	if !d.started {
		d.started, d.last = true, cur
		return nil
	}
	var moves []Move
	// now is a carrier's multiplier with this filing's moves in it.
	now := map[key]float64{}
	for k, v := range cur {
		now[k] = v
	}
	file := func(mv Move) {
		mv.Mult = clamp(mv.Mult)
		k := key{mv.Carrier, mv.A, mv.B}
		if math.Abs(mv.Mult-now[k]) < 0.0005 {
			return
		}
		if manual != nil && manual(mv.Carrier) {
			// Somebody prices this carrier: the desk only says what it
			// would do, and the fares stay where that somebody has them.
			mv.Proposal = true
		} else {
			now[k] = mv.Mult
		}
		moves = append(moves, mv)
	}
	for _, m := range d.markets {
		if len(m.Seats) < 2 {
			continue
		}
		codes := make([]string, 0, len(m.Seats))
		for code := range m.Seats {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, x := range codes {
			kx := key{x, m.A, m.B}
			was, is := d.last[kx], cur[kx]
			if was == 0 || math.Abs(is/was-1) < notice {
				continue
			}
			if is < was {
				d.cut(c, m, codes, x, was, is, now, file)
			} else {
				d.rise(c, m, codes, x, was, is, now, file)
			}
		}
	}
	d.settle(c, now, file)
	if c.LeadDay {
		d.lead(c, now, manual, file)
	}
	// What a desk filed this time is known at the next: the answer to an
	// answer comes a filing later, as it does.
	d.last = now
	return moves
}

// cut answers carrier x's cut in a market.
func (d *Desk) cut(c Conditions, m Market, codes []string, x string, was, is float64, now map[key]float64, file func(Move)) {
	cx := d.carriers[x]
	ratio := is / was
	// A cut that only takes back an increase the carrier led is not an
	// attack: the attempt's own bookkeeping deals with it.
	for _, at := range d.attempts {
		if at.leader == x && at.a == m.A && at.b == m.B {
			return
		}
	}
	// Nor is a follower that changed its mind.
	for _, at := range d.attempts {
		if _, ok := at.went[x]; ok && at.a == m.A && at.b == m.B {
			return
		}
	}
	// A carrier that cut to answer someone is not attacking anyone.
	if f := d.fights[key{x, m.A, m.B}]; f != nil && is < f.was {
		return
	}
	for _, y := range codes {
		if y == x {
			continue
		}
		ky := key{y, m.A, m.B}
		cy := d.carriers[y]
		if d.fights[ky] != nil {
			continue // already in a fight here
		}
		to, why := 0.0, ""
		switch {
		case cy.Model == equipment.Network && cx.Model != equipment.Network:
			// The small low-cost carrier is left alone; the one with a
			// quarter of the seats is matched; in between, half way.
			switch share := m.share(x); {
			case share < minorShare:
				continue
			case share < majorShare:
				to, why = now[ky]*(1+(ratio-1)/2), fmt.Sprintf("half way to %s's cut: it flies %.0f%% of the seats", x, share*100)
			default:
				to, why = now[ky]*ratio, fmt.Sprintf("matched %s's cut", x)
			}
		case cy.Model == equipment.Network || cy.Model == cx.Model:
			// A carrier of the same kind: match, and no lower.
			to, why = now[ky]*ratio, fmt.Sprintf("matched %s's cut", x)
		default:
			// A low-cost carrier prices off its own costs and moves only
			// when it has been undercut: then it goes back under.
			if cx.Level*is >= cy.Level*now[ky] {
				continue
			}
			to, why = cx.Level*is/cy.Level*0.97, fmt.Sprintf("back under %s", x)
		}
		if d.roll(c.Tick, "match", y, x, m.A, m.B) >= matchRate(cy.Model, cx.Model) {
			continue
		}
		d.fights[ky] = &fight{rival: x, was: now[ky], rivalWas: was}
		file(Move{Carrier: y, A: m.A, B: m.B, Mult: to, Kind: Match, Rival: x, Why: why})
		// Attacked where it lives by a carrier with a home of its own, a
		// network carrier answers there too.
		if cy.Model == equipment.Network && cx.Model == equipment.Network && cx.Hub != "" && cx.Hub != cy.Hub &&
			(m.A == cy.Hub || m.B == cy.Hub) && m.share(y) >= 0.5 {
			d.punish(c, y, x, ratio, &key{x, m.A, m.B}, now, file)
		}
	}
}

// punish files carrier y's cut in the best of rival x's hub markets that
// y also flies.
func (d *Desk) punish(c Conditions, y, x string, ratio float64, trigger *key, now map[key]float64, file func(Move)) {
	hub := d.carriers[x].Hub
	var hurt []Market
	for _, m := range d.markets {
		if (m.A == hub || m.B == hub) && m.Seats[x] > 0 && m.Seats[y] > 0 && !(m.A == trigger.a && m.B == trigger.b) && d.fights[key{y, m.A, m.B}] == nil {
			hurt = append(hurt, m)
		}
	}
	sort.Slice(hurt, func(i, j int) bool {
		if hurt[i].Seats[x] != hurt[j].Seats[x] {
			return hurt[i].Seats[x] > hurt[j].Seats[x]
		}
		return hurt[i].A+hurt[i].B < hurt[j].A+hurt[j].B
	})
	for i, m := range hurt {
		if i == punishMarkets {
			break
		}
		ky := key{y, m.A, m.B}
		d.fights[ky] = &fight{rival: x, was: now[ky], rivalWas: now[key{x, m.A, m.B}], trigger: trigger}
		file(Move{Carrier: y, A: m.A, B: m.B, Mult: now[ky] * ratio, Kind: Retaliate, Rival: x,
			Why: fmt.Sprintf("%s cut %s-%s, where %s lives", x, trigger.a, trigger.b, y)})
	}
}

// rise deals with carrier x's increase in a market: the end of a fight
// it started, or an offer to the others.
func (d *Desk) rise(c Conditions, m Market, codes []string, x string, was, is float64, now map[key]float64, file func(Move)) {
	ended := false
	for k, f := range d.fights {
		if f.rival != x {
			continue
		}
		// The fight over this market, or a retaliation it set off.
		here := k.a == m.A && k.b == m.B && f.trigger == nil
		there := f.trigger != nil && f.trigger.a == m.A && f.trigger.b == m.B
		if !(here || there) || is < f.rivalWas*(1-notice) && here {
			continue
		}
		if there && is < d.fightStart(x, m)*(1-notice) {
			continue
		}
		ended = true
		delete(d.fights, k)
		file(Move{Carrier: k.carrier, A: k.a, B: k.b, Mult: f.was, Kind: Restore, Rival: x, Why: fmt.Sprintf("%s took its cut back", x)})
	}
	if ended {
		return
	}
	// A carrier coming back up from its own answer to a fight is not
	// leading anything.
	if d.fights[key{x, m.A, m.B}] != nil {
		return
	}
	for _, at := range d.attempts {
		if at.leader == x && at.a == m.A && at.b == m.B {
			return
		}
		if _, ok := at.went[x]; ok && at.a == m.A && at.b == m.B {
			return // a follower's own rise
		}
	}
	d.attempts = append(d.attempts, &attempt{leader: x, a: m.A, b: m.B, was: was, ratio: is / was, deadline: c.Tick + patience,
		asked: map[string]bool{x: true}, went: map[string]float64{}})
}

// fightStart is where carrier x stood in a market before the cut that
// set a retaliation off: the level it must come back to for the
// retaliation to end.
func (d *Desk) fightStart(x string, m Market) float64 {
	for k, f := range d.fights {
		if f.rival == x && k.a == m.A && k.b == m.B && f.trigger == nil {
			return f.rivalWas
		}
	}
	return 0
}

// settle moves every open increase on: the carriers that have not
// decided decide, and at the deadline an increase a carrier that matters
// refused is withdrawn by everyone.
func (d *Desk) settle(c Conditions, now map[key]float64, file func(Move)) {
	keep := d.attempts[:0]
	for _, at := range d.attempts {
		m := d.markets[d.by[at.a+"/"+at.b]]
		kl := key{at.leader, at.a, at.b}
		// The leader gave up on its own.
		if now[kl] < at.was*(1+(at.ratio-1)/2) {
			for y, was := range at.went {
				file(Move{Carrier: y, A: at.a, B: at.b, Mult: was, Kind: Rollback, Rival: at.leader, Why: fmt.Sprintf("%s withdrew its increase", at.leader)})
			}
			continue
		}
		codes := make([]string, 0, len(m.Seats))
		for code := range m.Seats {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, y := range codes {
			if at.asked[y] {
				continue
			}
			at.asked[y] = true
			ky := key{y, at.a, at.b}
			if d.fights[ky] != nil {
				continue // in a fight: not now
			}
			if d.roll(c.Tick, "follow", y, at.leader, at.a, at.b) < followRate(d.carriers[y].Model, c.FuelChange) {
				at.went[y] = now[ky]
				file(Move{Carrier: y, A: at.a, B: at.b, Mult: now[ky] * at.ratio, Kind: Follow, Rival: at.leader, Why: fmt.Sprintf("followed %s's increase", at.leader)})
			}
		}
		if c.Tick < at.deadline {
			keep = append(keep, at)
			continue
		}
		// The deadline. A carrier with a tenth of the seats that stayed
		// down takes the travellers of everyone who went up.
		var refused []string
		for _, y := range codes {
			if y == at.leader || m.share(y) < minorShare {
				continue
			}
			if was, ok := at.went[y]; !ok || now[key{y, at.a, at.b}] < was*(1+(at.ratio-1)/2) {
				refused = append(refused, y)
			}
		}
		if len(refused) == 0 {
			continue // it stuck
		}
		why := fmt.Sprintf("%s did not follow", refused[0])
		file(Move{Carrier: at.leader, A: at.a, B: at.b, Mult: at.was, Kind: Rollback, Rival: refused[0], Why: why})
		for y, was := range at.went {
			file(Move{Carrier: y, A: at.a, B: at.b, Mult: was, Kind: Rollback, Rival: refused[0], Why: why})
		}
	}
	d.attempts = keep
}

// lead has the largest network carrier in each market try an increase
// now and then, more often when fuel is going up.
func (d *Desk) lead(c Conditions, now map[key]float64, manual func(string) bool, file func(Move)) {
	rate, size := tryRate, step
	if c.FuelChange > 0.05 {
		rate, size = tryRate*3, fuelStep
	}
	// A carrier leads everywhere it is the largest, or nowhere: an
	// increase is a system-wide filing.
	leaders := map[string]bool{}
	for code, cr := range d.carriers {
		if cr.Model == equipment.Network && (manual == nil || !manual(code)) && d.roll(c.Tick, "lead", code) < rate {
			leaders[code] = true
		}
	}
	if len(leaders) == 0 {
		return
	}
	open := map[string]bool{}
	for _, at := range d.attempts {
		open[at.a+"/"+at.b] = true
	}
	for _, m := range d.markets {
		if open[m.A+"/"+m.B] {
			continue
		}
		top, most := "", 0
		for code, seats := range m.Seats {
			if seats > most || (seats == most && code < top) {
				top, most = code, seats
			}
		}
		k := key{top, m.A, m.B}
		if !leaders[top] || d.fights[k] != nil || now[k] >= ceiling {
			continue
		}
		was := now[k]
		file(Move{Carrier: top, A: m.A, B: m.B, Mult: was * (1 + size), Kind: Increase, Why: fmt.Sprintf("up %.0f%%", size*100)})
		if len(m.Seats) > 1 {
			d.attempts = append(d.attempts, &attempt{leader: top, a: m.A, b: m.B, was: was, ratio: now[k] / was, deadline: c.Tick + patience,
				asked: map[string]bool{top: true}, went: map[string]float64{}})
		}
	}
}

// Open is the increases waiting to be followed and the fights going on,
// for the instruments.
func (d *Desk) Open() (increases, fights int) { return len(d.attempts), len(d.fights) }
