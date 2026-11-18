// Package equipment is what a carrier's aircraft cost it and how long
// they stand on the ground.
//
// The numbers by type are the reference table's (internal/refdata): what
// a new aircraft is worth and rents for, what one ten years old rents
// for, what a block hour costs to maintain, and the manufacturer's
// turnaround. This package turns them into the things the day needs: a
// rent and a maintenance rate for an aircraft of a given age, the terms
// of a lease, and a turnaround built from its tasks -- passengers off,
// cabin, passengers on; bags off and on; fuel -- so the time an aircraft
// stands at a gate is the time its slowest task takes, and changes with
// the load, the fuel for the next leg and the way the carrier works.
package equipment

import (
	"hash/fnv"
	"math"

	"github.com/adamf/wholesky/internal/refdata"
)

// Model is how a carrier works its aircraft.
type Model string

const (
	// Network is a full-service carrier: one door at a bridge, a cleaned
	// and catered cabin, banks at its hubs.
	Network Model = "network"
	// LowCost turns on stairs at both doors with the crew tidying the
	// cabin and no galley exchange.
	LowCost Model = "lcc"
	// UltraLowCost is the same with denser cabins and tighter turns.
	UltraLowCost Model = "ulcc"
)

// ModelOf is a carrier's model from the reference table; a carrier the
// table does not name is a network carrier.
func ModelOf(carrier string) Model {
	switch refdata.CarrierModel(carrier) {
	case "ulcc":
		return UltraLowCost
	case "lcc":
		return LowCost
	}
	return Network
}

// Type is the reference table's row for a type code the world uses, with
// a stand-in built from the seat count when the table has none.
func Type(code string, seats int) refdata.Aircraft {
	if a, ok := refdata.AircraftType(code); ok {
		return a
	}
	// The nearest type by seats stands in.
	var best refdata.Aircraft
	for i, a := range refdata.AircraftTypes() {
		if i == 0 || math.Abs(float64(a.Seats-seats)) < math.Abs(float64(best.Seats-seats)) {
			best = a
		}
	}
	return best
}

// ageFactor is the share of its new value an aircraft keeps at an age, by
// body: appraisers' values at 5, 10, 15, 20 and 25 years (AVAC and IBA
// tables, read from the trade press).
func ageFactor(body string, age float64) float64 {
	curve := []float64{1.00, 0.74, 0.52, 0.33, 0.19, 0.10}
	if body == "W" {
		curve = []float64{1.00, 0.68, 0.47, 0.28, 0.14, 0.07}
	}
	if age <= 0 {
		return 1
	}
	i := int(age / 5)
	if i >= len(curve)-1 {
		return curve[len(curve)-1]
	}
	f := age/5 - float64(i)
	return curve[i] + (curve[i+1]-curve[i])*f
}

// Value is the market value of an aircraft of a type and age, in dollars.
// A type out of production has no new value; its value is worked back
// from what a ten-year-old one rents for.
func Value(a refdata.Aircraft, age float64) float64 {
	fresh := a.ValueNew * 1e6
	if fresh == 0 {
		// Rent at ten years is about 0.9 per cent of value a month, and
		// value at ten years about half of new.
		fresh = a.Lease10 * 1e3 / 0.009 / ageFactor(a.Body, 10)
	}
	return fresh * ageFactor(a.Body, age)
}

// Rent is the monthly dry lease rent of an aircraft of a type and age,
// in dollars: the table's rent for a new aircraft and for one of ten to
// twelve years, a straight line between, and four per cent a year less
// after that, because value falls faster than the lease rate factor
// rises.
func Rent(a refdata.Aircraft, age float64) float64 {
	old := a.Lease10 * 1e3
	fresh := a.LeaseNew * 1e3
	if fresh == 0 {
		fresh = old * 1.45
	}
	switch {
	case age <= 0:
		return fresh
	case age < 11:
		return fresh + (old-fresh)*age/11
	}
	return old * math.Pow(0.96, age-11)
}

// MaintenancePerBlockHour is the direct maintenance cost of a block hour
// on an aircraft of a type and age, in dollars. A new aircraft is on
// warranty and its engines are on their first run; from year eight to
// twelve it costs the table's mature rate; an old one costs half as much
// again.
func MaintenancePerBlockHour(a refdata.Aircraft, age float64) float64 {
	steps := []struct {
		to   float64
		mult float64
	}{{3, 0.60}, {6, 0.75}, {8, 0.90}, {13, 1.00}, {17, 1.12}, {21, 1.25}}
	mult := 1.40
	for _, s := range steps {
		if age < s.to {
			mult = s.mult
			break
		}
	}
	return a.Maintenance * mult
}

