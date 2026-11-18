package growth

import (
	"math"
	"testing"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/world"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// The S-curve: a carrier with two flights in three has more than two
// thirds of the travellers, and a cheaper carrier has more than its
// flights give it.
func TestShareFollowsFlightsAndFares(t *testing.T) {
	even := Share(Rival{Daily: 4, Level: 1}, []Rival{{Daily: 4, Level: 1}})
	if !near(even, 0.5, 1e-9) {
		t.Errorf("two carriers with the same flights and fares: %.3f", even)
	}
	// 6 flights against 3: 2^1.2 / (2^1.2 + 1) = 0.697.
	if got := Share(Rival{Daily: 6, Level: 1}, []Rival{{Daily: 3, Level: 1}}); !near(got, 0.697, 0.001) || got <= 2.0/3 {
		t.Errorf("two flights in three: %.3f of the travellers", got)
	}
	// A fifth cheaper with the same flights: 0.8^-0.8 = 1.195, so 0.544.
	if got := Share(Rival{Daily: 4, Level: 0.8}, []Rival{{Daily: 4, Level: 1}}); !near(got, 0.544, 0.001) {
		t.Errorf("a fifth cheaper: %.3f", got)
	}
	if Share(Rival{Daily: 3, Level: 1}, nil) != 1 || Share(Rival{}, []Rival{{Daily: 3, Level: 1}}) != 0 {
		t.Error("alone a carrier has the market, and with no flights it has none of it")
	}
}

// A new route fills over six months from 64 over 88.5 of its mature
// load, which is Spirit's Detroit to Philadelphia flight in 1996.
func TestRamp(t *testing.T) {
	if !near(Ramp(0), 64/88.5, 0.005) || Ramp(180) != 1 || Ramp(900) != 1 || !near(Ramp(90), (64/88.5+1)/2, 0.005) {
		t.Errorf("day 0 %.3f, day 90 %.3f, day 180 %.3f", Ramp(0), Ramp(90), Ramp(180))
	}
}

var a320 = refdata.Aircraft{IATA: "320", Body: "N", Seats: 180}

// A low-cost carrier puts four daily A320 round trips into a market of
// 1,500 travellers a day each way where a network carrier flies eight.
// Worked by hand below; the figures are the public ones the package
// names.
func TestProjectALowCostEntry(t *testing.T) {
	p := Proposal{Carrier: "F9", Model: equipment.LowCost, Level: 0.80, From: "DEN", To: "PHX", KM: 970, BlockMin: 115, TurnMin: 40, Aircraft: a320, Daily: 4}
	m := Market{Passengers: 1500, Measured: true, Fare: 20000, Rivals: []Rival{{Carrier: "UA", Model: equipment.Network, Daily: 8, Level: 1.06}}}
	c := Costs{Fuel: 280000, Maintenance: 150000, Airports: 180000, Navigation: 40000, Handling: 90000, Crew: 160000, Passenger: 700,
		Rent: 330000, Deposit: 660000}
	got := Project(p, m, c)
	// 8 flights, 1,440 seats, 15.3 block hours: two aircraft at 11.2 a day.
	if got.Flights != 8 || got.Seats != 1440 || !near(got.BlockHours, 15.33, 0.01) || got.Aircraft != 2 {
		t.Errorf("flights %d, seats %d, block hours %.2f, aircraft %d", got.Flights, got.Seats, got.BlockHours, got.Aircraft)
	}
	// The market: 3,000 travellers a day both ways, 28% more with the
	// first low-cost carrier in it.
	if !near(got.MarketNow, 3000, 0.1) || !near(got.MarketAfter, 3840, 0.1) {
		t.Errorf("market %.0f to %.0f", got.MarketNow, got.MarketAfter)
	}
	// The share: United answers with 21% off, to 0.8374 of the average.
	// 4^1.2 x 0.80^-0.8 = 6.3096; 8^1.2 x 0.8374^-0.8 = 13.976; 0.3110.
	if !near(got.Share, 0.3110, 0.002) {
		t.Errorf("share %.4f", got.Share)
	}
	// 3,840 x 0.311 = 1,194 travellers on 1,440 seats.
	if !near(got.Passengers, 1194, 8) || !near(got.LoadFactor, 0.829, 0.006) {
		t.Errorf("passengers %.0f, load factor %.3f", got.Passengers, got.LoadFactor)
	}
	// Its fare: the market's 200 dollars is the network carrier's at 1.06,
	// so 0.80 / 1.06 of it: 150.94.
	if got.Fare != 15094 {
		t.Errorf("fare %d", got.Fare)
	}
	// A day: 8 flights at 9,000 dollars, two aircraft at 330,000 a month,
	// and 7 dollars a ticket.
	monthly := 330000.0
	rent := int64(2 * monthly * 12 / 365 * 100)
	fixed := int64(8*900000) + rent
	if got.Lines["aircraft rent"] != rent || got.Lines["fuel"] != 8*280000 || got.Cost-got.Lines["selling"] != fixed {
		t.Errorf("cost lines %v", got.Lines)
	}
	if rev := int64(got.Passengers * 15094); got.Revenue != rev || got.Contribution != got.Revenue-got.Cost || got.Contribution <= 0 {
		t.Errorf("revenue %d, cost %d, contribution %d", got.Revenue, got.Cost, got.Contribution)
	}
	// It breaks even where the seats' margin covers the fixed cost.
	if want := float64(fixed) / float64(15094-700) / 1440; !near(got.BreakEven, want, 1e-6) || got.BreakEven >= got.LoadFactor {
		t.Errorf("break-even %.3f, want %.3f under the load factor %.3f", got.BreakEven, want, got.LoadFactor)
	}
	// To start: two deposits of two months' rent, no slots.
	if got.Start != 2*66000000 {
		t.Errorf("cash to start %d", got.Start)
	}
	// The first half-year earns less than six months at maturity, and the
	// start is paid back.
	if got.FirstHalf >= 180*got.Contribution || got.Payback <= 0 || got.Payback > 3*365 {
		t.Errorf("first 180 days %d against %d at maturity; payback %d days", got.FirstHalf, 180*got.Contribution, got.Payback)
	}
	if len(got.Notes) < 2 {
		t.Errorf("the notes say what was assumed: %v", got.Notes)
	}
}

// What changes the answer: a slot-controlled airport adds the price of
// the slots, a market too thin loses money and never pays back, a
// market nobody flies gives the entrant the travellers who want a
// nonstop, and a service with more travellers than seats says so.
func TestProjectTheOtherCases(t *testing.T) {
	p := Proposal{Carrier: "BA", Model: equipment.Network, Level: 1.06, From: "LHR", To: "EDI", KM: 530, BlockMin: 85, TurnMin: 45, Aircraft: a320, Daily: 3}
	c := Costs{Fuel: 200000, Maintenance: 120000, Airports: 300000, Navigation: 50000, Handling: 100000, Crew: 150000, Passenger: 900, Rent: 330000, Deposit: 660000, SlotFrom: 15e6}
	thick := Market{Passengers: 2500, Fare: 15000, Rivals: []Rival{{Carrier: "U2", Model: equipment.LowCost, Daily: 6, Level: 0.8}}}
	got := Project(p, thick, c)
	if got.Start != int64((1*660000+3*15e6)*100) {
		t.Errorf("three slot pairs at 15 million dollars and one deposit: %d", got.Start)
	}
	// A network carrier's entry enlarges the market a tenth, and nobody
	// cuts fares for it.
	if !near(got.MarketAfter, 5500, 0.1) || got.Measured {
		t.Errorf("market after %.0f, measured %v", got.MarketAfter, got.Measured)
	}
	thin := thick
	thin.Passengers = 150
	lost := Project(p, thin, c)
	if lost.Contribution >= 0 || lost.Payback != -1 || lost.BreakEven <= lost.LoadFactor {
		t.Errorf("a thin market: contribution %d, payback %d, break-even %.2f at a load of %.2f", lost.Contribution, lost.Payback, lost.BreakEven, lost.LoadFactor)
	}
	empty := Market{Passengers: 400, Measured: true, Fare: 15000}
	first := Project(p, empty, c)
	if first.Share != 1 || !near(first.MarketAfter, 800*Nonstop, 0.1) {
		t.Errorf("nobody flies it: share %.2f of %.0f", first.Share, first.MarketAfter)
	}
	p.Daily = 1
	full := Project(p, thick, c)
	if !near(full.LoadFactor, Ceiling, 1e-9) {
		t.Errorf("more travellers than seats: load factor %.3f", full.LoadFactor)
	}
	// Aircraft: the 2019 block hours a day by kind of carrier, and a
	// network carrier's four daily round trips of 85 minutes are 11.3
	// hours, which is more than one of its aircraft flies.
	if Utilisation(equipment.Network, false) != 10.8 || Utilisation(equipment.LowCost, false) != 11.2 || Utilisation(equipment.UltraLowCost, false) != 11.9 || Utilisation(equipment.Network, true) != 13.0 {
		t.Error("utilisation is not the 2019 Form 41 figures")
	}
	p.Daily = 4
	if four := Project(p, thick, c); four.Aircraft != 2 {
		t.Errorf("four daily round trips of 85 minutes: %d aircraft", four.Aircraft)
	}
	if none := Project(Proposal{}, thick, c); none.Seats != 0 || none.Revenue != 0 {
		t.Errorf("no service: %+v", none)
	}
}

// A book across days: each day's profit goes to the cash, an order takes
// its start out and is refused when the cash is not there, and the trend
// is the days summed.
func TestBook(t *testing.T) {
	b := &Book{Opening: 1_000_000_00, Cash: 1_000_000_00}
	b.Close(Day{Day: 1, Revenue: 500_000_00, Cost: 450_000_00, Passengers: 4000, Seats: 5000, ASK: 5_000_000, RPK: 4_000_000})
	b.Close(Day{Day: 2, Revenue: 520_000_00, Cost: 600_000_00, Passengers: 4200, Seats: 5500, ASK: 5_500_000, RPK: 4_200_000})
	if b.Cash != 1_000_000_00+50_000_00-80_000_00 || len(b.Days) != 2 || b.Days[1].Profit != -80_000_00 {
		t.Fatalf("cash %d, days %+v", b.Cash, b.Days)
	}
	// The same day closed again replaces the first count.
	b.Close(Day{Day: 2, Revenue: 520_000_00, Cost: 500_000_00, ASK: 5_500_000, RPK: 4_200_000})
	if b.Cash != 1_000_000_00+50_000_00+20_000_00 || len(b.Days) != 2 {
		t.Fatalf("after a day closed twice: cash %d, %d days", b.Cash, len(b.Days))
	}
	tr := b.Trend(0)
	if tr.Days != 2 || tr.Profit != 70_000_00 || !near(tr.Margin, 70.0/1020, 1e-9) || !near(tr.LoadFactor, 8.2/10.5, 1e-9) ||
		!near(tr.RASK, 1020_000_00/10.5e6, 1e-9) || !near(tr.CASK, 950_000_00/10.5e6, 1e-9) || !near(tr.Yield, 1020_000_00/8.2e6, 1e-9) || !near(tr.Growth, 1.1, 1e-9) || tr.Runway != 0 {
		t.Errorf("trend %+v", tr)
	}
	if err := b.Place(Order{ID: "a", Paid: 2_000_000_00}); err == nil {
		t.Error("an order for more than the cash was placed")
	}
	if err := b.Place(Order{ID: "b", Due: 50, Paid: 600_000_00}); err != nil || b.Cash != 1_070_000_00-600_000_00 {
		t.Errorf("order: %v, cash %d", err, b.Cash)
	}
	if err := b.Place(Order{ID: "c", Due: 20, Paid: 100_00}); err != nil || b.Orders[0].ID != "c" {
		t.Errorf("orders come due in order: %+v", b.Orders)
	}
	// A carrier losing money has a runway.
	lose := &Book{Cash: 900_000_00}
	lose.Close(Day{Day: 1, Revenue: 100_000_00, Cost: 130_000_00})
	if tr := lose.Trend(0); tr.Runway != int(870_000_00/30_000_00) {
		t.Errorf("runway %d days", tr.Runway)
	}
	// Four hundred days are kept.
	for d := 0; d < 500; d++ {
		lose.Close(Day{Day: 10 + d})
	}
	if len(lose.Days) != keep || lose.Days[len(lose.Days)-1].Day != 509 {
		t.Errorf("%d days kept, the last is %d", len(lose.Days), lose.Days[len(lose.Days)-1].Day)
	}
}

// The flights an order adds: five daily round trips on two aircraft, out
// and back, each aircraft leaving when it is back and turned, on flight
// numbers and tails nobody has.
func TestScheduleOfAnOrder(t *testing.T) {
	existing := []world.Flight{
		{Carrier: "F9", Number: "0410", From: "DEN", To: "LAS", DepMin: 7 * 60, Tail: "WF9001"},
		{Carrier: "F9", Number: "0411", From: "LAS", To: "DEN", DepMin: 10 * 60, Tail: "WF9001"},
		{Carrier: "F9", Number: "0412", From: "DEN", To: "MCO", DepMin: 6*60 + 30, Tail: "WF9002"},
	}
	taken := map[string]bool{"WF9001": true, "WF9002": true, "GF9001": true}
	got := Schedule("F9", Order{From: "DEN", To: "PHX", Type: "320", Daily: 5, Aircraft: 2}, existing, taken, 970, 115, 40, 180)
	if len(got) != 10 {
		t.Fatalf("%d flights", len(got))
	}
	seen := map[string]bool{"0410": true, "0411": true, "0412": true}
	last := map[string]world.Flight{}
	for i, f := range got {
		if seen[f.Number] || len(f.Number) != 4 {
			t.Errorf("flight number %s is taken or not four digits", f.Number)
		}
		seen[f.Number] = true
		if f.Tail != "GF9002" && f.Tail != "GF9003" {
			t.Errorf("tail %s: GF9001 was taken", f.Tail)
		}
		if f.ArrMin-f.DepMin != 115 || f.BlockMin != 115 || f.Seats != 180 || f.KM != 970 || f.Equipment != "320" || f.Carrier != "F9" {
			t.Errorf("flight %+v", f)
		}
		if out := i%2 == 0; out != (f.From == "DEN") {
			t.Errorf("flight %d of the order leaves %s", i, f.From)
		}
		// An aircraft is where its last flight left it, and has had its
		// turn.
		if prev, ok := last[f.Tail]; ok && (prev.To != f.From || f.DepMin < prev.ArrMin+40) {
			t.Errorf("%s flies %s-%s at %d after arriving %s at %d", f.Tail, f.From, f.To, f.DepMin, prev.To, prev.ArrMin)
		}
		last[f.Tail] = f
	}
	// The day starts when the carrier's day at the station starts: 6.30.
	if got[0].DepMin != 6*60+30 || got[2].DepMin != 6*60+30+45 {
		t.Errorf("first departures at %d and %d", got[0].DepMin, got[2].DepMin)
	}
	// A day has room for so many. One aircraft that leaves at 6.30 on a
	// seven-hour flight is back at 21.30, and its next departure would be
	// after ten at night: one round trip, however many were ordered.
	if long := Schedule("F9", Order{From: "DEN", To: "HNL", Type: "320", Daily: 12, Aircraft: 1}, existing, map[string]bool{}, 5400, 420, 60, 180); len(long) != 2 {
		t.Errorf("an aircraft flew %d legs of seven hours in a day", len(long))
	}
}
