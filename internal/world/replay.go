package world

// A recorded day. The Bureau of Transportation Statistics publishes every
// US scheduled passenger flight with its tail number, scheduled and actual
// times, delay causes, cancellations and diversions. CompileReplay turns one
// day of that file into a manifest whose flights are real -- real schedule,
// real airframes, real fates -- so the sky can fly a day that happened
// rather than a day it made up.
//
// What stays synthetic, and is labelled so: aircraft type (BTS carries the
// tail, not the type; the type is inferred from carrier and stage length),
// seat counts, and everything about passengers, who BTS does not count.

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Actual is what really happened to a flight, from the record.
type Actual struct {
	// DepDelay and ArrDelay are minutes against schedule; negative is early.
	DepDelay int `json:"dep_delay"`
	ArrDelay int `json:"arr_delay"`
	// Causes attributes the delay the way the record does, in minutes, so
	// the movement message can carry the industry's reason codes.
	Carrier      int `json:"carrier,omitempty"`
	Weather      int `json:"weather,omitempty"`
	NAS          int `json:"nas,omitempty"`
	Security     int `json:"security,omitempty"`
	LateAircraft int `json:"late_aircraft,omitempty"`
	// Cancelled flights carry the recorded reason: A carrier, B weather, C
	// national airspace, D security.
	Cancelled  bool   `json:"cancelled,omitempty"`
	CancelCode string `json:"cancel_code,omitempty"`
	// Diverted flights landed somewhere else first.
	Diverted   bool   `json:"diverted,omitempty"`
	DivertedTo string `json:"diverted_to,omitempty"`
}

// Replay says what day a manifest is a recording of.
type Replay struct {
	Source    string    `json:"source"`
	Date      time.Time `json:"date"`
	Flights   int       `json:"flights"`
	Cancelled int       `json:"cancelled"`
	Diverted  int       `json:"diverted"`
	Tails     int       `json:"tails"`
}

// ReplayOptions name the recording.
type ReplayOptions struct {
	// DataDir holds the OpenFlights snapshot, for airports and airline names.
	DataDir string
	// BTS is the on-time performance CSV (the marketing-carrier file, or a
	// day extracted from it).
	BTS string
	// Date is the day to replay, YYYY-MM-DD. Rows for other days are skipped.
	Date string
}

// regionalCarriers fly regional jets for the majors under capacity
// purchase agreements. Their fleets are small jets whatever the distance.
var regionalCarriers = map[string]bool{
	"OO": true, "YX": true, "MQ": true, "OH": true, "9E": true, "PT": true,
	"QX": true, "G7": true, "C5": true, "YV": true, "ZW": true, "EV": true,
	"CP": true, "AX": true,
}

// hawaii is where the domestic record's wide bodies go. Distance cannot
// tell Honolulu from Newark: SFO-EWR and LAX-HNL are both 4,100 km, and
// one is flown by a narrow body and the other is not.
var hawaii = map[string]bool{"HNL": true, "OGG": true, "KOA": true, "LIH": true, "ITO": true}

// replayEquipment infers a type from what BTS does not record: the
// regionals fly 76-seat jets, Southwest flies 737s, the majors fly narrow
// bodies domestically -- the longest transcontinental legs on the larger
// one -- and wide bodies to Hawaii.
func replayEquipment(carrier, from, to string, km float64) (string, int) {
	switch {
	case regionalCarriers[carrier]:
		return "E75", 76
	case carrier == "WN":
		return "73H", 174
	case hawaii[from] || hawaii[to]:
		return "789", 290
	case km > 3000:
		return "321", 220
	default:
		return "320", 180
	}
}

