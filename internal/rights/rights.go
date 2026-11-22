// Package rights is what a disrupted passenger is owed, by the law that
// covers the flight. Regulation (EC) 261/2004 and the United Kingdom's
// retained copy of it pay fixed compensation by distance for a
// cancellation announced on the day and for an arrival three hours late
// (the Court of Justice's Sturgeon reading), unless extraordinary
// circumstances caused it -- weather and air traffic control do, a crew
// that times out or an aircraft that goes technical does not -- and owe
// care while the passenger waits: meals by the delay's length, a hotel
// when the wait runs overnight. United States law pays no compensation
// for a delay or a cancellation; 14 CFR 260 owes an automatic refund
// when the passenger does not take the alternative, and the large
// carriers' own customer service commitments, published on the
// Department of Transportation's dashboard, give meals and hotels when
// the carrier caused it.
//
// The amounts of compensation and the thresholds are the regulations'
// (Articles 5 to 9 of the adopted text, read on legislation.gov.uk; 14
// CFR 260.2 from the eCFR). What a meal or a hotel costs, and the
// exchange rates, are the world's numbers.
package rights

import (
	"math"
)

// Regime is the passenger rights law that covers a flight.
type Regime string

const (
	EU261 Regime = "EU261"
	UK261 Regime = "UK261"
	US    Regime = "US"
	None  Regime = "none"
)

// Cause is why a flight was disrupted, as the law weighs it.
type Cause string

const (
	Weather   Cause = "weather"
	ATC       Cause = "air traffic control"
	Crew      Cause = "crew"
	Technical Cause = "technical"
	Carrier   Cause = "carrier"
)

// Extraordinary says the carrier did not control the cause: Article 5(3)
// relieves it of compensation, though not of care.
func (c Cause) Extraordinary() bool { return c == Weather || c == ATC }

// eea are the countries where 261/2004 applies: the European Union, the
// European Economic Area and Switzerland, by the names the world's airport
// data uses.
var eea = map[string]bool{
	"Austria": true, "Belgium": true, "Bulgaria": true, "Croatia": true, "Cyprus": true, "Czech Republic": true,
	"Denmark": true, "Estonia": true, "Finland": true, "France": true, "Germany": true, "Greece": true,
	"Hungary": true, "Ireland": true, "Italy": true, "Latvia": true, "Lithuania": true, "Luxembourg": true,
	"Malta": true, "Netherlands": true, "Poland": true, "Portugal": true, "Romania": true, "Slovakia": true,
	"Slovenia": true, "Spain": true, "Sweden": true, "Iceland": true, "Liechtenstein": true, "Norway": true,
	"Switzerland": true,
}

// Of is the regime that covers a flight from one country to another by a
// carrier of a third: Article 3, departing a member state, or arriving in
// one on a carrier of one; the UK's copy for the United Kingdom; United
// States law for a flight touching the United States.
func Of(fromCountry, toCountry, carrierCountry string) Regime {
	switch {
	case eea[fromCountry]:
		return EU261
	case fromCountry == "United Kingdom":
		return UK261
	case eea[toCountry] && eea[carrierCountry]:
		return EU261
	case toCountry == "United Kingdom" && (carrierCountry == "United Kingdom" || eea[carrierCountry]):
		return UK261
	case fromCountry == "United States" || toCountry == "United States":
		return US
	}
	return None
}

// Intra says both ends lie inside the regime's territory: an
// intra-community flight for Article 7's middle band, a domestic one for
// the United States refund threshold.
func Intra(r Regime, fromCountry, toCountry string) bool {
	switch r {
	case EU261:
		return eea[fromCountry] && eea[toCountry]
	case UK261:
		return fromCountry == "United Kingdom" && toCountry == "United Kingdom"
	case US:
		return fromCountry == "United States" && toCountry == "United States"
	}
	return false
}

// The world's prices, in US cents: a euro and a pound in dollars, a meal
// voucher, a night in a hotel with the transport to it.
const (
	EURinUSD   = 1.08
	GBPinUSD   = 1.27
	mealCents  = 1500
	hotelCents = 15000
)

// band is Article 7's distance band: 0 up to 1500 km, 1 to 3500 km or
// intra-community beyond, 2 the rest.
func band(km int, intra bool) int {
	switch {
	case km <= 1500:
		return 0
	case km <= 3500 || intra:
		return 1
	}
	return 2
}

// compensation is Article 7(1) in US cents.
func compensation(r Regime, b int) int64 {
	switch r {
	case EU261:
		return int64(math.Round([]float64{250, 400, 600}[b] * EURinUSD * 100))
	case UK261:
		return int64(math.Round([]float64{220, 350, 520}[b] * GBPinUSD * 100))
	}
	return 0
}

// careAfter is Article 6(1)'s wait before meals are owed, by band.
func careAfter(b int) int { return []int{120, 180, 240}[b] }

// Disruption is one passenger's disrupted journey.
type Disruption struct {
	Regime Regime
	KM     int
	// Intra says both ends are in the regime's territory, which keeps a
	// long flight in the middle band.
	Intra     bool
	Cause     Cause
	Cancelled bool
	// ArrDelay is how late the passenger arrives, in minutes: the flight's
	// own delay, or the alternative's after a cancellation.
	ArrDelay int
	// DepDelay is how late the passenger leaves, for the care thresholds.
	DepDelay int
	// Overnight says the wait runs past the night.
	Overnight bool
	// Rerouted says a cancelled passenger was offered an alternative.
	Rerouted bool
	// Domestic marks a US domestic itinerary, for the refund rule's
	// threshold.
	Domestic bool
}

