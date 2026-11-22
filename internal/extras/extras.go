// Package extras is what the carriers sell beside the seat, and which of
// their passengers buy it: checked bags, a bag in the cabin, a seat chosen
// in advance, a seat with more legroom, boarding first, the internet, a
// snack box, a drink, a pillow and a blanket.
//
// A carrier's price list is a jetway catalogue (pkg/ancillary). The
// prices are the carriers' own as published or as a government report
// gives them, and each list below says where each price is from. Which
// passenger buys what is this package's synthetic behaviour. The shares
// are estimates, set so that what a carrier earns comes out near what the
// real carriers report:
//
//   - United States carriers took 7.3 billion dollars in baggage fees in
//     2024 against 185.3 billion in fares, 3.9 % (Bureau of Transportation
//     Statistics).
//   - At American in 2018, 45 % of checked bags paid a fee. The rest were
//     free by cabin, by standing with the carrier or by its credit card.
//     In 2023 United took 1.3 billion dollars in seat fees and 1.2 billion
//     in bag fees. (Senate Permanent Subcommittee on Investigations, "The
//     Sky's the Limit", November 2024.)
//   - Spirit took 53.01 dollars of fare and 68.57 of other revenue from
//     each passenger on each flight in 2023 (its annual report). Frontier
//     took 62 % of its revenue from extras in 2024, and Ryanair 23.40
//     euros a passenger in its 2024 year.
//
// An ultra-low-cost carrier is a different case in the books. The
// simulator's fares follow the Department of Transportation's fare
// report, and its ultra-low-cost carriers sell at about three quarters of
// a market's average fare. Spirit's fare and extras together were about
// that share of an average fare. So the price an ultra-low-cost
// passenger pays here stands for all that the passenger spends, and the
// extras are a part of it and are not added to it (Menu.AllIn). For every
// other carrier the fare report's fares leave the fees out, and the
// extras are more revenue.
package extras

import (
	"strings"
	"sync"

	"github.com/adamf/jetway/pkg/ancillary"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/equipment"
)

// The services, by the code each has in every carrier's catalogue. The
// codes are this simulator's. The industry's sub codes are filed with
// ATPCO and are not free to read.
const (
	Bag      = "BAG" // a checked bag
	CarryOn  = "CBN" // a bag in the cabin, where the carrier charges for one
	Seat     = "SEA" // a seat chosen in advance
	Legroom  = "LEG" // a seat with more legroom
	Priority = "PRI" // boarding first
	Internet = "NET"
	Snack    = "BOX" // a snack box or a fresh meal
	Drink    = "DRK" // an alcoholic drink
	Soft     = "SFT" // water or a soft drink, where the carrier charges for one
	Comfort  = "KIT" // a pillow and a blanket
)

// Names are the services in plain words, for a page that lists them.
var Names = map[string]string{
	Bag: "checked bags", CarryOn: "cabin bags", Seat: "seat assignments", Legroom: "seats with more legroom",
	Priority: "priority boarding", Internet: "internet", Snack: "food", Drink: "alcoholic drinks",
	Soft: "soft drinks", Comfort: "pillows and blankets",
}

// Menu is one carrier's price list and how its passengers buy from it.
type Menu struct {
	Catalogue *ancillary.Catalogue
	// AllIn says the price a passenger pays for this carrier's ticket in
	// the simulator stands for the fare and the extras together. What is
	// sold from the menu is then a part of the carrier's revenue, and is
	// not added to it.
	AllIn bool
	shares
}

// Revenue is a carrier's revenue from the fares it sold and the extras its
// passengers bought. The extras are added to the fares, except where the
// ticket price already stands for both.
func (m *Menu) Revenue(fares, extras int64) int64 {
	if m.AllIn {
		return fares
	}
	return fares + extras
}

// shares is how a carrier's passengers behave. Each is an estimate.
type shares struct {
	// checks is the share of parties that check a bag.
	checks float64
	// elite is the share of passengers whose standing with the carrier,
	// or its credit card, makes the first bag free. member is the share
	// who have the carrier's free loyalty account and no more.
	elite, member float64
	// prepaid is the share of bag fees paid before the airport.
	prepaid float64
	// The rest are the share of the passengers a price applies to who
	// pay it.
	carryOn, seat, legroom, priority, internet, snack, drink, soft, comfort float64
}

