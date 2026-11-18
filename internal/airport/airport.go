// Package airport is what an airport costs a carrier and constrains it
// to: the charges for a turnaround, the charges for the airspace on the
// way, the minimum time a connection needs, and what a slot is worth
// where slots are scarce.
//
// The charges rest on the Jacobs review of airport charges for 2024,
// which prices a landing and a take-off at fifty airports and divides by
// the passengers carried; the reference table (internal/refdata) holds
// its index. An airport the review does not cover takes its region's
// level, which is an estimate. Connecting times are the station
// standards the reference table has, and a default by the airport's size
// where it has none. Slot values follow the recorded trades: Heathrow
// pairs at 60 to 75 million dollars, Washington National at five to six,
// LaGuardia at two to four, Newark at 1.2.
package airport

import (
	"math"

	"github.com/adamf/wholesky/internal/refdata"
)

// USDPerSDR converts the review's unit to dollars.
const USDPerSDR = 1.33

// regional is the level of charges where the review has no figure, in SDR
// per passenger: its own regional averages, with small European airports
// at the level low-cost carriers pay. Estimates.
var regional = map[string]float64{
	refdata.NorthAmerica: 12,
	refdata.Europe:       27,
	refdata.MiddleEast:   18,
	refdata.Asia:         18,
	refdata.Africa:       16,
	refdata.LatinAmerica: 20,
	refdata.Oceania:      30,
}

// Index is an airport's charges for a landing and a take-off per
// passenger, in SDR. In the United States and Canada the review's figure
// includes the facility charge and security fee the passenger pays as a
// tax on the ticket, which the airline only collects: six tenths of the
// figure is taken as the airline's own. A small airport in Europe
// charges what a low-cost carrier will pay.
func Index(iata, region string, departuresPerDay int) float64 {
	if a, ok := refdata.Airport(iata); ok && a.Charges > 0 {
		if region == refdata.NorthAmerica {
			return a.Charges * 0.6
		}
		return a.Charges
	}
	i := regional[region]
	if i == 0 {
		i = 18
	}
	if region == refdata.Europe && departuresPerDay < 40 {
		i = 8
	}
	return i
}

// TurnCost is what an airport charges for one turnaround of an aircraft
// of mtow tonnes with passengers departing, in dollars: a quarter of the
// charge goes with the aircraft's weight and three quarters with its
// passengers, which is the split of the Frankfurt tariff. At Frankfurt's
// index an A320 with 146 passengers costs 7,660 dollars and a 777-300ER
// with 301 costs 19,380; built from the 2026 tariff itself they cost
// about 7,500 and 19,000 to 21,800.
func TurnCost(index, mtow float64, passengers int) float64 {
	return index * USDPerSDR * (0.31*mtow + 0.75*float64(passengers))
}

// EnRoute is the charge for the airspace a flight of km kilometres
// crosses, in dollars. In Europe it is EUROCONTROL's formula: the unit
// rate times the distance in hundreds of kilometres times the square
// root of the weight over fifty tonnes, at a traffic-weighted unit rate
// of about 70 euros. Flights within the United States and Canada pay
// nothing en route, because ticket taxes pay for air traffic control.
// Elsewhere 0.40 dollars a kilometre at fifty tonnes is an estimate. A
// flight between two regions pays half its distance at each rate.
func EnRoute(km int, mtow float64, from, to string) float64 {
	rate := func(region string) float64 {
		switch region {
		case refdata.Europe:
			return 0.76 // 70 euros per 100 km
		case refdata.NorthAmerica:
			return 0
		}
		return 0.40
	}
	return float64(km) * math.Sqrt(mtow/50) * (rate(from) + rate(to)) / 2
}

// The four status pairs of a connection, as the index into an MCT.
const (
	DD = iota
	DI
	ID
	II
)