// Aircraft is one aircraft of a carrier's fleet.
type Aircraft struct {
	Tail   string  `json:"tail"`
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Seats  int     `json:"seats"`
	Age    float64 `json:"age_years"`
	Leased bool    `json:"leased"`
	Lessor string  `json:"lessor,omitempty"`
	// Rent is the monthly rent in dollars for a leased aircraft: the
	// market's when the lease was signed, fixed for the term. For an
	// owned one it is what the aircraft costs to own a month,
	// depreciation and interest.
	Rent float64 `json:"rent_month"`
	// TermLeft is the months the lease has to run.
	TermLeft int `json:"term_left_months,omitempty"`
	// Reserves is what a lessee pays the lessor each flight hour toward
	// the heavy checks and the engines' shop visits, in dollars.
	Reserves float64 `json:"reserves_flight_hour,omitempty"`
	Value    float64 `json:"value"`
	// Maintenance is the direct maintenance cost of a block hour.
	Maintenance float64 `json:"maintenance_block_hour"`
}

var lessors = []string{"AerCap", "SMBC Aviation Capital", "Avolon", "Air Lease", "BOC Aviation", "Aviation Capital Group", "DAE Capital", "ICBC Leasing"}

// New is the aircraft a world's schedule implies for a tail: its age and
// whether it is leased are drawn from the tail, so every machine and
// every day agree. A low-cost carrier's fleet is younger and mostly
// leased; a network carrier owns most of its own. About half the world's
// fleet is leased, low-cost carriers lease 60 per cent or more and the
// large US carriers 20 to 35 per cent.
func New(carrier, tail, code string, seats int) Aircraft {
	t := Type(code, seats)
	model := ModelOf(carrier)
	u := draw(carrier, tail, "age")
	maxAge, leasedShare := 22.0, 0.40
	if model != Network {
		maxAge, leasedShare = 14.0, 0.70
	}
	// More young aircraft than old: the square of a uniform draw.
	age := math.Floor(u*u*maxAge*10) / 10
	ac := Aircraft{Tail: tail, Type: t.IATA, Name: t.Name, Seats: seats, Age: age,
		Value: Value(t, age), Maintenance: MaintenancePerBlockHour(t, age)}
	if ac.Seats == 0 {
		ac.Seats = t.Seats
	}
	if draw(carrier, tail, "leased") < leasedShare {
		ac.Leased = true
		ac.Lessor = lessors[int(draw(carrier, tail, "lessor")*float64(len(lessors)))%len(lessors)]
		// The rent was fixed when the lease was signed, some years ago,
		// for the age the aircraft had then.
		into := math.Floor(draw(carrier, tail, "into") * math.Min(8, age+1))
		ac.Rent = Rent(t, age-into)
		ac.TermLeft = int(math.Max(6, (10-into)*12-draw(carrier, tail, "m")*12))
		// Cash reserves run at about 44 per cent of rent at 300 flight
		// hours a month: one worked example from a lessor's filing.
		ac.Reserves = math.Round(ac.Rent * 0.44 / 300)
	} else {
		// Depreciation over 25 years to a tenth of the value, and interest
		// at five per cent on what is left of it.
		ac.Rent = Value(t, 0)*0.9/25/12 + ac.Value*0.05/12
	}
	ac.Rent = math.Round(ac.Rent)
	return ac
}

func draw(parts ...string) float64 {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	v := h.Sum64()
	v ^= v >> 33
	v *= 0xff51afd7ed558ccd
	v ^= v >> 33
	return (float64(v>>11) + 0.5) / float64(1<<53)
}

// Offer is what a lessor asks for an aircraft today.
type Offer struct {
	Type    string  `json:"type"`
	Name    string  `json:"name"`
	Seats   int     `json:"seats"`
	Age     float64 `json:"age_years"`
	Rent    float64 `json:"rent_month"`
	Term    int     `json:"term_months"`
	Deposit float64 `json:"deposit"`
	// ACMI is the rate per block hour for the same aircraft with its
	// crew, maintenance and insurance, for a season: an estimate, because
	// no rate card is public.
	ACMI float64 `json:"acmi_block_hour"`
}

// Market is what the lessors offer: each type new, where it is still
// built, and at eight years old. cycle scales the rents: 1 is the market
// of the table (2024 to 2026, a shortage), 0.7 a glut like 2020.
func Market(cycle float64) []Offer {
	if cycle <= 0 {
		cycle = 1
	}
	var out []Offer
	for _, t := range refdata.AircraftTypes() {
		for _, age := range []float64{0, 8} {
			if age == 0 && t.LeaseNew == 0 {
				continue
			}
			rent := math.Round(Rent(t, age) * cycle)
			term := 120
			if t.Widebody() {
				term = 144
			}
			if age > 0 {
				term = 72
			}
			// A wet lease covers the rent, a crew, the maintenance and a
			// margin over about 250 hours a month.
			acmi := math.Round((rent/250+MaintenancePerBlockHour(t, age)+float64(t.Seats)*3.2+450)*1.15/10) * 10
			out = append(out, Offer{Type: t.IATA, Name: t.Name, Seats: t.Seats, Age: age, Rent: rent, Term: term, Deposit: rent * 2, ACMI: acmi})
		}
	}
	return out
}