// Passenger is one passenger on one flight, as the menu needs to know
// them.
type Passenger struct {
	// Class is the booking class and Cabin the compartment: F, C or Y.
	Class, Cabin string
	// KM is the flight's distance, Minutes its scheduled time, and
	// International whether it crosses a border.
	KM, Minutes   int
	International bool
	// Bags is the checked bags this passenger brought. A party's bags
	// are all on its first passenger.
	Bags int
	// Continuing says the passenger came off a connecting flight. What
	// is paid once for a journey, the bags and the boarding, was paid at
	// its start.
	Continuing bool
}

// Sale is what a passenger bought of one service.
type Sale struct {
	Code  string
	Units int
	Cents int64
}

// Bags is what a party checks on a carrier: the weight of each bag in
// kilos, or nothing. u is the party's own number.
func (m *Menu) Bags(u uint64) []int {
	if draw(u, 11) >= m.checks {
		return nil
	}
	n := 1
	if draw(u, 12) < 0.5 {
		n = 2
	}
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, 12+int(draw(u, 13+uint64(i))*17))
	}
	return out
}

// Buy is what a passenger buys from the menu on one flight. u is the
// passenger's own number, so the answer is the same each time it is
// asked. A service that is part of the passenger's fare or standing is
// not a sale.
func (m *Menu) Buy(p Passenger, u uint64) []Sale {
	q := ancillary.Passenger{Cabin: p.Cabin, Class: p.Class, International: p.International, KM: p.KM, Minutes: p.Minutes}
	switch x := draw(u, 1); {
	case x < m.elite:
		q.Tier = "elite"
	case x < m.elite+m.member:
		q.Tier = "member"
	}
	var sales []Sale
	// A sale is the units the passenger pays for. A unit that the fare or
	// the passenger's standing covers is not one.
	buy := func(code string, units int, at ancillary.Point) {
		q.Point = at
		var s Sale
		for unit := 1; unit <= units; unit++ {
			quote, ok := m.Catalogue.Price(code, q, unit)
			if !ok {
				break
			}
			if quote.Amount > 0 {
				s.Units++
				s.Cents += quote.Amount
			}
		}
		if s.Units > 0 {
			s.Code = code
			sales = append(sales, s)
		}
	}
	take := func(code string, share float64, salt uint64, at ancillary.Point) {
		if share > 0 && draw(u, salt) < share {
			buy(code, 1, at)
		}
	}
	if p.Bags > 0 && !p.Continuing {
		at := ancillary.AtAirport
		if draw(u, 2) < m.prepaid {
			at = ancillary.AtBooking
		}
		buy(Bag, p.Bags, at)
	}
	if !p.Continuing {
		take(CarryOn, m.carryOn, 3, ancillary.AtBooking)
		take(Priority, m.priority, 6, ancillary.AtBooking)
	}
	// A passenger pays for more legroom or for a plain seat, not both.
	if m.legroom > 0 && draw(u, 4) < m.legroom {
		buy(Legroom, 1, ancillary.AtBooking)
	} else {
		take(Seat, m.seat, 5, ancillary.AtBooking)
	}
	take(Internet, m.internet, 7, ancillary.OnBoard)
	take(Snack, m.snack, 8, ancillary.OnBoard)
	take(Drink, m.drink, 9, ancillary.OnBoard)
	take(Soft, m.soft, 10, ancillary.OnBoard)
	take(Comfort, m.comfort, 16, ancillary.OnBoard)
	return sales
}

// draw is a number from 0 up to 1 made from a passenger's number and a
// salt, so that each decision is its own.
func draw(u, salt uint64) float64 {
	x := u + salt*0x9e3779b97f4a7c15
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x>>11) / (1 << 53)
}

var (
	menus    sync.Map // carrier -> *Menu
	usULCC   = map[string]bool{"NK": true, "F9": true, "G4": true, "SY": true, "MX": true, "XP": true}
	flat45   = map[string]bool{"DL": true, "AS": true, "HA": true}
	basic    = []string{"N", "S"} // the classes sold with no seat assignment
	front    = []string{"F", "C"}
	before   = []ancillary.Point{ancillary.AtBooking, ancillary.AtAirport, ancillary.AtGate}
	inFlight = []ancillary.Point{ancillary.OnBoard}
)

// For is a carrier's menu.
func For(carrier string) *Menu {
	carrier = strings.ToUpper(carrier)
	if m, ok := menus.Load(carrier); ok {
		return m.(*Menu)
	}
	var m *Menu
	switch {
	case carrier == "B6":
		m = jetBlue()
	case carrier == "WN":
		m = southwest()
	case equipment.ModelOf(carrier) == equipment.UltraLowCost && usULCC[carrier]:
		m = ultraLowCostUS()
	case equipment.ModelOf(carrier) == equipment.UltraLowCost:
		m = ultraLowCost()
	case equipment.ModelOf(carrier) == equipment.LowCost:
		m = lowCost()
	default:
		m = network(carrier)
	}
	m.Catalogue.Carrier = carrier
	menus.Store(carrier, m)
	return m
}

