package dayplan

import (
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

var slotDay = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

func calm(world.Flight, time.Time) (int, int) { return 0, 0 }

// queueManifest is one arrival airport with an arrival every ten minutes
// from 1400, flown by the carriers given in turn, each leg seventy-five
// minutes long from a field that is not regulated.
func queueManifest(country string, carriers ...string) *world.Manifest {
	m := &world.Manifest{
		Airports: []world.Airport{
			{IATA: "HUB", ICAO: "XHUB", Country: country, Lat: 50, Lon: 8},
			{IATA: "OUT", ICAO: "XOUT", Country: country, Lat: 48, Lon: 11},
		},
	}
	for _, c := range carriers {
		m.Carriers = append(m.Carriers, world.Carrier{Designator: c, Hub: "HUB"})
	}
	// Arrivals before the window set the airport's normal rate: six an hour.
	for i := 0; i < 36; i++ {
		arr := 8*60 + i*10
		c := carriers[i%len(carriers)]
		m.Flights = append(m.Flights, world.Flight{Carrier: c, Number: pad(100 + i), From: "OUT", To: "HUB", DepMin: arr - 75, ArrMin: arr, BlockMin: 75, KM: 400})
	}
	for i := 0; i < 12; i++ {
		arr := 14*60 + i*10
		c := carriers[i%len(carriers)]
		m.Flights = append(m.Flights, world.Flight{Carrier: c, Number: pad(500 + i), From: "OUT", To: "HUB", DepMin: arr - 75, ArrMin: arr, BlockMin: 75, KM: 400})
	}
	return m
}

func hubWeather() []Cell {
	return []Cell{{Name: "HUBA", Lat: 50, Lon: 8, RadiusKM: 50, Start: 14 * 60, End: 16 * 60, Factor: 0.5, Airports: []string{"HUB"}}}
}

func inWindow(m *world.Manifest) []world.Flight {
	var out []world.Flight
	for _, f := range m.Flights {
		if f.ArrMin >= 14*60 {
			out = append(out, f)
		}
	}
	return out
}

// A flight cancelled before the day runs takes no slot: an operations
// centre that thins its schedule ahead of a storm gives the flights that
// still fly the capacity the cancelled ones would have used.
func TestACancelledFlightTakesNoSlot(t *testing.T) {
	m := queueManifest("Germany", "LH")
	base := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	held := inWindow(m)
	cancel := map[string]string{}
	for i, f := range held {
		if i%2 == 1 {
			cancel[Key(f)] = "thinned ahead of the weather"
		}
	}
	thin := BuildWith(m, slotDay, calm, Options{Weather: hubWeather(), Cancel: cancel})
	var before, after int
	for i, f := range held {
		if i%2 == 1 {
			if pf := thin.Of(f); !pf.Cancelled || pf.ATFM != 0 || pf.CTOT != 0 {
				t.Errorf("%s was cancelled before the day and still holds a slot: %+v", Key(f), pf)
			}
			continue
		}
		before += base.Of(f).ATFM
		after += thin.Of(f).ATFM
		if thin.Of(f).ATFM > base.Of(f).ATFM {
			t.Errorf("%s waits %d with half the queue cancelled and %d with all of it", Key(f), thin.Of(f).ATFM, base.Of(f).ATFM)
		}
	}
	if before == 0 {
		t.Fatal("the regulation held nothing")
	}
	// Six an hour held to three: with every second flight gone the rest
	// arrive at the regulated rate and wait for nothing.
	if after != 0 {
		t.Errorf("the flights that still fly wait %d minutes; the cancelled ones' slots were theirs to take (waited %d with a full queue)", after, before)
	}
	if thin.Summary.SlotDelayMin >= base.Summary.SlotDelayMin {
		t.Errorf("slot delay %d thinned, %d not", thin.Summary.SlotDelayMin, base.Summary.SlotDelayMin)
	}
}

// A cancellation on the day gives the slot back. Under a European
// regulation the flights behind it are rationed again, first planned first
// served: each one still on the ground with time to make an earlier slot
// moves up, nobody moves back, and a flight that has left keeps its fate.
func TestReleaseMovesTheQueueUp(t *testing.T) {
	m := queueManifest("Germany", "LH", "AF")
	p := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	held := inWindow(m)
	before := map[string]Flight{}
	for _, f := range held {
		before[Key(f)] = p.Of(f)
	}
	// The third arrival of the window is cancelled at 1215, when the first
	// has left (off-block 1245 less nothing: it has no delay) -- pick a
	// moment before every held flight's off-block so only notice binds.
	gone := held[2]
	now := 11 * 60
	p.Update(gone, func(pf *Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" })
	moved := p.Release(m, gone, now)
	if len(moved) == 0 {
		t.Fatal("nobody moved up when a slot came back")
	}
	if pf := p.Of(gone); pf.ATFM != 0 || pf.CTOT != 0 || pf.Regulation != "" {
		t.Errorf("the cancelled flight still holds its slot: %+v", pf)
	}
	improved := 0
	for _, f := range held {
		if Key(f) == Key(gone) {
			continue
		}
		was, is := before[Key(f)], p.Of(f)
		if is.ATFM > was.ATFM {
			t.Errorf("%s waits longer after a cancellation: %d, was %d", Key(f), is.ATFM, was.ATFM)
		}
		if is.ATFM < was.ATFM {
			improved++
			if is.DepDelay != is.ATFM || (is.ATFM > 0 && is.CTOT != f.DepMin+is.ATFM+TaxiMin) {
				t.Errorf("%s improved to %+v: the delay and the take-off time must follow the slot", Key(f), is)
			}
		}
		if f.ArrMin < gone.ArrMin && is.ATFM != was.ATFM {
			t.Errorf("%s was ahead of the cancelled flight and moved: %d, was %d", Key(f), is.ATFM, was.ATFM)
		}
	}
	if improved == 0 || improved != len(moved) {
		t.Errorf("%d flights improved, %d reported", improved, len(moved))
	}
	_, regs, sum := p.Picture()
	total := 0
	for _, f := range held {
		total += p.Of(f).ATFM
	}
	if len(regs) != 1 || regs[0].DelayMin != total || sum.SlotDelayMin != total {
		t.Errorf("the regulation counts %d minutes and the summary %d; the flights wait %d", regs[0].DelayMin, sum.SlotDelayMin, total)
	}
}

// A flight that has already left, and one too close to its off-block to
// make an earlier slot, stay where they are when a slot comes back.
func TestReleaseLeavesFlightsThatCannotMove(t *testing.T) {
	m := queueManifest("Germany", "LH")
	p := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	held := inWindow(m)
	gone := held[1]
	next := held[2]
	was := p.Of(next)
	if was.ATFM == 0 {
		t.Fatalf("the flight behind the cancelled one is not held: %+v", was)
	}
	// Cancelled one minute before the next flight's new off-block: it
	// cannot be told and pushed back in time, so it keeps its slot.
	now := next.DepMin + was.DepDelay - 1
	p.Update(gone, func(pf *Flight) { pf.Cancelled = true })
	p.Release(m, gone, now)
	if is := p.Of(next); is.ATFM != was.ATFM || is.CTOT != was.CTOT {
		t.Errorf("a flight a minute from its off-block was improved: %+v, was %+v", is, was)
	}
}

// Under a ground delay program in the United States the slot of a
// cancelled flight is still the carrier's: its own later flight takes it,
// and only a slot the carrier cannot use goes to another carrier, which
// hands back the slot it leaves (compression).
func TestGroundDelayProgramKeepsTheSlotWithItsCarrier(t *testing.T) {
	m := queueManifest("United States", "UA", "AA")
	p := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	_, regs, _ := p.Picture()
	if len(regs) != 1 || regs[0].Mode != ModeGDP {
		t.Fatalf("a regulation in the United States is a ground delay program: %+v", regs)
	}
	held := inWindow(m)
	before := map[string]Flight{}
	for _, f := range held {
		before[Key(f)] = p.Of(f)
	}
	// held[0], [2], [4].. are UA; [1], [3].. are AA. UA cancels its second
	// flight of the window.
	gone := held[2]
	p.Update(gone, func(pf *Flight) { pf.Cancelled = true })
	p.Release(m, gone, 11*60)
	// UA's next flight takes the slot; the AA flight between them does not
	// move, although it was planned earlier.
	uaNext, aaBetween := held[4], held[3]
	if is, was := p.Of(uaNext), before[Key(uaNext)]; is.ATFM >= was.ATFM {
		t.Errorf("the carrier's own next flight did not take the slot it gave up: %d, was %d", is.ATFM, was.ATFM)
	}
	if is, was := p.Of(aaBetween), before[Key(aaBetween)]; is.ATFM != was.ATFM {
		t.Errorf("another carrier's flight took a slot its owner could use: %d, was %d", is.ATFM, was.ATFM)
	}
	for _, f := range held {
		if f.Carrier == "AA" && p.Of(f).ATFM > before[Key(f)].ATFM {
			t.Errorf("%s waits longer after compression", Key(f))
		}
	}
}

// When the carrier has no flight that can use the slot it gave up, the
// next carrier's flight moves into it and the first carrier owns the slot
// that flight left, so its own later flight can move up into that.
func TestCompressionPaysTheOwnerBack(t *testing.T) {
	m := queueManifest("United States", "UA", "AA")
	p := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	held := inWindow(m)
	before := map[string]Flight{}
	for _, f := range held {
		before[Key(f)] = p.Of(f)
	}
	// UA cancels its last flight but one; its last flight has gone too far
	// to be told (we freeze it by cancelling late for it but not for AA).
	// Simpler: UA cancels a flight and every later UA flight is cancelled
	// too, so only AA can use the slot.
	gone := held[6]
	var later []world.Flight
	for _, f := range held[7:] {
		if f.Carrier == "UA" {
			later = append(later, f)
		}
	}
	for _, f := range later {
		p.Update(f, func(pf *Flight) { pf.Cancelled = true })
		p.Release(m, f, 11*60)
	}
	mid := map[string]Flight{}
	for _, f := range held {
		mid[Key(f)] = p.Of(f)
	}
	p.Update(gone, func(pf *Flight) { pf.Cancelled = true })
	moved := p.Release(m, gone, 11*60)
	if len(moved) == 0 {
		t.Fatal("a slot nobody of its owner's can use went unused with another carrier's flights behind it")
	}
	for _, c := range moved {
		f := c.Flight
		if f.Carrier != "AA" {
			t.Errorf("%s moved; only AA has flights left behind the slot", Key(f))
		}
		if p.Of(f).ATFM >= mid[Key(f)].ATFM || c.Was.ATFM != mid[Key(f)].ATFM {
			t.Errorf("%s reported moved and waits as long", Key(f))
		}
	}
}

// REA: a flight ready at the door may take a slot at shorter notice than
// one that has to be told and boarded. With a slot given back ahead of it,
// a ready flight moves up where the same flight, not ready, stayed.
func TestReadyTakesASlotAtShortNotice(t *testing.T) {
	m := queueManifest("Germany", "LH")
	p := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	held := inWindow(m)
	gone, next := held[1], held[2]
	was := p.Of(next)
	// Too late for an ordinary improvement, early enough for a ready one.
	now := next.DepMin + was.DepDelay - ImproveNotice + 1
	p.Update(gone, func(pf *Flight) { pf.Cancelled = true })
	p.Release(m, gone, now)
	if is := p.Of(next); is.ATFM != was.ATFM {
		t.Fatalf("improved without notice: %+v", is)
	}
	moved := p.Ready(m, next, now)
	if is := p.Of(next); len(moved) == 0 || Key(moved[0].Flight) != Key(next) || moved[0].Was.ATFM != was.ATFM || is.ATFM >= was.ATFM {
		t.Errorf("a ready flight with a free slot ahead of it stayed: %+v (moved %d), was %+v", is, len(moved), was)
	}
	// With nothing given back, ready or not, there is nothing to take.
	q := BuildWith(m, slotDay, calm, Options{Weather: hubWeather()})
	last := held[len(held)-1]
	wasLast := q.Of(last)
	if moved := q.Ready(m, last, 11*60); len(moved) != 0 || q.Of(last).ATFM != wasLast.ATFM {
		t.Errorf("a ready flight was improved with no slot free: %+v", q.Of(last))
	}
}

// Weather that develops in the afternoon regulates the flights still on
// the ground. One that has left keeps its fate: it is not held again, and
// it does not depart a second time.
func TestWeatherAddedToTheDayLeavesDepartedFlightsAlone(t *testing.T) {
	m := queueManifest("Germany", "LH")
	p := BuildWith(m, slotDay, calm, Options{Calm: true})
	held := inWindow(m)
	// At 1330 the first four of the window have left (off-block 1245,
	// 1255, 1305, 1315).
	now := 13*60 + 30
	p.Inject(m, hubWeather(), nil, now)
	for _, f := range held {
		pf := p.Of(f)
		if f.DepMin <= now && (pf.ATFM != 0 || pf.DepDelay != 0) {
			t.Errorf("%s had left at %d and was held: %+v", Key(f), now, pf)
		}
	}
	total := 0
	for _, f := range held {
		total += p.Of(f).ATFM
	}
	if total == 0 {
		t.Error("the weather held nobody still on the ground")
	}
}