// CompileReplay builds a manifest from one recorded day.
func CompileReplay(opts ReplayOptions) (*Manifest, error) {
	date, err := time.Parse("2006-01-02", opts.Date)
	if err != nil {
		return nil, fmt.Errorf("replay: date %q: %w", opts.Date, err)
	}
	airports, err := readAirports(filepath.Join(opts.DataDir, "airports.dat"))
	if err != nil {
		return nil, err
	}
	byID, err := readAirlines(filepath.Join(opts.DataDir, "airlines.dat"))
	if err != nil {
		return nil, err
	}
	// The snapshot keys airlines by its own id; the record speaks IATA.
	airlines := map[string]rawAirline{}
	for _, al := range byID {
		if _, taken := airlines[al.iata]; !taken || al.country == "United States" {
			airlines[al.iata] = al
		}
	}
	fh, err := os.Open(opts.BTS)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	r := csv.NewReader(fh)
	r.ReuseRecord = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("replay: %s: %w", opts.BTS, err)
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.TrimSpace(h)] = i
	}
	need := []string{"FlightDate", "IATA_Code_Operating_Airline", "Flight_Number_Operating_Airline",
		"Origin", "Dest", "CRSDepTime", "CRSElapsedTime", "Distance", "Cancelled", "Diverted"}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			return nil, fmt.Errorf("replay: %s lacks column %s", opts.BTS, n)
		}
	}
	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	num := func(rec []string, name string) int {
		v := get(rec, name)
		if v == "" {
			return 0
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0
		}
		return int(f)
	}
	flag := func(rec []string, name string) bool { return num(rec, name) == 1 }

	locs := map[string]*time.Location{}
	locFor := func(a *Airport) *time.Location {
		if l, ok := locs[a.TZ]; ok {
			return l
		}
		l, err := time.LoadLocation(a.TZ)
		if err != nil {
			l = time.UTC
		}
		locs[a.TZ] = l
		return l
	}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	type agg struct {
		touch  map[string]int
		routes map[string]bool
	}
	carriers := map[string]*agg{}
	used := map[string]bool{}
	tails := map[string]bool{}
	rep := &Replay{Source: filepath.Base(opts.BTS), Date: dayStart}
	var flights []Flight
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("replay: %s: %w", opts.BTS, err)
		}
		if get(rec, "FlightDate") != opts.Date {
			continue
		}
		carrier := get(rec, "IATA_Code_Operating_Airline")
		from, to := get(rec, "Origin"), get(rec, "Dest")
		origin, ok1 := airports[from]
		_, ok2 := airports[to]
		if carrier == "" || !ok1 || !ok2 {
			continue // an airport the snapshot does not know cannot be drawn
		}
		crs := get(rec, "CRSDepTime")
		if len(crs) != 4 {
			continue
		}
		hh, _ := strconv.Atoi(crs[:2])
		mm, _ := strconv.Atoi(crs[2:])
		local := time.Date(date.Year(), date.Month(), date.Day(), hh, mm, 0, 0, locFor(origin))
		depMin := int(local.UTC().Sub(dayStart).Minutes())
		// The world's day is 0..1440 UTC; a west-coast evening departure is
		// the next UTC morning and wraps, as the flight day expects.
		depMin = ((depMin % 1440) + 1440) % 1440
		block := num(rec, "CRSElapsedTime")
		if block <= 0 {
			block = 60
		}
		miles := num(rec, "Distance")
		km := float64(miles) * 1.609
		equip, seats := replayEquipment(carrier, from, to, km)
		number := fmt.Sprintf("%04d", num(rec, "Flight_Number_Operating_Airline"))
		f := Flight{
			Carrier: carrier, Number: number, From: from, To: to,
			DepMin: depMin, ArrMin: depMin + block, BlockMin: block,
			Equipment: equip, Seats: seats, KM: int(km),
			Tail:      get(rec, "Tail_Number"),
			Marketing: get(rec, "IATA_Code_Marketing_Airline"),
		}
		if f.Marketing != "" {
			f.MarketingNumber = fmt.Sprintf("%04d", num(rec, "Flight_Number_Marketing_Airline"))
			if f.Marketing == carrier && f.MarketingNumber == number {
				f.Marketing, f.MarketingNumber = "", ""
			}
		}
		a := &Actual{
			DepDelay: num(rec, "DepDelay"), ArrDelay: num(rec, "ArrDelay"),
			Carrier: num(rec, "CarrierDelay"), Weather: num(rec, "WeatherDelay"),
			NAS: num(rec, "NASDelay"), Security: num(rec, "SecurityDelay"),
			LateAircraft: num(rec, "LateAircraftDelay"),
			Cancelled:    flag(rec, "Cancelled"), CancelCode: get(rec, "CancellationCode"),
			Diverted: flag(rec, "Diverted"), DivertedTo: get(rec, "Div1Airport"),
		}
		if a.Cancelled {
			rep.Cancelled++
			a.DepDelay, a.ArrDelay = 0, 0
		}
		if a.Diverted {
			rep.Diverted++
			if _, ok := airports[a.DivertedTo]; !ok {
				a.DivertedTo = ""
			}
		}
		f.Actual = a
		// Two rows for one operating flight -- a duplicate in the record --
		// would fly the aircraft twice.
		key := carrier + number + from + to + crs
		if used[key] {
			continue
		}
		used[key] = true
		if f.Tail != "" {
			tails[f.Tail] = true
		}
		flights = append(flights, f)
		c := carriers[carrier]
		if c == nil {
			c = &agg{touch: map[string]int{}, routes: map[string]bool{}}
			carriers[carrier] = c
		}
		c.touch[from]++
		c.touch[to]++
		c.routes[from+"-"+to] = true
	}
	if len(flights) == 0 {
		return nil, fmt.Errorf("replay: no flights on %s in %s", opts.Date, opts.BTS)
	}
	rep.Flights, rep.Tails = len(flights), len(tails)

	var cs []Carrier
	for code, a := range carriers {
		hub, best := "", -1
		for apt, n := range a.touch {
			if n > best || (n == best && apt < hub) {
				hub, best = apt, n
			}
		}
		name, country, icao := code, "United States", ""
		if al, ok := airlines[code]; ok {
			if al.name != "" {
				name = al.name
			}
			if al.country != "" {
				country = al.country
			}
			icao = al.icao
		}
		format := "typeb"
		if hash32(code)%3 == 0 {
			format = "edifact"
		}
		transport := "tcp"
		if format == "typeb" && hash32(code)%5 == 1 {
			transport = "matip"
		}
		cs = append(cs, Carrier{
			Designator: code, Name: name, Country: country, Hub: hub,
			ICAO:       icaoDesignator(code, icao),
			TTYAddress: ttyAddress(hub, code), Format: format, Transport: transport,
			Routes: len(a.routes),
		})
	}
	sortCarriers(cs)

	seen := map[string]bool{}
	var as []Airport
	for _, f := range flights {
		for _, code := range []string{f.From, f.To, f.Actual.DivertedTo} {
			if code == "" || seen[code] {
				continue
			}
			seen[code] = true
			as = append(as, *airports[code])
		}
	}
	sortAirports(as)
	sort.SliceStable(flights, func(i, j int) bool {
		if flights[i].Carrier != flights[j].Carrier {
			return flights[i].Carrier < flights[j].Carrier
		}
		if flights[i].DepMin != flights[j].DepMin {
			return flights[i].DepMin < flights[j].DepMin
		}
		return flights[i].Number < flights[j].Number
	})
	return &Manifest{
		Seed: 0, Scale: 1, Region: "United States",
		Airports: as, Carriers: cs, Flights: flights,
		Replay:      rep,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// RecordedDelay is the flight's recorded departure and arrival delay, in
// minutes, when the manifest is a replay; ok is false for a synthetic
// flight, whose delays the flight day invents.
func (f Flight) RecordedDelay() (dep, arr int, ok bool) {
	if f.Actual == nil {
		return 0, 0, false
	}
	return f.Actual.DepDelay, f.Actual.ArrDelay, true
}

// DelayCodes renders the recorded causes as the industry's two-digit
// reason codes with durations, longest first: 93 late arrival of the
// aircraft, 71 weather, 81 air traffic flow, 61 flight operations and crew
// (the carrier's own), 86 security. The mapping from BTS's five causes to
// IATA's codes is this project's; BTS does not use the codes.
func (a *Actual) DelayCodes() []struct {
	Code    string
	Minutes int
} {
	type dc = struct {
		Code    string
		Minutes int
	}
	var out []dc
	for _, c := range []dc{{"93", a.LateAircraft}, {"71", a.Weather}, {"81", a.NAS}, {"61", a.Carrier}, {"86", a.Security}} {
		if c.Minutes > 0 {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Minutes > out[j].Minutes })
	return out
}
