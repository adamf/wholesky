package extras

import (
	"math"
	"testing"

	"github.com/adamf/jetway/pkg/ancillary"
)

// The carriers' lists, price by price. Each price is the one the list's
// comment gives a source for.
func TestTheCarriersLists(t *testing.T) {
	main := ancillary.Passenger{Cabin: "Y", Class: "Q", KM: 1500, Minutes: 190}
	with := func(p ancillary.Passenger, tier string, at ancillary.Point) ancillary.Passenger {
		p.Tier, p.Point = tier, at
		return p
	}
	for _, c := range []struct {
		name, carrier, code string
		p                   ancillary.Passenger
		unit                int
		dollars             float64
		offered, included   bool
	}{
		// 50 dollars for the first bag at American and United, 45 paid
		// in advance; 60 and 55 for the second. 45 and 55 at Delta.
		{"American, first bag at the airport", "AA", Bag, with(main, "", ancillary.AtAirport), 1, 50, true, false},
		{"American, first bag in advance", "AA", Bag, with(main, "", ancillary.AtBooking), 1, 45, true, false},
		{"United, second bag at the airport", "UA", Bag, with(main, "", ancillary.AtAirport), 2, 60, true, false},
		{"United, second bag in advance", "UA", Bag, with(main, "", ancillary.AtBooking), 2, 55, true, false},
		{"Delta, first bag at the airport", "DL", Bag, with(main, "", ancillary.AtAirport), 1, 45, true, false},
		{"Delta, second bag", "DL", Bag, with(main, "", ancillary.AtAirport), 2, 55, true, false},
		{"United, a card holder's first bag", "UA", Bag, with(main, "elite", ancillary.AtAirport), 1, 0, true, true},
		{"United, first bag in business", "UA", Bag, with(ancillary.Passenger{Cabin: "C", Class: "J", KM: 1500}, "", ancillary.AtAirport), 1, 0, true, true},
		{"United, first bag to Europe", "UA", Bag, with(ancillary.Passenger{Cabin: "Y", Class: "Q", KM: 6000, International: true}, "", ancillary.AtAirport), 1, 0, true, true},
		{"United, first bag to Europe on a fare with nothing", "UA", Bag, with(ancillary.Passenger{Cabin: "Y", Class: "N", KM: 6000, International: true}, "", ancillary.AtAirport), 1, 50, true, false},
		// Southwest since May 2025, JetBlue outside its peak days.
		{"Southwest, first bag", "WN", Bag, with(main, "", ancillary.AtAirport), 1, 45, true, false},
		{"JetBlue, first bag in advance", "B6", Bag, with(main, "", ancillary.AtBooking), 1, 39, true, false},
		{"JetBlue, first bag at the airport", "B6", Bag, with(main, "", ancillary.AtAirport), 1, 49, true, false},
		// The internet: free to a member at United, sold to the rest.
		// Free to everybody at JetBlue. 8 dollars at Southwest. None at
		// Frontier.
		{"United, the internet for a member", "UA", Internet, with(main, "member", ancillary.OnBoard), 1, 0, true, true},
		{"United, the internet for a stranger", "UA", Internet, with(main, "", ancillary.OnBoard), 1, 10, true, false},
		{"JetBlue, the internet for a stranger", "B6", Internet, with(main, "", ancillary.OnBoard), 1, 0, true, true},
		{"Southwest, the internet for a stranger", "WN", Internet, with(main, "", ancillary.OnBoard), 1, 8, true, false},
		{"Southwest, the internet for a member", "WN", Internet, with(main, "member", ancillary.OnBoard), 1, 0, true, true},
		{"Frontier, the internet", "F9", Internet, with(main, "", ancillary.OnBoard), 1, 0, false, false},
		// What is on the cart.
		{"Delta, a snack box", "DL", Snack, with(main, "", ancillary.OnBoard), 1, 10, true, false},
		{"Delta, a snack box on a short flight", "DL", Snack, with(ancillary.Passenger{Cabin: "Y", Minutes: 90}, "", ancillary.OnBoard), 1, 0, false, false},
		{"Delta, a spirit", "DL", Drink, with(main, "", ancillary.OnBoard), 1, 11, true, false},
		{"Delta, a spirit in first", "DL", Drink, with(ancillary.Passenger{Cabin: "F"}, "", ancillary.OnBoard), 1, 0, true, true},
		{"Southwest, food", "WN", Snack, with(main, "", ancillary.OnBoard), 1, 0, false, false},
		{"JetBlue, a blanket", "B6", Comfort, with(main, "", ancillary.OnBoard), 1, 10, true, false},
		{"United, a blanket", "UA", Comfort, with(main, "", ancillary.OnBoard), 1, 0, false, false},
		{"Frontier, water", "F9", Soft, with(main, "", ancillary.OnBoard), 1, 4.5, true, false},
		{"United, water", "UA", Soft, with(main, "", ancillary.OnBoard), 1, 0, false, false},
		// An ultra-low-cost carrier charges for the cabin bag and for
		// every seat. A network carrier charges for a seat only on the
		// fares sold without one.
		{"Frontier, a bag in the cabin", "F9", CarryOn, with(main, "", ancillary.AtBooking), 1, 59, true, false},
		{"United, a bag in the cabin", "UA", CarryOn, with(main, "", ancillary.AtBooking), 1, 0, false, false},
		{"Frontier, first checked bag in advance", "F9", Bag, with(main, "", ancillary.AtBooking), 1, 64, true, false},
		{"Frontier, a seat", "F9", Seat, with(main, "", ancillary.AtBooking), 1, 22, true, false},
		{"United, a seat on an ordinary fare", "UA", Seat, with(main, "", ancillary.AtBooking), 1, 0, true, true},
		{"United, a seat on a fare with nothing", "UA", Seat, with(ancillary.Passenger{Cabin: "Y", Class: "N", KM: 1500}, "", ancillary.AtBooking), 1, 22, true, false},
		{"United, more legroom on 1,500 km", "UA", Legroom, with(main, "", ancillary.AtBooking), 1, 79, true, false},
	} {
		m := For(c.carrier)
		if err := m.Catalogue.Validate(); err != nil {
			t.Fatalf("%s: %v", c.carrier, err)
		}
		q, ok := m.Catalogue.Price(c.code, c.p, c.unit)
		if ok != c.offered {
			t.Errorf("%s: offered %v, want %v", c.name, ok, c.offered)
			continue
		}
		if ok && (q.Amount != int64(c.dollars*100+0.5) || q.Included != c.included) {
			t.Errorf("%s: %d cents, included %v; want %.2f dollars, included %v", c.name, q.Amount, q.Included, c.dollars, c.included)
		}
	}
	// An ultra-low-cost carrier's ticket price stands for the fare and
	// the extras together. Nobody else's does.
	for carrier, want := range map[string]bool{"F9": true, "NK": true, "FR": true, "UA": false, "B6": false, "WN": false, "BA": false} {
		if got := For(carrier).AllIn; got != want {
			t.Errorf("%s: the ticket price holds the extras: %v, want %v", carrier, got, want)
		}
	}
	if For("ua") != For("UA") {
		t.Error("a carrier's menu depends on how its code is written")
	}
}

