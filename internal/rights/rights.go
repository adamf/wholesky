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