// Owed is what a passenger is owed, in US cents.
type Owed struct {
	Compensation int64 `json:"compensation"`
	Care         int64 `json:"care"`
	// RefundRight says the passenger may take the fare back instead of
	// flying: a cancellation anywhere, or a significant delay under 14
	// CFR 260 (three hours domestic, six international).
	RefundRight bool `json:"refund_right"`
}

// For is what the law owes one passenger for the disruption.
func For(d Disruption) Owed {
	var o Owed
	b := band(d.KM, d.Intra)
	switch d.Regime {
	case EU261, UK261:
		if !d.Cause.Extraordinary() {
			owed := false
			if d.Cancelled {
				// Article 5(1)(c)(iii): announced on the day, compensation is
				// owed unless the alternative arrives within two hours.
				owed = !d.Rerouted || d.ArrDelay >= 120
			} else {
				owed = d.ArrDelay >= 180
			}
			if owed {
				o.Compensation = compensation(d.Regime, b)
				// Article 7(2): half when the alternative arrives within two,
				// three or four hours, by band.
				if d.Cancelled && d.Rerouted && d.ArrDelay <= []int{120, 180, 240}[b] {
					o.Compensation /= 2
				}
			}
		}
		// Article 9 care is owed whatever the cause.
		if d.Cancelled || d.DepDelay >= careAfter(b) {
			o.Care += mealCents
		}
		if d.Overnight {
			o.Care += hotelCents
		}
		o.RefundRight = d.Cancelled || d.DepDelay >= 300
	case US:
		threshold := 360
		if d.Domestic {
			threshold = 180
		}
		o.RefundRight = d.Cancelled || d.ArrDelay >= threshold
		if !d.Cause.Extraordinary() {
			if d.Cancelled || d.DepDelay >= 180 {
				o.Care += mealCents
			}
			if d.Overnight {
				o.Care += hotelCents
			}
		}
	default:
		o.RefundRight = d.Cancelled
	}
	return o
}

// Bumped is one passenger who held a confirmed seat, came for the flight
// and was denied boarding against their will.
type Bumped struct {
	Regime Regime
	KM     int
	// Intra says both ends are in the regime's territory. For United
	// States law that is a domestic flight.
	Intra bool
	// Outbound says the flight leaves from the regime's territory. Part
	// 250 does not cover a flight that leaves from a foreign airport.
	Outbound bool
	// Fare is the passenger's one-way fare to the destination, in US
	// cents.
	Fare int64
	// Wait is how much later than planned the alternative arrives, in
	// minutes, and Overnight says the passenger waits the night for it.
	Wait      int
	Overnight bool
	// Smaller says a smaller aircraft took the flight.
	Smaller bool
}

// The limits of 14 CFR 250.5 as amended on 24 October 2024 (89 FR 84818),
// in US cents.
const (
	bumpedLimit  = 107500
	bumpedDouble = 215000
)

// DeniedBoarding is what the law owes a passenger denied boarding
// against their will.
//
// United States law, 14 CFR 250.5: nothing when the alternative arrives
// within 1 hour of the planned arrival. Twice the fare, to a limit of
// $1,075, when it arrives less than 2 hours after it on a domestic
// flight, or less than 4 hours on a flight to another country. 4 times
// the fare, to a limit of $2,150, when it arrives later than that.
// Nothing is owed when a smaller aircraft took the flight (250.6(b)), or
// on a flight that leaves from a foreign airport.
//
// Regulation 261, Article 4(3): the compensation of Article 7 at once, by
// distance, halved when the alternative arrives within 2, 3 or 4 hours.
// The reason for the denial does not matter.
//
// Care is owed under both while the passenger waits.
func DeniedBoarding(b Bumped) Owed {
	var o Owed
	band := band(b.KM, b.Intra)
	switch b.Regime {
	case EU261, UK261:
		o.Compensation = compensation(b.Regime, band)
		if b.Wait <= []int{120, 180, 240}[band] {
			o.Compensation /= 2
		}
		o.Care = mealCents
		if b.Overnight {
			o.Care += hotelCents
		}
		o.RefundRight = true
	case US:
		o.RefundRight = true
		if b.Wait >= 180 {
			o.Care += mealCents
		}
		if b.Overnight {
			o.Care += hotelCents
		}
		if b.Smaller || !b.Outbound || b.Wait <= 60 {
			break
		}
		long := 120
		if !b.Intra {
			long = 240
		}
		if b.Wait < long {
			o.Compensation = min(2*b.Fare, bumpedLimit)
		} else {
			o.Compensation = min(4*b.Fare, bumpedDouble)
		}
	default:
		o.RefundRight = true
	}
	return o
}

// Offer is what a carrier offers a passenger to give up a seat by
// agreement, in US cents: twice the passenger's one-way fare, to the
// limit of the law's first tier. No rule sets it and no public figure
// says what volunteers accept, because a travel credit is reported to
// the Department of Transportation as no compensation (GAO-20-191). It
// is an estimate.
func Offer(fare int64) int64 { return min(2*fare, bumpedLimit) }