// What one passenger buys is the same each time it is asked, comes from
// the carrier's own list, and follows the journey: bags and boarding are
// paid for once, where the journey begins.
func TestWhatAPassengerBuys(t *testing.T) {
	ua, f9 := For("UA"), For("F9")
	who := Passenger{Class: "Q", Cabin: "Y", KM: 1500, Minutes: 190, Bags: 2}
	total := func(sales []Sale, code string) (cents int64, units int) {
		for _, s := range sales {
			if s.Code == code {
				cents += s.Cents
				units += s.Units
			}
		}
		return
	}
	paidBags, freeBags, twice := 0, 0, 0
	for u := uint64(1); u <= 4000; u++ {
		a, b := ua.Buy(who, u), ua.Buy(who, u)
		if len(a) != len(b) {
			t.Fatalf("passenger %d bought %v and then %v", u, a, b)
		}
		cents, units := total(a, Bag)
		switch {
		case units == 0:
			freeBags++
		case units == 2 && (cents == 5000+6000 || cents == 4500+5500):
			paidBags++
		default:
			t.Fatalf("passenger %d paid %d cents for %d bags: want 2 at 50 and 60 dollars, or at 45 and 55", u, cents, units)
		}
		for _, s := range a {
			if _, ok := ua.Catalogue.Price(s.Code, ancillary.Passenger{Cabin: "Y", Class: "Q", KM: 1500, Minutes: 190, Point: ancillary.OnBoard}, 1); !ok {
				if _, ok := ua.Catalogue.Price(s.Code, ancillary.Passenger{Cabin: "Y", Class: "Q", KM: 1500, Minutes: 190, Point: ancillary.AtBooking}, 1); !ok {
					t.Fatalf("passenger %d bought %s, which United does not sell", u, s.Code)
				}
			}
			if s.Code == Seat {
				t.Fatalf("passenger %d paid for a seat on a fare that has one", u)
			}
		}
		if c1, _ := total(a, Legroom); c1 != 0 {
			if c2, _ := total(a, Seat); c2 != 0 {
				twice++
			}
		}
	}
	// 52 in 100 have their bags free by standing or card.
	if share := float64(freeBags) / 4000; share < 0.49 || share > 0.55 {
		t.Errorf("%.3f of United's passengers have their bags free; want 0.52", share)
	}
	if twice != 0 {
		t.Errorf("%d passengers paid for legroom and for a plain seat", twice)
	}
	// On a fare sold with no seat, a passenger pays for more legroom or
	// for a plain seat and never for both. 8.5 in 100 take the legroom,
	// and 45 in 100 of the rest a plain seat.
	bare := Passenger{Class: "N", Cabin: "Y", KM: 1500, Minutes: 190}
	leg, plain := 0, 0
	for u := uint64(1); u <= 8000; u++ {
		sales := ua.Buy(bare, u)
		l, _ := total(sales, Legroom)
		p, _ := total(sales, Seat)
		switch {
		case l != 0 && p != 0:
			t.Fatalf("passenger %d paid for legroom and for a plain seat", u)
		case l == 7900:
			leg++
		case p == 2200:
			plain++
		case l != 0 || p != 0:
			t.Fatalf("passenger %d paid %d for legroom and %d for a seat", u, l, p)
		}
	}
	if share := float64(leg) / 8000; math.Abs(share-0.085) > 0.01 {
		t.Errorf("%.3f pay for more legroom, want 0.085", share)
	}
	if share := float64(plain) / 8000; math.Abs(share-0.915*0.45) > 0.02 {
		t.Errorf("%.3f pay for a plain seat, want 0.412", share)
	}
	// A carrier's revenue: the extras are on top of the fares, except at
	// an ultra-low-cost carrier, whose ticket price stands for both.
	if got := ua.Revenue(1000, 82); got != 1082 {
		t.Errorf("United, 1000 of fares and 82 of extras: %d, want 1082", got)
	}
	if got := f9.Revenue(1000, 390); got != 1000 {
		t.Errorf("Frontier, 1000 of ticket prices of which 390 are extras: %d, want 1000", got)
	}
	// A passenger who came off a connecting flight has paid for the bags
	// and for the bag in the cabin already.
	on := who
	on.Continuing = true
	for u := uint64(1); u <= 2000; u++ {
		for _, s := range f9.Buy(on, u) {
			if s.Code == Bag || s.Code == CarryOn || s.Code == Priority {
				t.Fatalf("a continuing passenger paid again for %s", s.Code)
			}
		}
	}
	// Half of Frontier's passengers pay for a bag in the cabin, at 59
	// dollars: 29.50 each on average.
	var cabin int64
	for u := uint64(1); u <= 4000; u++ {
		c, _ := total(f9.Buy(who, u), CarryOn)
		cabin += c
	}
	if each := float64(cabin) / 100 / 4000; math.Abs(each-29.5) > 1.5 {
		t.Errorf("Frontier's passengers pay %.2f dollars each for a bag in the cabin; want 29.50", each)
	}
}

// Half of a network carrier's parties check a bag, 1 or 2 of them, and 4
// in 10 of an ultra-low-cost carrier's.
func TestWhatAPartyChecks(t *testing.T) {
	for carrier, want := range map[string]float64{"UA": 0.50, "F9": 0.40, "FR": 0.30} {
		m := For(carrier)
		parties, pieces := 0, 0
		for u := uint64(1); u <= 10000; u++ {
			b := m.Bags(u)
			if len(b) > 2 {
				t.Fatalf("%s: a party checked %d bags", carrier, len(b))
			}
			for _, kg := range b {
				if kg < 12 || kg > 28 {
					t.Fatalf("%s: a bag of %d kg", carrier, kg)
				}
			}
			if len(b) > 0 {
				parties++
				pieces += len(b)
			}
		}
		if share := float64(parties) / 10000; math.Abs(share-want) > 0.02 {
			t.Errorf("%s: %.3f of parties check a bag, want %.2f", carrier, share, want)
		}
		if each := float64(pieces) / float64(parties); math.Abs(each-1.5) > 0.05 {
			t.Errorf("%s: %.2f bags for each party that checks, want 1.5", carrier, each)
		}
	}
}
