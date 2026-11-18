package dayplan

import (
	"math"
	"sort"

	"github.com/adamf/wholesky/internal/world"
)

// A regulation's capacity is a list of slots, one every 60/rate minutes
// from its start. Each flight it holds takes the first free slot at or
// after its estimated time over the airport, in the order the flights were
// planned; the wait is the slot's time less the flight's own. The list is
// kept, because the day changes it: a flight that will not fly gives its
// slot back, and what happens to a slot given back is the difference
// between the two regimes the world's air traffic runs under.
//
// In Europe the Network Manager's allocator (CASA) owns the list. A slot
// that comes back is offered down the queue, first planned first served,
// to every flight that can still make it: the true revision process. No
// carrier has a claim on it.
//
// Under a ground delay program in the United States and Canada the slots
// were rationed by schedule and each belongs to the carrier whose flight
// was given it. A carrier that cancels keeps the slot and may put another
// of its flights in it (substitution). Only when it has no flight that
// can use the slot does compression move another carrier's flight up, and
// then the first carrier is paid with the slot that flight leaves, which
// one of its own later flights can take. That is why a carrier in a
// ground delay program cancels early and says so: the slot is worth
// something to it only while it can still be used.
//
// The method is the published one in both cases (EUROCONTROL's ATFCM
// Users Manual; the FAA's collaborative decision making procedures, and
// Vossen and Ball's account of ration by schedule and compression). The
// notice a flight needs before it can take an earlier slot is the world's
// number.
const (
	// ModeCASA is a regulation whose freed slots are rationed again to
	// every flight, first planned first served.
	ModeCASA = "casa"
	// ModeGDP is a ground delay program: a freed slot stays with its
	// carrier, then compression.
	ModeGDP = "gdp"
	// ImproveNotice is how long before its new off-block time a flight
	// must hear of an earlier slot to take it: the passengers are not at
	// the gate for a departure nobody announced.
	ImproveNotice = 30
	// ReadyNotice is the same for a flight that has reported ready (REA):
	// doors closed, waiting for its time.
	ReadyNotice = 5
)

// modeFor is the regime an airport's regulations run under.
func modeFor(country string) string {
	switch country {
	case "United States", "Canada":
		return ModeGDP
	}
	return ModeCASA
}

// slotBook is one regulation's slot list.
type slotBook struct {
	start float64 // the time of slot 0, minutes into the day
	iv    float64 // minutes between slots
	// depart says the slots are off-block times (a ground stop releasing
	// its departures in turn), not times over the arrival airport.
	depart  bool
	flights map[string]world.Flight // every flight ever held, by key
	slot    map[string]int          // flight -> the slot it holds
	taken   map[int]string          // slot -> the flight holding it
	owner   map[int]string          // slot -> the carrier it was rationed to
	delay   map[string]int          // flight -> minutes this regulation holds it
	// frozen flights had left when the regulation began to hold them:
	// they keep their place and are never moved.
	frozen map[string]bool
	// open are slots given back and not yet retaken, in order.
	open []int
}

func newSlotBook(start, iv float64, depart bool) *slotBook {
	return &slotBook{start: start, iv: iv, depart: depart, flights: map[string]world.Flight{}, slot: map[string]int{},
		taken: map[int]string{}, owner: map[int]string{}, delay: map[string]int{}, frozen: map[string]bool{}}
}

// eto is the flight's own time at the regulated point: its scheduled
// arrival, or its scheduled off-block under a ground stop.
func (b *slotBook) eto(f world.Flight) float64 {
	if b.depart {
		return float64(f.DepMin)
	}
	return float64(f.ArrMin)
}

func (b *slotBook) timeOf(k int) float64 { return b.start + float64(k)*b.iv }

// bucket is the first slot a flight ready at `at` could use.
func (b *slotBook) bucket(at float64) int {
	if at <= b.start {
		return 0
	}
	return int(math.Floor((at - b.start) / b.iv))
}

// take gives a flight the first free slot at or after its earliest time
// and returns the time it gets.
func (b *slotBook) take(f world.Flight, earliest float64) float64 {
	k := b.bucket(earliest)
	for b.taken[k] != "" {
		k++
	}
	key := Key(f)
	b.flights[key] = f
	b.taken[k], b.slot[key] = key, k
	if _, owned := b.owner[k]; !owned {
		b.owner[k] = f.Carrier
	}
	return math.Max(earliest, b.timeOf(k))
}

