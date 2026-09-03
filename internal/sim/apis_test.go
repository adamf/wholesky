package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/wholesky/internal/world"
)

// Advance passenger information, end to end: an international flight
// closes its door and the border control agency's node receives a PAXLST
// naming everyone on board, through the switch.
func TestBorderAgencyReceivesTheFlightCloseList(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()

	var f world.Flight
	found := false
	for _, c := range s.Manifest.Carriers {
		for _, g := range s.Flights[c.Designator] {
			if s.countryOf(g.From) != "" && s.countryOf(g.To) != "" && s.countryOf(g.From) != s.countryOf(g.To) {
				f, found = g, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Skip("the trimmed world has no international flight")
	}
	var locs []string
	for i := 0; i < 3; i++ {
		res, err := s.Book(ctx, f, "Y", 0, "BORDER"+string(rune('A'+i)))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	settleAll(t, s, locs)
	quiesce(t, s)

	tn := s.Tenants[f.Carrier]
	day := s.BookingDate
	if err := tn.SendPNL(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
	deadline := time.Now().Add(20 * time.Second)
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
	// Everyone checks in and boards; the door closes; the list goes.
	for _, l := range []string{"BORDERA", "BORDERB", "BORDERC"} {
		acc, err := tn.DCS.Accept(ctx, key, dcs.AcceptRequest{Surname: l})
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
	for s.Border.Lists() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the border agency never received the passenger list")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if s.Border.Persons() != 3 || s.Border.ByArrival()[f.To] != 3 {
		t.Fatalf("three travellers named to the arrival country: %d %v", s.Border.Persons(), s.Border.ByArrival())
	}
	sum, ok := tn.Summarise(f.Carrier+f.Number, f.From)
	if !ok || sum.APIS != 3 {
		t.Fatalf("the panel says what was sent: %+v", sum)
	}
}
