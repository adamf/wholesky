package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/wholesky/internal/world"
)

// The PNR push, end to end: an international flight's records go to the
// state's passenger information unit once as booked, and again at the door
// with each traveller's seat and bags, through the switch.
func TestStateReceivesThePNRPushBeforeAndAtTheDoor(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	f, found := internationalFlight(s)
	if !found {
		t.Skip("the trimmed world has no international flight")
	}
	var locs []string
	for i := 0; i < 3; i++ {
		res, err := s.Book(ctx, f, "Y", 0, "PUSH"+string(rune('A'+i)))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	settleAll(t, s, locs)
	quiesce(t, s)

	tn := s.Tenants[f.Carrier]
	day := s.BookingDate
	// Before the name list: the push is the reservations alone.
	if err := tn.PushPNRGOV(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for s.Border.Pushes() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the state never received the first push")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if s.Border.Records() != 3 || s.Border.CheckedIn() != 0 {
		t.Fatalf("three records as booked, none checked in: %d/%d", s.Border.Records(), s.Border.CheckedIn())
	}

	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	deadline = time.Now().Add(20 * time.Second)
	for {
		fl, err := tn.DCS.Flight(key)
		if err == nil && len(fl.Passengers) >= 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the name list never reached the airport")
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, l := range []string{"PUSHA", "PUSHB"} {
		acc, err := tn.DCS.Accept(ctx, key, dcs.AcceptRequest{Surname: l, Bags: []int{17}})
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range acc.Passengers {
			if _, err := tn.DCS.Board(ctx, key, p.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tn.Close(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for s.Border.Pushes() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("the state never received the door's push")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Three records again; two of them now carry a seat and a bag.
	if s.Border.Records() != 6 || s.Border.CheckedIn() != 2 {
		t.Fatalf("door push: records %d checked-in %d", s.Border.Records(), s.Border.CheckedIn())
	}
	sum, ok := tn.Summarise(f.Carrier+f.Number, f.From)
	if !ok || sum.PNRGOV != 3 {
		t.Fatalf("the panel says what was pushed: %+v", sum)
	}
}

// internationalFlight is the first flight in the world that crosses a
// border, for the stories a border agency takes part in.
func internationalFlight(s *Sim) (world.Flight, bool) {
	for _, c := range s.Manifest.Carriers {
		for _, g := range s.Flights[c.Designator] {
			if s.countryOf(g.From) != "" && s.countryOf(g.To) != "" && s.countryOf(g.From) != s.countryOf(g.To) {
				return g, true
			}
		}
	}
	return world.Flight{}, false
}