// wait is the minutes a flight in slot k waits, having been ready at
// earliest.
func (b *slotBook) wait(f world.Flight, k int, earliest float64) int {
	at := math.Max(earliest, b.timeOf(k))
	d := int(math.Round(at - b.eto(f)))
	if d < 0 {
		d = 0
	}
	if !b.depart && d > MaxSlotDelay {
		d = MaxSlotDelay
	}
	return d
}

// counts is the regulation's tally: the flights it holds and their wait.
func (b *slotBook) counts() (flights, minutes int) {
	for _, d := range b.delay {
		if d > 0 {
			flights++
			minutes += d
		}
	}
	return flights, minutes
}

// held is the book's flights in the order they were planned.
func (b *slotBook) held() []world.Flight {
	out := make([]world.Flight, 0, len(b.slot))
	for key := range b.slot {
		out = append(out, b.flights[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if ei, ej := b.eto(out[i]), b.eto(out[j]); ei != ej {
			return ei < ej
		}
		return Key(out[i]) < Key(out[j])
	})
	return out
}

// earliest is the first moment a flight could be at the regulated point
// if it were told now: its own time, or later when it could not be off
// the blocks by then with the notice it needs.
func (p *Plan) earliest(b *slotBook, f world.Flight, now int) float64 {
	e := b.eto(f)
	if now < 0 {
		return e
	}
	notice := ImproveNotice
	if p.ready[Key(f)] {
		notice = ReadyNotice
	}
	if late := float64(now+notice) - float64(f.DepMin); late > 0 {
		e += late
	}
	return e
}

// movable says a flight can still be given a different slot: it is held,
// it has not left, and it has not been cancelled out of the list.
func (p *Plan) movable(b *slotBook, f world.Flight, now int) bool {
	key := Key(f)
	pf := p.Flights[key]
	if pf == nil || b.frozen[key] || p.released[key] || b.delay[key] <= 0 {
		return false
	}
	return now < 0 || f.DepMin+pf.DepDelay > now
}

// revise is the true revision process: every flight that can still move
// is offered, in the order it was planned, the first free slot it can
// make. It returns the flights that moved up.
func (p *Plan) revise(b *slotBook, now int) []world.Flight {
	var moved []world.Flight
	for _, f := range b.held() {
		if !p.movable(b, f, now) {
			continue
		}
		key := Key(f)
		cur := b.slot[key]
		e := p.earliest(b, f, now)
		k := b.bucket(e)
		for k < cur && b.taken[k] != "" {
			k++
		}
		if k >= cur {
			continue
		}
		// The move is made even when the wait it buys is none -- a flight
		// held past the longest wait the plan counts stays at that wait --
		// because the slot it leaves is the next flight's improvement.
		delete(b.taken, cur)
		b.taken[k], b.slot[key], b.delay[key] = key, k, min(b.delay[key], b.wait(f, k, e))
		moved = append(moved, f)
	}
	b.open = b.open[:0]
	return moved
}

// compress fills the slots given back under a ground delay program: the
// owner's own next flight that can use the slot takes it; failing that
// another carrier's does, and the owner gets the slot that flight leaves.
// Each move opens a later slot, which is worked in turn; a slot nobody can
// use stays open for a flight that reports ready later.
func (p *Plan) compress(b *slotBook, now int) []world.Flight {
	var moved []world.Flight
	seen := map[string]bool{}
	sort.Ints(b.open)
	for i := 0; i < len(b.open); {
		s := b.open[i]
		owner := b.owner[s]
		var pick *world.Flight
		for _, f := range b.held() {
			key := Key(f)
			if b.slot[key] <= s || !p.movable(b, f, now) {
				continue
			}
			e := p.earliest(b, f, now)
			if b.bucket(e) > s {
				continue
			}
			if f.Carrier == owner {
				g := f
				pick = &g
				break
			}
			if pick == nil {
				g := f
				pick = &g
			}
		}
		if pick == nil {
			i++
			continue
		}
		key := Key(*pick)
		old := b.slot[key]
		delete(b.taken, old)
		b.taken[s], b.slot[key] = key, s
		b.delay[key] = min(b.delay[key], b.wait(*pick, s, p.earliest(b, *pick, now)))
		if pick.Carrier != owner {
			// The trade: the mover's carrier now owns the slot it moved
			// into, and the carrier that gave that slot up owns the one
			// the mover left.
			b.owner[s], b.owner[old] = pick.Carrier, owner
		}
		if !seen[key] {
			seen[key] = true
			moved = append(moved, *pick)
		}
		b.open[i] = old
		sort.Ints(b.open[i:])
	}
	return moved
}

// fill offers a regulation's free slots to the flights behind them, by
// the regulation's regime.
func (p *Plan) fill(r *Regulation, now int) []world.Flight {
	if r.book == nil {
		return nil
	}
	if r.Mode == ModeGDP {
		return p.compress(r.book, now)
	}
	return p.revise(r.book, now)
}

// settle works out a flight's slot from every regulation that holds it:
// the longest wait binds, and names the regulation and the cause. It
// reports whether the wait changed.
func (p *Plan) settle(f world.Flight) bool {
	key := Key(f)
	pf := p.Flights[key]
	if pf == nil {
		return false
	}
	best := 0
	var by *Regulation
	for i := range p.Regulations {
		r := &p.Regulations[i]
		if r.book == nil {
			continue
		}
		if d, ok := r.book.delay[key]; ok && d > best {
			best, by = d, r
		}
	}
	was := pf.ATFM
	pf.ATFM = best
	if by != nil {
		pf.Regulation, pf.Cause = by.Name, by.Cause
		pf.CTOT = f.DepMin + best + TaxiMin
	} else {
		pf.Regulation, pf.Cause, pf.CTOT = "", nil, 0
	}
	return was != best
}

// recount brings every regulation's tally and the day's summary into line
// with the slot lists.
func (p *Plan) recount() {
	p.Summary.Slotted, p.Summary.SlotDelayMin = 0, 0
	for i := range p.Regulations {
		r := &p.Regulations[i]
		if r.book != nil {
			r.Flights, r.DelayMin = r.book.counts()
		}
		p.Summary.Slotted += r.Flights
		p.Summary.SlotDelayMin += r.DelayMin
	}
}

// Change is one flight whose slot the day improved, with the fate it had.
type Change struct {
	Flight world.Flight
	Was    Flight
}

// Release gives back the slots of a flight that will not fly, at now
// minutes into the day (-1 before the day), and offers them to the
// flights behind it. It is called when the cancellation is announced:
// until then nobody outside the carrier knows, and the slot is held. It
// returns the flights that took an earlier slot, with the fate each had,
// and works the day again down every tail that moved.
func (p *Plan) Release(m *world.Manifest, f world.Flight, now int) []Change {
	if p == nil || p.Replay {
		return nil
	}
	key := Key(f)
	p.mu.Lock()
	if p.released == nil {
		p.released = map[string]bool{}
	}
	if p.released[key] {
		p.mu.Unlock()
		return nil
	}
	p.released[key] = true
	before := map[string]Flight{}
	var moved []world.Flight
	for i := range p.Regulations {
		r := &p.Regulations[i]
		b := r.book
		if b == nil {
			continue
		}
		k, ok := b.slot[key]
		if !ok {
			continue
		}
		delete(b.slot, key)
		delete(b.taken, k)
		delete(b.delay, key)
		if !b.frozen[key] {
			b.open = append(b.open, k)
		}
		for _, g := range b.held() {
			if _, seen := before[Key(g)]; !seen {
				before[Key(g)] = *p.Flights[Key(g)]
			}
		}
		moved = append(moved, p.fill(r, now)...)
	}
	p.settle(f)
	changes := p.apply(moved, before)
	p.mu.Unlock()
	p.replanTails(m, changes)
	return changes
}

// Ready records that a flight has reported ready (REA) and offers it, and
// the flights behind it, whatever slots are free at the shorter notice a
// ready flight needs. A flight is improved only when a slot ahead of it
// has really been given back.
func (p *Plan) Ready(m *world.Manifest, f world.Flight, now int) []Change {
	if p == nil || p.Replay {
		return nil
	}
	key := Key(f)
	p.mu.Lock()
	if p.ready == nil {
		p.ready = map[string]bool{}
	}
	p.ready[key] = true
	before := map[string]Flight{}
	var moved []world.Flight
	for i := range p.Regulations {
		r := &p.Regulations[i]
		b := r.book
		if b == nil {
			continue
		}
		if _, ok := b.slot[key]; !ok {
			continue
		}
		for _, g := range b.held() {
			if _, seen := before[Key(g)]; !seen {
				before[Key(g)] = *p.Flights[Key(g)]
			}
		}
		if r.Mode == ModeGDP {
			moved = append(moved, p.compress(b, now)...)
			// A slot its owner cannot use and nobody was moved into is
			// open to a flight that is ready now.
			moved = append(moved, p.takeOpen(b, f, now)...)
		} else {
			moved = append(moved, p.revise(b, now)...)
		}
	}
	changes := p.apply(moved, before)
	p.mu.Unlock()
	p.replanTails(m, changes)
	return changes
}

// takeOpen moves one flight into the earliest open slot it can use.
func (p *Plan) takeOpen(b *slotBook, f world.Flight, now int) []world.Flight {
	if !p.movable(b, f, now) {
		return nil
	}
	key := Key(f)
	e := p.earliest(b, f, now)
	sort.Ints(b.open)
	for i, s := range b.open {
		if s >= b.slot[key] {
			break
		}
		if b.taken[s] != "" || b.bucket(e) > s {
			continue
		}
		old := b.slot[key]
		delete(b.taken, old)
		b.taken[s], b.slot[key] = key, s
		b.delay[key] = min(b.delay[key], b.wait(f, s, e))
		if owner := b.owner[s]; owner != f.Carrier {
			b.owner[s], b.owner[old] = f.Carrier, owner
		}
		b.open[i] = old
		return []world.Flight{f}
	}
	return nil
}

// apply settles the fate of every flight the slot lists moved and returns
// the ones whose wait really fell, with the fate each had. Called under
// the lock.
func (p *Plan) apply(moved []world.Flight, before map[string]Flight) []Change {
	var out []Change
	seen := map[string]bool{}
	for _, g := range moved {
		key := Key(g)
		if seen[key] {
			continue
		}
		seen[key] = true
		pf := p.Flights[key]
		if pf == nil || !p.settle(g) {
			continue
		}
		was := before[key]
		if pf.ATFM >= was.ATFM {
			continue
		}
		// The flight's delay without the chain down its tail: its own or
		// the slot's, whichever is longer, and a reserve callout on top.
		// A tail's chain is worked again by the caller.
		base := max(pf.Own, pf.ATFM)
		if pf.Reserve && p.Staff == nil {
			base += ReserveCall
		}
		if g.Tail == "" {
			pf.DepDelay, pf.ArrDelay = base, base
		}
		out = append(out, Change{Flight: g, Was: was})
	}
	p.recount()
	return out
}

// replanTails works the day again down each tail whose leg moved.
func (p *Plan) replanTails(m *world.Manifest, changes []Change) {
	done := map[string]bool{}
	for _, c := range changes {
		f := c.Flight
		if f.Tail == "" || done[f.Carrier+"/"+f.Tail] {
			continue
		}
		done[f.Carrier+"/"+f.Tail] = true
		p.Replan(m, f.Carrier, f.Tail)
	}
}

// Slot is a flight's place in a regulation, for the instruments.
type Slot struct {
	Regulation string `json:"regulation"`
	Mode       string `json:"mode"`
	// At is the slot's time, minutes into the day: over the airport, or
	// off the blocks under a ground stop.
	At    int    `json:"at"`
	Owner string `json:"owner,omitempty"`
	Wait  int    `json:"wait"`
}

// Slots is every slot a flight holds, the binding one first.
func (p *Plan) Slots(f world.Flight) []Slot {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	key := Key(f)
	var out []Slot
	for i := range p.Regulations {
		r := &p.Regulations[i]
		if r.book == nil {
			continue
		}
		k, ok := r.book.slot[key]
		if !ok {
			continue
		}
		out = append(out, Slot{Regulation: r.Name, Mode: r.Mode, At: int(r.book.timeOf(k)), Owner: r.book.owner[k], Wait: r.book.delay[key]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Wait > out[j].Wait })
	return out
}
