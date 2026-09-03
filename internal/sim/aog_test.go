package sim

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// An aircraft goes technical after check-in has opened: a smaller type takes
// the flight, departure control re-seats the cabin, the inventory's cabins
// shrink to the new aircraft, distribution hears an ASM EQT, and the live
// invariant check calls the result a downgauge, not an oversell.
func TestAircraftSubstitutionReseatsShrinksAndTellsDistribution(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	// A flight whose aircraft the fleet can substitute.
	var c world.Carrier
	var f world.Flight
	found := false
	for _, cc := range s.Manifest.Carriers {
		for _, ff := range s.Flights[cc.Designator] {
			if host.SmallerType(ff.Equipment) != "" {
				c, f, found = cc, ff, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("the trimmed world's fleet offers no substitutable aircraft")
	}
	day := s.BookingDate
	var locs []string
	for i := 0; i < 6; i++ {
		res, err := s.Book(ctx, f, "Y", 0, fmt.Sprintf("AOG%d", i))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	settleAll(t, s, locs)
	quiesce(t, s)
	tn := s.Tenants[c.Designator]
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	deadline := time.Now().Add(20 * time.Second)
	for {
		fl, err := tn.DCS.Flight(key)
		if err == nil && len(fl.Passengers) >= 6 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the name list never reached the airport")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for i := 0; i < 6; i++ {
		if _, err := tn.DCS.Accept(ctx, key, dcs.AcceptRequest{Surname: fmt.Sprintf("AOG%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := tn.DCS.Flight(key)
	typ, err := tn.Substitute(ctx, f, day)
	if err != nil {
		t.Fatal(err)
	}
	if typ == "" {
		t.Skip("the fleet offers nothing smaller than this aircraft")
	}
	after, _ := tn.DCS.Flight(key)
	if after.Equipment != typ || after.Cabin.Seats() >= before.Cabin.Seats() {
		t.Fatalf("the flight now flies the smaller %s: %s %d -> %s %d", typ, before.Equipment, before.Cabin.Seats(), after.Equipment, after.Cabin.Seats())
	}
	seated := 0
	for _, p := range after.Passengers {
		if p.Status == dcs.StatusAccepted && p.Seat != "" {
			if _, ok := after.Cabin.Has(p.Seat); !ok {
				t.Fatalf("%s holds a seat the %s does not have: %s", p.Surname, typ, p.Seat)
			}
			seated++
		}
	}
	if seated == 0 {
		t.Fatal("somebody should still be seated")
	}
	comps, ok := tn.Inventory.Capacity(c.Designator, f.Number, key.Date, f.From)
	if !ok {
		t.Fatal("capacity unknown after the substitution")
	}
	total := 0
	for _, n := range comps {
		total += n
	}
	if total != after.Cabin.Seats() {
		t.Fatalf("inventory cabins %d vs aircraft %d", total, after.Cabin.Seats())
	}
	if !tn.Downgauged(f.Number, f.From) {
		t.Fatal("the leg is marked downgauged for the invariant check")
	}
	deadline = time.Now().Add(15 * time.Second)
	for {
		msgs, _ := s.GDSStore.ListMessages(ctx, store.MessageFilter{Limit: 10000})
		heard := false
		for _, m := range msgs {
			if m.Direction == store.Inbound && strings.HasPrefix(m.Kind, "ASM") {
				full, err := s.GDSStore.GetMessage(ctx, m.ID)
				if err == nil && strings.Contains(string(full.Raw), "EQT") && strings.Contains(string(full.Raw), f.Carrier+f.Number) {
					heard = true
				}
			}
		}
		if heard {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("distribution never heard the equipment change")
		}
		time.Sleep(50 * time.Millisecond)
	}
	sum, ok := tn.Summarise(f.Carrier+f.Number, f.From)
	if !ok || !strings.Contains(sum.Substituted, typ) {
		t.Fatalf("the panel says what happened: %+v", sum.Substituted)
	}
}