func usd(dollars float64) (int64, string) { return int64(dollars*100 + 0.5), "USD" }

func priced(service string, dollars float64, r ancillary.Rule) ancillary.Rule {
	r.Service = service
	r.Amount, r.Currency = usd(dollars)
	return r
}

func service(code string, rfic pnr.RFIC, perUnit bool) ancillary.Service {
	return ancillary.Service{Code: code, RFIC: rfic, Name: Names[code], PerUnit: perUnit}
}

var services = map[string]ancillary.Service{
	Bag:      service(Bag, pnr.RFICBaggage, true),
	CarryOn:  service(CarryOn, pnr.RFICBaggage, false),
	Seat:     service(Seat, pnr.RFICAir, false),
	Legroom:  service(Legroom, pnr.RFICAir, false),
	Priority: service(Priority, pnr.RFICAirport, false),
	Internet: service(Internet, pnr.RFICInflight, false),
	Snack:    service(Snack, pnr.RFICInflight, true),
	Drink:    service(Drink, pnr.RFICInflight, true),
	Soft:     service(Soft, pnr.RFICInflight, true),
	Comfort:  service(Comfort, pnr.RFICInflight, false),
}

func catalogue(codes []string, rules ...[]ancillary.Rule) *ancillary.Catalogue {
	c := &ancillary.Catalogue{}
	for _, code := range codes {
		c.Services = append(c.Services, services[code])
	}
	for _, rs := range rules {
		c.Rules = append(c.Rules, rs...)
	}
	return c
}

// bags is a checked bag price list: 2 bags free in the front cabins and
// for a passenger whose standing or card covers the party's bags, then
// the first and the second bag paid in advance and at the airport, then
// each bag after them.
func bags(firstAhead, first, secondAhead, second, more float64, longHaulFree bool) []ancillary.Rule {
	rules := []ancillary.Rule{
		{Service: Bag, Points: inFlight, NotOffered: true},
		{Service: Bag, Cabins: front, ToUnit: 2, Included: true},
		{Service: Bag, Tiers: []string{"elite"}, ToUnit: 2, Included: true},
	}
	if longHaulFree {
		// A long international flight carries the first bag in the fare,
		// except on the fares sold with nothing.
		rules = append(rules,
			priced(Bag, firstAhead, ancillary.Rule{Classes: basic, International: ancillary.Yes, MinKM: 3500, ToUnit: 1, Points: []ancillary.Point{ancillary.AtBooking}}),
			priced(Bag, first, ancillary.Rule{Classes: basic, International: ancillary.Yes, MinKM: 3500, ToUnit: 1}),
			ancillary.Rule{Service: Bag, International: ancillary.Yes, MinKM: 3500, ToUnit: 1, Included: true})
	}
	return append(rules,
		priced(Bag, firstAhead, ancillary.Rule{ToUnit: 1, Points: []ancillary.Point{ancillary.AtBooking}}),
		priced(Bag, first, ancillary.Rule{ToUnit: 1}),
		priced(Bag, secondAhead, ancillary.Rule{FromUnit: 2, ToUnit: 2, Points: []ancillary.Point{ancillary.AtBooking}}),
		priced(Bag, second, ancillary.Rule{FromUnit: 2, ToUnit: 2}),
		priced(Bag, more, ancillary.Rule{FromUnit: 3}))
}

// legroom is a seat with more legroom in the main cabin, by distance.
func legroom(short, medium, long float64) []ancillary.Rule {
	return []ancillary.Rule{
		{Service: Legroom, Cabins: front, NotOffered: true},
		priced(Legroom, short, ancillary.Rule{MaxKM: 1200, Points: before}),
		priced(Legroom, medium, ancillary.Rule{MinKM: 1201, MaxKM: 3500, Points: before}),
		priced(Legroom, long, ancillary.Rule{MinKM: 3501, Points: before}),
	}
}

