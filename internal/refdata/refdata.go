// Package refdata is the real data the world's economy is built on, kept
// in one place so it can be read and checked: what jet fuel has cost
// every trading day since 1990, what travellers paid and how many flew
// in the thousand largest domestic markets of the United States, what
// each aircraft type burns, costs to lease and to maintain, what
// airports charge and how long a connection needs at them, and which
// carriers belong to which alliance.
//
// Two of the files are public data sets as their publishers serve them:
// the US Energy Information Administration's Gulf Coast jet fuel and
// Brent spot prices, and the US Department of Transportation's Consumer
// Airfare Report. The others are tables compiled by hand from public
// sources, and every row says how each number is known: S for a figure
// read from a named source, R for one recalled and not checked again, E
// for an estimate, C for a value computed from sourced data. SOURCES.md
// beside this file names the sources and the dates they were read.
// Nothing here is licensed data: no filed fare, no schedule, no
// appraiser's database.
package refdata

import (
	"embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed data/*.csv
var files embed.FS

func rows(name string) [][]string {
	f, err := files.Open("data/" + name)
	if err != nil {
		panic("refdata: " + err.Error())
	}
	defer f.Close()
	r := csv.NewReader(f)
	all, err := r.ReadAll()
	if err != nil || len(all) < 2 {
		panic(fmt.Sprintf("refdata: %s: %v", name, err))
	}
	return all[1:]
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// Aircraft is one aircraft type.
type Aircraft struct {
	IATA, ICAO, Name string
	// Body is T for a turboprop, R for a regional jet, N for a
	// narrow-body and W for a wide-body.
	Body  string
	Seats int
	// MTOW is the maximum take-off weight in tonnes, which landing and
	// en-route charges are levied on.
	MTOW float64
	// A flight of d kilometres burns LTOFuel + BurnFixed + BurnPerKM x d
	// kilograms: the landing and take-off cycle with 26 minutes of taxi,
	// and a line fitted to EUROCONTROL's climb, cruise and descent fuel
	// by stage length.
	LTOFuel, BurnFixed, BurnPerKM float64
	// ValueNew is the market value of a new aircraft in millions of
	// dollars, zero for a type out of production. LeaseNew and Lease10
	// are the monthly dry operating lease rent, in thousands of dollars,
	// of a new aircraft and of one ten to twelve years old.
	ValueNew, LeaseNew, Lease10 float64
	// Maintenance is the direct maintenance cost per block hour at mature
	// age, in dollars.
	Maintenance float64
	// TurnFull and TurnMinimum are the manufacturer's full-service
	// turnaround and its minimum or transit turnaround, in minutes.
	TurnFull, TurnMinimum float64
	// Basis says how each group of numbers is known.
	Basis string
}

// Widebody reports whether the type has two aisles.
func (a Aircraft) Widebody() bool { return a.Body == "W" }

var (
	aircraftOnce sync.Once
	aircraft     []Aircraft
	aircraftBy   map[string]Aircraft
)

func loadAircraft() {
	aircraftBy = map[string]Aircraft{}
	for _, r := range rows("aircraft.csv") {
		a := Aircraft{IATA: r[0], ICAO: r[1], Name: r[2], Body: r[3], Seats: int(num(r[4])), MTOW: num(r[5]),
			LTOFuel: num(r[6]), BurnFixed: num(r[7]), BurnPerKM: num(r[8]), ValueNew: num(r[9]), LeaseNew: num(r[10]), Lease10: num(r[11]),
			Maintenance: num(r[12]), TurnFull: num(r[13]), TurnMinimum: num(r[14]), Basis: r[15]}
		aircraft = append(aircraft, a)
		aircraftBy[a.IATA] = a
		aircraftBy[a.ICAO] = a
	}
}

// AircraftTypes is every type in the table.
func AircraftTypes() []Aircraft {
	aircraftOnce.Do(loadAircraft)
	return aircraft
}

// AircraftType finds a type by its IATA or ICAO code.
func AircraftType(code string) (Aircraft, bool) {
	aircraftOnce.Do(loadAircraft)
	a, ok := aircraftBy[strings.ToUpper(code)]
	return a, ok
}

// Price is one day's or one month's price.
type Price struct {
	Date  time.Time
	Value float64
}

var (
	fuelOnce  sync.Once
	jetDaily  []Price
	brentMon  []Price
	firstDay  time.Time
	jetByDate map[int]float64
)

func loadFuel() {
	jetByDate = map[int]float64{}
	for _, r := range rows("jet-fuel-gulf-coast-daily.csv") {
		d, err := time.Parse("2006-01-02", r[0])
		if err != nil {
			continue
		}
		jetDaily = append(jetDaily, Price{d, num(r[1])})
	}
	for _, r := range rows("brent-monthly.csv") {
		d, err := time.Parse("2006-01", r[0])
		if err != nil {
			continue
		}
		brentMon = append(brentMon, Price{d, num(r[1])})
	}
	firstDay = jetDaily[0].Date
}

// JetFuel is the US Gulf Coast kerosene-type jet fuel spot price, in
// dollars per US gallon, for every trading day the series has.
func JetFuel() []Price {
	fuelOnce.Do(loadFuel)
	return jetDaily
}

// Brent is the monthly average Brent spot price in dollars per barrel.
func Brent() []Price {
	fuelOnce.Do(loadFuel)
	return brentMon
}

// JetFuelOn is the Gulf Coast spot price on a date: that day's, or the
// last trading day's before it. ok is false before the series begins or
// after it ends.
func JetFuelOn(day time.Time) (float64, bool) {
	fuelOnce.Do(loadFuel)
	d := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	last := jetDaily[len(jetDaily)-1].Date
	if d.Before(firstDay) || d.After(last.AddDate(0, 0, 4)) {
		return 0, false
	}
	i := sort.Search(len(jetDaily), func(i int) bool { return jetDaily[i].Date.After(d) })
	if i == 0 {
		return 0, false
	}
	return jetDaily[i-1].Value, true
}

// Market is one city pair or airport pair of the Consumer Airfare Report
// for one quarter: the passengers a day each way and the average one-way
// fare they paid, with the carrier that carried most of them and the
// carrier with the lowest average fare.
type Market struct {
	Year, Quarter             int
	City1, City2              string // city market names, or airport codes for an airport pair
	Miles                     int
	Passengers, Fare          float64
	Largest                   string
	LargestShare, LargestFare float64
	Lowest                    string
	LowestShare, LowestFare   float64
}

var (
	marketOnce sync.Once
	cityPairs  []Market
	airports   []Market
)

func loadMarkets() {
	read := func(name string) []Market {
		var out []Market
		for _, r := range rows(name) {
			out = append(out, Market{Year: int(num(r[0])), Quarter: int(num(r[1])), City1: r[4], City2: r[5], Miles: int(num(r[6])),
				Passengers: num(r[7]), Fare: num(r[8]), Largest: r[9], LargestShare: num(r[10]), LargestFare: num(r[11]),
				Lowest: r[12], LowestShare: num(r[13]), LowestFare: num(r[14])})
		}
		return out
	}
	cityPairs = read("airfare-city-pairs.csv")
	airports = read("airfare-airport-pairs.csv")
}

// CityPairs is the report's Table 1: the thousand largest city-pair
// markets of the contiguous United States, four quarters.
func CityPairs() []Market {
	marketOnce.Do(loadMarkets)
	return cityPairs
}

// AirportPairs is the report's Table 1a: the airport pairs within those
// markets, where City1 and City2 are airport codes.
func AirportPairs() []Market {
	marketOnce.Do(loadMarkets)
	return airports
}

// AirportFacts is what the table knows about one airport.
type AirportFacts struct {
	IATA string
	// SlotTier is how hard a slot is to get: S when the airport is full
	// all day and entry is by purchase, A when it is capped or full most
	// of the day, B when it is full at the peaks, C when it is
	// coordinated and slots can be had off-peak. Empty is not
	// coordinated. The tiers are estimates.
	SlotTier string
	// Charges is the airport's charges for a landing and a take-off per
	// passenger, in SDR, from the Jacobs review of 2024; zero when the
	// airport is not one of its fifty.
	Charges float64
	// MCT is the station's minimum connecting times in minutes: domestic
	// to domestic, domestic to international, international to domestic,
	// international to international. All zero when none is known.
	MCT [4]int
	// MCTBasis is P for a published reproduction of the station table, X
	// for a value in an IATA example, R recalled, E an estimate.
	MCTBasis string
}

var (
	airportOnce sync.Once
	airportBy   map[string]AirportFacts
)

func loadAirports() {
	airportBy = map[string]AirportFacts{}
	for _, r := range rows("airports.csv") {
		airportBy[r[0]] = AirportFacts{IATA: r[0], SlotTier: r[1], Charges: num(r[2]),
			MCT: [4]int{int(num(r[3])), int(num(r[4])), int(num(r[5])), int(num(r[6]))}, MCTBasis: r[7]}
	}
}

// Airport is the table's facts about an airport, the zero value with ok
// false when it has none.
func Airport(iata string) (AirportFacts, bool) {
	airportOnce.Do(loadAirports)
	a, ok := airportBy[strings.ToUpper(iata)]
	return a, ok
}

// Airports is every airport the table has facts about, by code.
func Airports() []AirportFacts {
	airportOnce.Do(loadAirports)
	out := make([]AirportFacts, 0, len(airportBy))
	for _, a := range airportBy {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IATA < out[j].IATA })
	return out
}

var (
	allianceOnce sync.Once
	allianceOf   map[string]string
)

// Alliance is the alliance a carrier belonged to in November 2026, or "".
func Alliance(carrier string) string {
	allianceOnce.Do(func() {
		allianceOf = map[string]string{}
		for _, r := range rows("alliances.csv") {
			allianceOf[r[0]] = r[1]
		}
	})
	return allianceOf[strings.ToUpper(carrier)]
}

var (
	modelOnce sync.Once
	modelOf   map[string]string
)

// CarrierModel is "lcc" or "ulcc" for a carrier the table names as a
// low-cost or ultra-low-cost carrier, and "" for any other. The list is
// recalled, not sourced.
func CarrierModel(carrier string) string {
	modelOnce.Do(func() {
		modelOf = map[string]string{}
		for _, r := range rows("carrier-models.csv") {
			modelOf[r[0]] = r[1]
		}
	})
	return modelOf[strings.ToUpper(carrier)]
}

// Region names the part of the world a country is in, for the things
// that are known by region and not by airport: fuel differentials,
// airport charges, how sensitive demand is to price. The time zone name
// settles the continent and the country the rest.
func Region(country, tz string) string {
	switch country {
	case "United States", "Canada":
		return NorthAmerica
	case "United Arab Emirates", "Qatar", "Saudi Arabia", "Bahrain", "Kuwait", "Oman", "Israel", "Jordan", "Lebanon", "Iraq", "Iran", "Syria", "Yemen":
		return MiddleEast
	case "Australia", "New Zealand":
		return Oceania
	}
	switch {
	case strings.HasPrefix(tz, "Europe/"), strings.HasPrefix(tz, "Atlantic/Reykjavik"), strings.HasPrefix(tz, "Atlantic/Canary"), strings.HasPrefix(tz, "Atlantic/Madeira"), strings.HasPrefix(tz, "Atlantic/Azores"), strings.HasPrefix(tz, "Atlantic/Faroe"):
		return Europe
	case strings.HasPrefix(tz, "America/"), strings.HasPrefix(tz, "Atlantic/"):
		return LatinAmerica
	case strings.HasPrefix(tz, "Africa/"), strings.HasPrefix(tz, "Indian/"):
		return Africa
	case strings.HasPrefix(tz, "Australia/"), strings.HasPrefix(tz, "Pacific/"):
		return Oceania
	case strings.HasPrefix(tz, "Asia/"):
		return Asia
	}
	return Asia
}

// The regions.
const (
	NorthAmerica = "North America"
	LatinAmerica = "Latin America"
	Europe       = "Europe"
	MiddleEast   = "Middle East"
	Africa       = "Africa"
	Asia         = "Asia"
	Oceania      = "Oceania"
)
