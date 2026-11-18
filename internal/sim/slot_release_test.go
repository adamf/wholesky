package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// busiestArrivals is the airport most of the world's flights land at.
func busiestArrivals(s *Sim) string {
	n := map[string]int{}
	for _, f := range s.Manifest.Flights {
		n[f.To]++
	}
	best := ""
	for ap, c := range n {
		if c > n[best] || (c == n[best] && ap < best) {
			best = ap
		}
	}
	return best
}

// A cancellation gives its slot back. Weather holds the arrivals at the
// world's busiest airport; one of the held flights is cancelled; the
// flights behind it in the regulation move up, the carriers that fly them
// read it on their tapes, and the Network Manager revises the slots it
// had already issued. Before, the cancelled flight kept its place in the
// queue and every flight behind it waited for an aircraft that never came.
func TestACancellationGivesItsSlotToTheFlightsBehind(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	hub := busiestArrivals(s)
	a := s.airports[hub]
	cell := dayplan.Cell{Name: hub + "T", Lat: a.Lat, Lon: a.Lon, Start: 0, End: 26 * 60, Factor: 0.3, Airports: []string{hub}}
	s.fate.Inject(s.Manifest, []dayplan.Cell{cell}, nil, 0)
	var held []world.Flight
	for _, f := range s.Manifest.Flights {
		if f.To == hub && len(s.fate.Slots(f)) > 0 {
			held = append(held, f)
		}
	}
	if len(held) < 6 {
		t.Skipf("only %d flights held at %s", len(held), hub)
	}
	// The first flight in the queue that waits, of a carrier this machine
	// runs: the flights behind it have its slot to gain.
	var gone world.Flight
	found := false
	for _, f := range s.Manifest.Flights {
		if f.To != hub || s.Tenants[f.Carrier] == nil {
			continue
		}
		if sl := s.fate.Slots(f); len(sl) > 0 && sl[0].Wait > 0 && (!found || sl[0].At < s.fate.Slots(gone)[0].At) {
			gone, found = f, true
		}
	}
	if !found {
		t.Skip("no held flight of a carrier this machine runs")
	}
	wait := func() int {
		n := 0
		for _, f := range held {
			if dayplan.Key(f) != dayplan.Key(gone) {
				n += s.fate.Of(f).ATFM
			}
		}
		return n
	}
	before := wait()
	s.fate.Update(gone, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, "cancelled by operations", "A" })
	changes := s.releaseSlotsAt(ctx, gone, "cancelled by operations", 0)
	if len(s.fate.Slots(gone)) != 0 {
		t.Errorf("the cancelled flight still holds %+v", s.fate.Slots(gone))
	}
	if after := wait(); after >= before || len(changes) == 0 {
		t.Fatalf("the flights behind waited %d minutes before the cancellation and %d after; %d moved", before, after, len(changes))
	}
	if s.SlotsImproved.Load() == 0 {
		t.Error("no carrier here was told its slot improved")
	}
	// The carrier whose flight moved reads it on the tape.
	said := false
	for _, c := range changes {
		for _, ev := range s.Airline.Tape(c.Flight.Carrier) {
			if ev.Kind == "incident" && (strings.Contains(ev.Text, "slot improved") || strings.Contains(ev.Text, "slot lifted")) {
				said = true
			}
		}
	}
	if !said {
		t.Error("no tape says a slot improved")
	}
	// A flight that already held its slot message hears the revision.
	if s.ANSP != nil {
		s.slotsImproved(ctx, changes, "the test says so", 26*60)
		deadline := time.Now().Add(15 * time.Second)
		for {
			heard := 0
			for _, c := range changes {
				if tn := s.Tenants[c.Flight.Carrier]; tn != nil {
					heard += tn.SlotsHeard()["SRM"] + tn.SlotsHeard()["SLC"]
				}
			}
			if heard > 0 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("operations never heard a revised slot")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// The lever: a seat cancels a held flight and the slot goes back with the
// cancellation's other messages. A flight cancelled twice gives back once.
func TestSeatCancellationReleasesTheSlot(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	hub := busiestArrivals(s)
	a := s.airports[hub]
	s.fate.Inject(s.Manifest, []dayplan.Cell{{Name: hub + "T", Lat: a.Lat, Lon: a.Lon, Start: 0, End: 26 * 60, Factor: 0.3, Airports: []string{hub}}}, nil, 0)
	pos, _ := seatWorld{s}.Clock()
	var f world.Flight
	found := false
	for _, g := range s.Manifest.Flights {
		if g.To == hub && s.Tenants[g.Carrier] != nil && len(s.fate.Slots(g)) > 0 && float64(g.DepMin+s.fate.Of(g).DepDelay) > pos+5 {
			f, found = g, true
			break
		}
	}
	if !found {
		t.Skip("no held flight still on the ground at this hour")
	}
	if _, err := (seatWorld{s}).Act(ctx, f.Carrier, airline.Action{Kind: "cancel", Flight: f.Carrier + f.Number, Board: f.From}); err != nil {
		t.Fatal(err)
	}
	if sl := s.fate.Slots(f); len(sl) != 0 {
		t.Errorf("a cancelled flight holds %+v", sl)
	}
	if again := s.fate.Release(s.Manifest, f, int(pos)); len(again) != 0 {
		t.Errorf("a second release moved %d flights", len(again))
	}
}