// network is a network carrier's menu. The bag prices are those the
// large United States carriers put in force in April and May 2026, as
// consumer groups listed them: 50 dollars for the first bag at American
// and United, or 45 paid in advance, and 60 or 55 for the second; 45 and
// 55 at Delta and Alaska. The internet is free to a member of the
// carrier's loyalty scheme, which is free to join, at United, American,
// Delta and Alaska, and is sold to the rest. A snack box is 10 dollars
// and a spirit 11 on Delta's own menu. The seat prices are inside the
// ranges the Senate subcommittee's report gives for 2023: 19 to 319
// dollars for more legroom at United, up to 140 at American. The price of
// a plain seat on a fare sold without one is an estimate.
func network(carrier string) *Menu {
	bag := bags(45, 50, 55, 60, 200, true)
	if flat45[carrier] {
		bag = bags(45, 45, 55, 55, 200, true)
	}
	return &Menu{
		Catalogue: catalogue([]string{Bag, Seat, Legroom, Priority, Internet, Snack, Drink}, bag,
			[]ancillary.Rule{
				priced(Seat, 22, ancillary.Rule{Classes: basic, Points: before}),
				{Service: Seat, Points: before, Included: true},
			},
			legroom(39, 79, 149),
			[]ancillary.Rule{
				{Service: Priority, Cabins: front, Included: true},
				{Service: Priority, Tiers: []string{"elite"}, Included: true},
				priced(Priority, 24, ancillary.Rule{Points: before}),
				{Service: Internet, Tiers: []string{"member", "elite"}, Points: inFlight, Included: true},
				priced(Internet, 10, ancillary.Rule{Points: inFlight}),
				priced(Snack, 10, ancillary.Rule{MinMinutes: 150, Points: inFlight}),
				{Service: Drink, Cabins: front, Points: inFlight, Included: true},
				priced(Drink, 11, ancillary.Rule{Points: inFlight}),
			}),
		shares: shares{checks: 0.50, elite: 0.52, member: 0.36, prepaid: 0.70,
			seat: 0.45, legroom: 0.085, priority: 0.03, internet: 0.25, snack: 0.08, drink: 0.10},
	}
}

// jetBlue is JetBlue's menu. The internet is free to every passenger
// with no account, and so are snacks and soft drinks. A first bag is 39
// dollars outside its peak days, and 10 more inside a day of departure
// (as reported in 2026). A snack box is 9 or 10 dollars on its own menu.
// A blanket at 10 dollars is from a report of its menu and not from the
// carrier.
func jetBlue() *Menu {
	return &Menu{
		Catalogue: catalogue([]string{Bag, Seat, Legroom, Internet, Snack, Drink, Comfort}, bags(39, 49, 59, 69, 150, true),
			[]ancillary.Rule{
				priced(Seat, 18, ancillary.Rule{Classes: basic, Points: before}),
				{Service: Seat, Points: before, Included: true},
			},
			legroom(35, 70, 130),
			[]ancillary.Rule{
				{Service: Internet, Points: inFlight, Included: true},
				priced(Snack, 10, ancillary.Rule{MinMinutes: 150, Points: inFlight}),
				{Service: Drink, Cabins: front, Points: inFlight, Included: true},
				priced(Drink, 10, ancillary.Rule{Points: inFlight}),
				priced(Comfort, 10, ancillary.Rule{Cabins: []string{"Y"}, MinMinutes: 150, Points: inFlight}),
			}),
		shares: shares{checks: 0.50, elite: 0.25, member: 0.45, prepaid: 0.75,
			seat: 0.45, legroom: 0.085, internet: 0.25, snack: 0.08, drink: 0.10, comfort: 0.03},
	}
}

// southwest is Southwest's menu since it began to charge for bags in May
// 2025 and to assign seats in January 2026: 45 dollars for the first bag
// and 55 for the second (as reported in 2026), the internet free to a
// member and 8 dollars to the rest, drinks at its low prices, and no food
// for sale.
func southwest() *Menu {
	return &Menu{
		Catalogue: catalogue([]string{Bag, Seat, Legroom, Internet, Drink}, bags(45, 45, 55, 55, 150, false),
			[]ancillary.Rule{
				priced(Seat, 20, ancillary.Rule{Classes: basic, Points: before}),
				{Service: Seat, Points: before, Included: true},
			},
			legroom(30, 60, 110),
			[]ancillary.Rule{
				{Service: Internet, Tiers: []string{"member", "elite"}, Points: inFlight, Included: true},
				priced(Internet, 8, ancillary.Rule{Points: inFlight}),
				priced(Drink, 7, ancillary.Rule{Points: inFlight}),
			}),
		shares: shares{checks: 0.50, elite: 0.35, member: 0.40, prepaid: 0, seat: 0.45, legroom: 0.085, internet: 0.25, drink: 0.10},
	}
}

