package airport

import (
	"math"
	"testing"

	"github.com/adamf/wholesky/internal/refdata"
)

// The turn-cost formula at Frankfurt's index, against a turn built from
// Frankfurt's 2026 tariff: about 7,500 dollars for an A320 with 146
// passengers and 19,000 to 21,800 for a 777-300ER with 301.
func TestTurnCostAgainstFrankfurtsTariff(t *testing.T) {
	i := Index("FRA", refdata.Europe, 600)
	if i != 43.47 {
		t.Fatalf("Frankfurt's index %v", i)
	}
	if c := TurnCost(i, 73.5, 146); c < 7300 || c > 7900 {
		t.Errorf("an A320 at Frankfurt: %.0f", c)
	}
	if c := TurnCost(i, 351.5, 301); c < 19000 || c > 21800 {
		t.Errorf("a 777-300ER at Frankfurt: %.0f", c)
	}
	// Heathrow is the dearest, Kuala Lumpur the cheapest of the fifty.
	if lhr, kul := Index("LHR", refdata.Europe, 600), Index("KUL", refdata.Asia, 400); lhr/kul < 6 {
		t.Errorf("Heathrow %v, Kuala Lumpur %v", lhr, kul)
	}
	// A small European airport charges a low-cost carrier's price, and a
	// US airport's index is the airline's part of the review's.
	if small, big := Index("ZZZ", refdata.Europe, 12), Index("ZZZ", refdata.Europe, 300); small >= big {
		t.Errorf("small %v, large %v", small, big)
	}
	if jfk := Index("JFK", refdata.NorthAmerica, 500); math.Abs(jfk-41.85*0.6) > 0.01 {
		t.Errorf("JFK %v", jfk)
	}
}

// EUROCONTROL's formula at 70 euros: 0.85 euros a kilometre for an A320
// and 1.86 for a 777-300ER, which is 0.92 and 2.02 dollars.
func TestEnRoute(t *testing.T) {
	if c := EnRoute(1000, 73.5, refdata.Europe, refdata.Europe); c < 880 || c > 960 {
		t.Errorf("an A320 over 1,000 km of Europe: %.0f", c)
	}
	if c := EnRoute(1000, 351.5, refdata.Europe, refdata.Europe); c < 1950 || c > 2080 {
		t.Errorf("a 777-300ER over 1,000 km of Europe: %.0f", c)
	}
	if c := EnRoute(4000, 79, refdata.NorthAmerica, refdata.NorthAmerica); c != 0 {
		t.Errorf("a US domestic flight pays %.0f en route", c)
	}
	if atl, eu := EnRoute(6000, 254, refdata.Europe, refdata.NorthAmerica), EnRoute(6000, 254, refdata.Europe, refdata.Europe); atl != eu/2 {
		t.Errorf("a transatlantic flight pays half its distance at the European rate: %.0f against %.0f", atl, eu)
	}
}

func TestMinimumConnectingTimes(t *testing.T) {
	if m, pub := MCT("AMS", 600, II); m != 50 || !pub {
		t.Errorf("Amsterdam II %d (published %v)", m, pub)
	}
	if m, pub := MCT("VIE", 300, DD); m != 30 || !pub {
		t.Errorf("Vienna DD %d", m)
	}
	// Nothing filed: a small airport connects faster than a very large one.
	small, pub := MCT("ZZZ", 20, DD)
	large, _ := MCT("ZZZ", 900, DD)
	if pub || small >= large || small != 30 {
		t.Errorf("defaults: small %d, large %d, published %v", small, large, pub)
	}
	if Status(true, false) != ID || Status(false, true) != DI || Status(false, false) != DD || Status(true, true) != II {
		t.Error("status pairs")
	}
	if InterAirport("LHR", "LGW") != 180 || InterAirport("JFK", "EWR") != 240 {
		t.Error("between two airports")
	}
}

// The misconnection curve at an on-time outbound flight, from the table
// it was built to: with no slack and an on-time arrival 12 in 100 miss;
// with 15 minutes of slack and a 20-minute delay 27; with an hour of
// slack and an hour's delay 12.
func TestMisconnect(t *testing.T) {
	for _, c := range []struct {
		slack, late int
		want        float64
	}{{0, 0, 0.12}, {0, 10, 0.50}, {15, 20, 0.27}, {30, 45, 0.73}, {60, 60, 0.12}, {60, 0, 0}} {
		if got := Misconnect(c.slack, c.late, 0); math.Abs(got-c.want) > 0.011 {
			t.Errorf("slack %d, %d late: %.2f, want %.2f", c.slack, c.late, got, c.want)
		}
	}
	if held, left := Misconnect(0, 30, 20), Misconnect(0, 30, 0); held >= left {
		t.Error("a held departure saves nobody")
	}
}

// Slot values against the trades they were set from.
func TestSlotValues(t *testing.T) {
	// An early-morning long-haul pair at Heathrow: the record trades were
	// 60 to 75 million dollars.
	if v := SlotValue("LHR", 6*60, true); v < 60e6 || v > 75e6 {
		t.Errorf("Heathrow, early morning, long haul: %.0f", v)
	}
	// A short-haul pair at midday: about a tenth of that.
	if v := SlotValue("LHR", 13*60, false); v < 8e6 || v > 14e6 {
		t.Errorf("Heathrow, midday, short haul: %.0f", v)
	}
	// Washington National: five to six million a pair in 2011 to 2014,
	// narrow-body, through the day.
	if v := SlotValue("DCA", 9*60, false); v < 3e6 || v > 6.5e6 {
		t.Errorf("Washington National: %.0f", v)
	}
	if SlotValue("ATL", 9*60, false) != 0 || Coordinated("ATL") || !Coordinated("LHR") {
		t.Error("an airport without slots")
	}
}