// MCT is the minimum connecting time at a station for a status pair, in
// minutes, and whether it is the station's own or a default. Where the
// reference table has the station standard that is the answer. Where it
// does not, the default goes by the airport's size: a small single
// terminal connects in half an hour and a very large airport with
// terminals apart needs an hour and more. IATA's own global defaults for
// a station with nothing filed are 30, 60, 90 and 90.
func MCT(iata string, departuresPerDay, status int) (minutes int, published bool) {
	if a, ok := refdata.Airport(iata); ok && a.MCT[status] > 0 {
		return a.MCT[status], true
	}
	var m [4]int
	switch {
	case departuresPerDay < 50:
		m = [4]int{30, 45, 60, 45}
	case departuresPerDay < 250:
		m = [4]int{40, 60, 75, 60}
	case departuresPerDay < 600:
		m = [4]int{50, 60, 90, 75}
	default:
		m = [4]int{60, 90, 105, 90}
	}
	return m[status], false
}

// InterAirport is the minimum connecting time between two airports of
// one city: IATA's global default of four hours, three for the pairs
// with a fast link.
func InterAirport(a, b string) int {
	switch a + b {
	case "LHRLGW", "LGWLHR", "CDGORY", "ORYCDG", "NRTHND", "HNDNRT":
		return 180
	}
	return 240
}

// Status is the status pair of a connection from whether each flight is
// international.
func Status(inboundInternational, outboundInternational bool) int {
	switch {
	case inboundInternational && outboundInternational:
		return II
	case inboundInternational:
		return ID
	case outboundInternational:
		return DI
	}
	return DD
}

// Misconnect is the chance a passenger misses a connection scheduled
// with `slack` minutes over the minimum connecting time when the inbound
// flight is late by inbound minutes and the outbound by outbound. The
// minimum is cautious, so most passengers need about ten minutes less,
// and how much less varies by five. No published curve was found; this
// one is shaped to give about three connecting passengers in a hundred a
// missed connection at ordinary delays, the figure of the NEXTOR study
// of 2007.
func Misconnect(slack, inbound, outbound int) float64 {
	x := float64(inbound-outbound-slack-10) / 5
	return 1 / (1 + math.Exp(-x))
}

// SlotValue is what a daily year-round pair of slots is worth at an
// airport, in dollars: the tier's base value, the hour of the arrival,
// and the aircraft that will use it.
//
// The base is 30 million at Heathrow and 25 at Haneda (tier S), about 4
// million at an airport capped all day (A), 1.2 million at one full at
// its peaks (B) and a quarter of a million where slots can usually be
// had (C). A morning arrival is worth the base, an early one half as
// much again, midday seven tenths, evening half, night a fifth:
// Heathrow's trades fell 30 per cent by midday and 50 by evening. A
// long-haul aircraft is worth one and a half times the base and a
// short-haul one half, because a long-haul flight earns several times as
// much from a slot. The tiers are estimates.
func SlotValue(iata string, arrivalMin int, widebody bool) float64 {
	a, _ := refdata.Airport(iata)
	var base float64
	switch a.SlotTier {
	case "S":
		base = 30e6
		if iata == "HND" {
			base = 25e6
		}
	case "A":
		base = 4e6
		switch iata {
		case "DCA":
			base = 7e6
		case "JFK", "LGW":
			base = 3e6
		}
	case "B":
		base = 1.2e6
	case "C":
		base = 0.25e6
	default:
		return 0
	}
	hour := (arrivalMin / 60) % 24
	h := 0.2
	switch {
	case hour >= 5 && hour < 8:
		h = 1.5
	case hour >= 8 && hour < 11:
		h = 1.0
	case hour >= 11 && hour < 20:
		h = 0.7
	case hour >= 20 && hour < 22:
		h = 0.5
	}
	g := 0.5
	if widebody {
		g = 1.5
	}
	return base * h * g
}

// Coordinated reports whether an airport rations slots at all.
func Coordinated(iata string) bool {
	a, ok := refdata.Airport(iata)
	return ok && a.SlotTier != ""
}
