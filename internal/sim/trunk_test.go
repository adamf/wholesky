package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

// Two switches joined by a trunk: carriers are homed on one or the other
// by hash, the distribution systems live on the first, and a booking on
// a carrier homed on the second crosses the trunk to sell and to answer.
// The fabric counts subscriber links without counting its own trunk.
func TestTwoSwitchesSettleABookingAcrossTheTrunk(t *testing.T) {
	s := bootWorld(t, Options{Switches: 2})
	ctx := context.Background()
	if len(s.Switches) != 2 {
		t.Fatalf("switches: %d", len(s.Switches))
	}
	first, second := s.Switches[0], s.Switches[1]
	if !strings.Contains(strings.Join(first.LivePeers(), ","), "1Y") || !strings.Contains(strings.Join(second.LivePeers(), ","), "1X") {
		t.Fatalf("trunk not up: first sees %v, second sees %v", first.LivePeers(), second.LivePeers())
	}
	// Every carrier is on exactly one switch, and both have some.
	homed := map[int]int{}
	var remote world.Carrier
	for _, c := range s.Manifest.Carriers {
		if _, ok := s.Tenants[c.Designator]; !ok {
			continue
		}
		h := homeSwitch(c.Designator, 2)
		homed[h]++
		if h == 1 && remote.Designator == "" && len(s.Flights[c.Designator]) > 0 {
			remote = c
		}
	}
	if homed[0] == 0 || homed[1] == 0 {
		t.Fatalf("carriers were not spread across the switches: %v", homed)
	}
	if remote.Designator == "" {
		t.Skip("no carrier with flights is homed on the second switch in this world")
	}
	if got, want := s.linksUp(), len(s.Tenants)+len(s.GDSes)+2; got < want {
		t.Errorf("links up %d, want at least %d subscriber links", got, want)
	}

	f := s.Flights[remote.Designator][0]
	res, err := s.Book(ctx, f, "Y", 0, "TRUNKED")
	if err != nil {
		t.Fatal(err)
	}
	settleAll(t, s, []string{res.PNR.RecordLocator})
	rec, err := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Segments) == 0 || rec.Segments[0].Status != "HK" {
		t.Fatalf("the sell across the trunk did not confirm: %+v", rec.Segments)
	}
	// The carrier holds the record: it heard the sell over its own switch.
	deadline := time.Now().Add(10 * time.Second)
	for {
		held, _ := s.Tenants[remote.Designator].Store.FindPNRsByFlight(ctx, f.Carrier+strings.TrimLeft(f.Number, "0"), rec.Segments[0].WireDate, 100)
		found := false
		for _, h := range held {
			for _, p := range h.Passengers {
				if p.Surname == "TRUNKED" {
					found = true
				}
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the carrier on the second switch never held the booking")
		}
		time.Sleep(50 * time.Millisecond)
	}
	quiesce(t, s)
}