// ultraLowCostUS is the menu of a United States ultra-low-cost carrier.
// Everything is sold. The average first checked bag was 59 dollars at
// Frontier and 71.80 at Spirit in 2023, and a bag in the cabin up to 99
// (the Senate subcommittee's report). In 2026 Frontier's first bag was 69
// dollars at booking and 79 at the airport. Frontier has no internet on
// board. The seat, drink and snack prices are estimates.
func ultraLowCostUS() *Menu {
	return &Menu{AllIn: true,
		Catalogue: catalogue([]string{Bag, CarryOn, Seat, Legroom, Priority, Snack, Drink, Soft},
			[]ancillary.Rule{
				{Service: Bag, Points: inFlight, NotOffered: true},
				{Service: Bag, Tiers: []string{"elite"}, ToUnit: 1, Included: true},
				priced(Bag, 64, ancillary.Rule{ToUnit: 1, Points: []ancillary.Point{ancillary.AtBooking}}),
				priced(Bag, 79, ancillary.Rule{ToUnit: 1}),
				priced(Bag, 79, ancillary.Rule{FromUnit: 2, ToUnit: 2, Points: []ancillary.Point{ancillary.AtBooking}}),
				priced(Bag, 94, ancillary.Rule{FromUnit: 2, ToUnit: 2}),
				priced(Bag, 110, ancillary.Rule{FromUnit: 3}),
				priced(CarryOn, 59, ancillary.Rule{Points: before}),
				priced(Seat, 22, ancillary.Rule{Points: before}),
				priced(Legroom, 45, ancillary.Rule{MaxKM: 1200, Points: before}),
				priced(Legroom, 75, ancillary.Rule{MinKM: 1201, Points: before}),
				priced(Priority, 12, ancillary.Rule{Points: before}),
				priced(Snack, 9, ancillary.Rule{Points: inFlight}),
				priced(Drink, 11, ancillary.Rule{Points: inFlight}),
				priced(Soft, 4.5, ancillary.Rule{Points: inFlight}),
			}),
		shares: shares{checks: 0.40, elite: 0.03, prepaid: 0.80,
			carryOn: 0.50, seat: 0.55, legroom: 0.06, priority: 0.08, snack: 0.08, drink: 0.08, soft: 0.15},
	}
}

// ultraLowCost is the menu of an ultra-low-cost carrier outside the
// United States. Ryanair took 23.40 euros of extras from each passenger
// in its 2024 year, against an average fare of 49.80. The prices are
// estimates of that kind of list.
func ultraLowCost() *Menu {
	return &Menu{AllIn: true,
		Catalogue: catalogue([]string{Bag, CarryOn, Seat, Legroom, Snack, Drink, Soft},
			[]ancillary.Rule{
				{Service: Bag, Points: inFlight, NotOffered: true},
				priced(Bag, 32, ancillary.Rule{ToUnit: 1, Points: []ancillary.Point{ancillary.AtBooking}}),
				priced(Bag, 50, ancillary.Rule{ToUnit: 1}),
				priced(Bag, 45, ancillary.Rule{FromUnit: 2, Points: []ancillary.Point{ancillary.AtBooking}}),
				priced(Bag, 60, ancillary.Rule{FromUnit: 2}),
				priced(CarryOn, 18, ancillary.Rule{Points: before}),
				priced(Seat, 10, ancillary.Rule{Points: before}),
				priced(Legroom, 22, ancillary.Rule{Points: before}),
				priced(Snack, 8, ancillary.Rule{Points: inFlight}),
				priced(Drink, 7, ancillary.Rule{Points: inFlight}),
				priced(Soft, 3.5, ancillary.Rule{Points: inFlight}),
			}),
		shares: shares{checks: 0.30, prepaid: 0.85,
			carryOn: 0.35, seat: 0.45, legroom: 0.05, snack: 0.08, drink: 0.08, soft: 0.15},
	}
}

// lowCost is the menu of a low-cost carrier with no list of its own
// here. The prices are estimates.
func lowCost() *Menu {
	return &Menu{
		Catalogue: catalogue([]string{Bag, Seat, Legroom, Internet, Snack, Drink}, bags(38, 48, 50, 60, 120, false),
			[]ancillary.Rule{
				priced(Seat, 12, ancillary.Rule{Classes: basic, Points: before}),
				{Service: Seat, Points: before, Included: true},
			},
			legroom(25, 45, 90),
			[]ancillary.Rule{
				priced(Internet, 8, ancillary.Rule{Points: inFlight}),
				priced(Snack, 9, ancillary.Rule{Points: inFlight}),
				priced(Drink, 9, ancillary.Rule{Points: inFlight}),
			}),
		shares: shares{checks: 0.45, elite: 0.15, member: 0.40, prepaid: 0.75,
			seat: 0.45, legroom: 0.07, internet: 0.06, snack: 0.08, drink: 0.10},
	}
}
