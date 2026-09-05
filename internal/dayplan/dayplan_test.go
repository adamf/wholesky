package dayplan

import (
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

func testManifest() *world.Manifest {
	m := &world.Manifest{
		Airports: []world.Airport{
			{IATA: "JFK", ICAO: "KJFK", Lat: 40.64, Lon: -73.78},
			{IATA: "BOS", ICAO: "KBOS", Lat: 42.36, Lon: -71.01},
			{IATA: "ORD", ICAO: "KORD", Lat: 41.98, Lon: -87.90},
			{IATA: "LAX", ICAO: "KLAX", Lat: 33.94, Lon: -118.41},
		},
		Carriers: []world.Carrier{{Designator: "B6", Hub: "JFK"}, {Designator: "UA", Hub: "ORD"}},
	}
	// Sixty arrivals into JFK from BOS, one every twelve minutes from 0800:
	// five an hour, which is the airport's normal rate.
	for i := 0; i < 60; i++ {
		dep := 8*60 + i*12
		m.Flights = append(m.Flights, world.Flight{Carrier: "B6", Number: pad(100 + i), From: "BOS", To: "JFK", DepMin: dep, ArrMin: dep + 75, BlockMin: 75, KM: 300})
	}
	return m
}

func pad(n int) string { return "0" + itoa(n) }
func itoa(n int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// A weather cell over JFK halves the arrival rate: the arrivals inside the
// window queue first come first served, each later one waits longer, the
// slot is the off-block plus the wait plus the taxi, and the cause is
// weather at the arrival aerodrome, 84.
func TestRegulationSlotsArrivalsFirstComeFirstServed(t *testing.T) {
	m := testManifest()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	p := &Plan{Day: day, Flights: map[string]*Flight{}}
	for _, f := range m.Flights {
		p.Flights[Key(f)] = &Flight{}
	}
	p.Weather = []Cell{{Name: "JFKA", Lat: 40.64, Lon: -73.78, RadiusKM: 100, Start: 14 * 60, End: 18 * 60, Factor: 0.5, Airports: []string{"JFK"}}}
	p.regulate(m)
	if len(p.Regulations) != 1 {
		t.Fatalf("regulations %+v", p.Regulations)
	}
	reg := p.Regulations[0]
	if reg.Airport != "JFK" || reg.Name != "KJFKA26A" || reg.CauseText != "WA 84" || reg.NormalRate != 5 || reg.Rate != 2 {
		t.Errorf("regulation %+v", reg)
	}
	// Arrivals every twelve minutes held to two an hour: the waits grow.
	prev := -1
	held := 0
	for _, f := range m.Flights {
		pf := p.Flights[Key(f)]
		if f.ArrMin < 14*60 || f.ArrMin >= 18*60 {
			if pf.ATFM != 0 {
				t.Errorf("%s outside the window was slotted", Key(f))
			}
			continue
		}
		held++
		if pf.ATFM > 0 {
			if pf.ATFM < prev {
				t.Errorf("%s waits %d after a wait of %d", Key(f), pf.ATFM, prev)
			}
			if pf.CTOT != f.DepMin+pf.ATFM+TaxiMin || pf.Cause.IATA != "84" || pf.Regulation != reg.Name || pf.DepDelay != pf.ATFM {
				t.Errorf("%s slot %+v", Key(f), pf)
			}
			prev = pf.ATFM
		}
	}
	if held < 6 || reg.Flights < held-1 || reg.DelayMin == 0 {
		t.Errorf("held %d, regulation counted %d flights and %d minutes", held, reg.Flights, reg.DelayMin)
	}
}

// A late aircraft is late for its next leg: with a thirty-minute turn, a
// first leg landing an hour late makes the second leg forty minutes late
// when it had fifty of slack; a duty that runs all day on schedule needs a
// second crew when Part 117 says so; and a crew timed out by delay is
// rescued by reserves at the base and cancels the flight away from it.
func TestRotationsPassDelayOnAndCrewsTimeOut(t *testing.T) {
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	m := &world.Manifest{Carriers: []world.Carrier{{Designator: "UA", Hub: "ORD"}}}
	legs := []world.Flight{
		{Carrier: "UA", Number: "0001", From: "ORD", To: "BOS", DepMin: 7 * 60, ArrMin: 9 * 60, Tail: "N1"},
		{Carrier: "UA", Number: "0002", From: "BOS", To: "ORD", DepMin: 9*60 + 50, ArrMin: 12 * 60, Tail: "N1"},
		{Carrier: "UA", Number: "0003", From: "ORD", To: "LAX", DepMin: 13 * 60, ArrMin: 17 * 60, Tail: "N1"},
		{Carrier: "UA", Number: "0004", From: "LAX", To: "ORD", DepMin: 18 * 60, ArrMin: 22 * 60, Tail: "N1"},
		{Carrier: "UA", Number: "0005", From: "ORD", To: "BOS", DepMin: 23 * 60, ArrMin: 25 * 60, Tail: "N1"},
	}
	m.Flights = legs
	own := func(f world.Flight, _ time.Time) (int, int) {
		if f.Number == "0001" {
			return 60, 60
		}
		return 0, 0
	}
	p := Build(m, day, own)
	if p.Replay {
		t.Fatal("a synthetic day")
	}
	two := p.Of(legs[1])
	if two.LateAircraft != 40 || two.DepDelay != 40 {
		t.Errorf("second leg: %+v", two)
	}
	// Crew: 0600 report, legs at 7-9, 9:50-12, 13-17 is 11h15 of a 12h
	// (13h for a 0600 report with three segments) duty; the fourth leg
	// would end at 2215 -- 16h15 -- so a new crew takes it at LAX.
	if p.Of(legs[2]).Duty != 1 || p.Of(legs[3]).Duty != 2 {
		t.Errorf("duties: %d %d %d %d", p.Of(legs[0]).Duty, p.Of(legs[2]).Duty, p.Of(legs[3]).Duty, p.Of(legs[4]).Duty)
	}
	if p.Summary.Duties < 2 || p.Summary.TimedOut != 0 || p.Summary.Cancelled != 0 {
		t.Errorf("summary %+v", p.Summary)
	}
	// Now the fourth leg runs seven hours late: the second crew reported
	// at 1700 for a 12h limit, 14h on the extension, so 0700 is the wall.
	// The fourth lands at 0500; the fifth (2300 scheduled) leaves after the
	// turn at 0530 and would release at 0745 -- over the wall -- and it
	// leaves the base, so reserves fly it, ninety minutes later still.
	own2 := func(f world.Flight, _ time.Time) (int, int) {
		if f.Number == "0004" {
			return 420, 420
		}
		return 0, 0
	}
	p = Build(m, day, own2)
	five := p.Of(legs[4])
	if !five.Reserve || five.Cancelled || five.DepDelay != 390+ReserveCall {
		t.Errorf("fifth leg from the base with a timed-out crew: %+v (summary %+v)", five, p.Summary)
	}
	// Away from the base the same leg is cancelled for crew.
	m.Carriers[0].Hub = "LAX"
	p = Build(m, day, own2)
	five = p.Of(legs[4])
	if !five.Cancelled || five.Code != "A" || p.Summary.TimedOut != 1 || p.Summary.Cancelled != 1 {
		t.Errorf("fifth leg away from the base: %+v (summary %+v)", five, p.Summary)
	}
}

// On a recorded day the record is the plan: its delays stand, a flight the
// record blames on the national airspace or the weather gets the slot that
// explains it, and nothing new is cancelled.
func TestRecordedDayReadsItsSlotsFromTheCauses(t *testing.T) {
	day := time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)
	m := &world.Manifest{Replay: &world.Replay{Source: "bts"}, Airports: []world.Airport{{IATA: "JFK", ICAO: "KJFK"}},
		Carriers: []world.Carrier{{Designator: "B6", Hub: "JFK"}},
		Flights: []world.Flight{
			{Carrier: "B6", Number: "0100", From: "BOS", To: "JFK", DepMin: 900, ArrMin: 975, Tail: "N1", Actual: &world.Actual{DepDelay: 40, ArrDelay: 35, Weather: 30, NAS: 10}},
			{Carrier: "B6", Number: "0101", From: "JFK", To: "BOS", DepMin: 1020, ArrMin: 1095, Tail: "N1", Actual: &world.Actual{DepDelay: 20, ArrDelay: 15, Carrier: 20}},
			{Carrier: "B6", Number: "0102", From: "BOS", To: "JFK", DepMin: 1200, ArrMin: 1275, Tail: "N2", Actual: &world.Actual{Cancelled: true, CancelCode: "B"}},
		}}
	p := Build(m, day, nil)
	if !p.Replay {
		t.Fatal("a replay")
	}
	a := p.Of(m.Flights[0])
	if a.DepDelay != 40 || a.ArrDelay != 35 || a.ATFM != 30 || a.Cause == nil || a.Cause.IATA != "84" || a.CTOT != 900+40+TaxiMin || a.Regulation != "KJFKA26A" {
		t.Errorf("weather-delayed arrival: %+v", a)
	}
	if b := p.Of(m.Flights[1]); b.ATFM != 0 || b.DepDelay != 20 || b.Cancelled {
		t.Errorf("carrier-delayed departure: %+v", b)
	}
	if c := p.Of(m.Flights[2]); !c.Cancelled || c.Code != "B" {
		t.Errorf("recorded cancellation: %+v", c)
	}
	if p.Summary.Cancelled != 1 || p.Summary.Regulations != 1 || p.Summary.Slotted != 1 || len(p.Weather) != 0 {
		t.Errorf("summary %+v", p.Summary)
	}
}

// The synthetic day's weather is drawn from the day and is the same on
// every machine that builds it.
func TestWeatherIsDeterministic(t *testing.T) {
	m := testManifest()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	a := Build(m, day, func(world.Flight, time.Time) (int, int) { return 0, 0 })
	b := Build(m, day, func(world.Flight, time.Time) (int, int) { return 0, 0 })
	if len(a.Weather) == 0 || len(a.Weather) != len(b.Weather) || a.Weather[0].Name != b.Weather[0].Name || a.Weather[0].Start != b.Weather[0].Start {
		t.Errorf("weather differs: %+v vs %+v", a.Weather, b.Weather)
	}
	if a.Summary.Slotted != b.Summary.Slotted || a.Summary.Cells == 0 {
		t.Errorf("summaries differ: %+v %+v", a.Summary, b.Summary)
	}
}
