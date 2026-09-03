package sim

import (
	"context"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/wholesky/internal/world"
)

// A marketing carrier's stranded passengers are protected first on its own
// metal, then on the codeshares it markets -- sold under its own number, so
// the booking stays its own -- and only then interlined. A dead flight held
// under the marketing code is recognised as the operated flight it is.
func TestAlternativesIncludeTheCarriersOwnCodeshares(t *testing.T) {
	s := &Sim{Flights: map[string][]world.Flight{
		"DL": {
			{Carrier: "DL", Number: "0100", From: "ATL", To: "AEX", DepMin: 8 * 60, ArrMin: 9 * 60},   // dead, own metal
			{Carrier: "DL", Number: "0200", From: "ATL", To: "AEX", DepMin: 20 * 60, ArrMin: 21 * 60}, // own metal, late
		},
		"9E": {
			{Carrier: "9E", Number: "5168", From: "ATL", To: "AEX", DepMin: 11 * 60, ArrMin: 12 * 60, Marketing: "DL", MarketingNumber: "5168"}, // DL's codeshare
		},
		"AA": {
			{Carrier: "AA", Number: "0300", From: "ATL", To: "AEX", DepMin: 9 * 60, ArrMin: 10 * 60}, // interline, earliest of all
		},
	}}
	dead := pnr.Segment{Carrier: "DL", FlightNum: "0100", Board: "ATL", Off: "AEX", Depart: time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)}
	got, err := s.alternatives(context.Background(), dead)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, c := range got {
		order = append(order, c.Carrier+c.FlightNum)
	}
	want := []string{"DL0200", "DL5168", "AA0300"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("own metal, then own code on a partner, then interline: got %v want %v", order, want)
	}

	// The dead leg held under the marketing code: DL5168 is 9E5168, and the
	// operated flight must not be offered back as an alternative to itself.
	dead = pnr.Segment{Carrier: "DL", FlightNum: "5168", Board: "ATL", Off: "AEX", Depart: dead.Depart}
	got, err = s.alternatives(context.Background(), dead)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.Carrier == "9E" || (c.Carrier == "DL" && c.FlightNum == "5168") {
			t.Fatalf("the dead codeshare offered back to itself: %+v", got)
		}
		if c.Carrier == "DL" && c.FlightNum == "0100" && c.Depart.Day() != 27 {
			t.Fatalf("DL0100 at 08:00 is before the 11:00 dead departure and so is the next day's: %+v", c)
		}
	}
}
