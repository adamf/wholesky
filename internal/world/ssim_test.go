package world

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func ssimWorld() *Manifest {
	return &Manifest{
		Airports: []Airport{
			{IATA: "SEA", Lat: 47.449, Lon: -122.309, TZ: "America/Los_Angeles"},
			{IATA: "GEG", Lat: 47.62, Lon: -117.534, TZ: "America/Los_Angeles"},
			{IATA: "LHR", Lat: 51.4775, Lon: -0.4614, TZ: "Europe/London"},
		},
		Carriers: []Carrier{{Designator: "OO", Name: "SkyWest"}, {Designator: "AS", Name: "Alaska"}, {Designator: "BA", Name: "British Airways"}},
		Flights: []Flight{
			{Carrier: "OO", Number: "3000", From: "SEA", To: "GEG", DepMin: 14*60 + 30, ArrMin: 15*60 + 35, BlockMin: 65, Equipment: "E75", Seats: 76, KM: 361, Marketing: "AS", MarketingNumber: "3000"},
			{Carrier: "OO", Number: "3001", From: "GEG", To: "SEA", DepMin: 17 * 60, ArrMin: 18*60 + 5, BlockMin: 65, Equipment: "E75", Seats: 76, KM: 361},
			{Carrier: "BA", Number: "0049", From: "LHR", To: "SEA", DepMin: 22*60 + 40, ArrMin: 24*60 + 55, BlockMin: 135, Equipment: "789", Seats: 216, KM: 7700},
		},
	}
}

// The world's schedule goes out as chapter 7 and comes back as the same
// flights: times on UTC, the overnight arrival's day offset, the
// codeshare as the marketing designator on the operated leg, distances
// from the airports and rotations chained.
func TestSSIMRoundTripsTheSchedule(t *testing.T) {
	m := ssimWorld()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC) // a Thursday
	var buf bytes.Buffer
	if err := WriteSSIM(&buf, m, day); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"2UAS ", "2UBA ", "2UOO ", // one carrier record each, in order
		"26NOV2626NOV26   4   ", // the day, a Thursday
		" OO 30000101J", "SEA14301430-0800", "GEG15351535-0800", "E75",
		"XX050SEAGEGAS 3000", // the operated leg names its marketing flight
		" AS 30000101J",      // the marketing carrier's own leg
		"XX010SEAGEGOO 3000", // which says who operates it
		"LHR22402240+0000", "SEA00550055-0800",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// The marketing leg carries the disclosure and the overnight leg its day offset.
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "3 AS 3000") && l[148] != 'L' {
			t.Errorf("marketing leg without disclosure L: %q", l[140:160])
		}
		if strings.HasPrefix(l, "3 BA   49") && l[193] != '1' {
			t.Errorf("overnight arrival without a day offset: %q", l[190:200])
		}
	}

	flights, err := LoadSSIM(strings.NewReader(out), m)
	if err != nil {
		t.Fatal(err)
	}
	if len(flights) != 3 {
		t.Fatalf("flights back: %d (%+v)", len(flights), flights)
	}
	byKey := map[string]Flight{}
	for _, f := range flights {
		byKey[f.Carrier+f.Number] = f
	}
	for _, want := range m.Flights {
		got, ok := byKey[want.Carrier+want.Number]
		if !ok {
			t.Errorf("%s%s lost", want.Carrier, want.Number)
			continue
		}
		if got.From != want.From || got.To != want.To || got.DepMin != want.DepMin || got.ArrMin != want.ArrMin || got.BlockMin != want.BlockMin ||
			got.Equipment != want.Equipment || got.Seats != want.Seats || got.Marketing != want.Marketing || got.MarketingNumber != want.MarketingNumber {
			t.Errorf("%s%s\n want %+v\n got  %+v", want.Carrier, want.Number, want, got)
		}
		if got.KM < want.KM*95/100 || got.KM > want.KM*105/100 {
			t.Errorf("%s%s km %d, want about %d", want.Carrier, want.Number, got.KM, want.KM)
		}
		if got.Tail == "" {
			t.Errorf("%s%s has no rotation", want.Carrier, want.Number)
		}
	}
	if byKey["OO3000"].Tail != byKey["OO3001"].Tail {
		t.Errorf("the out-and-back is one aircraft: %s vs %s", byKey["OO3000"].Tail, byKey["OO3001"].Tail)
	}
}

// A file in local time is placed on UTC through its variation fields.
func TestLoadSSIMPlacesLocalTimeOnUTC(t *testing.T) {
	m := ssimWorld()
	src := "1AIRLINE STANDARD SCHEDULE DATA SET\n2LOO\n3 OO 30000101J26NOV2626NOV26   4    SEA06300630-0800  GEG07350735-0800  E75" +
		strings.Repeat(" ", 200-84) + "\n5 OO\n"
	flights, err := LoadSSIM(strings.NewReader(src), m)
	if err != nil {
		t.Fatal(err)
	}
	if len(flights) != 1 || flights[0].DepMin != 14*60+30 || flights[0].ArrMin != 15*60+35 {
		t.Errorf("local 0630 at -0800 is 1430Z: %+v", flights)
	}
	if season(time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)) != "W26" || season(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) != "W25" || season(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) != "S26" {
		t.Error("seasons")
	}
	if seatsOf("C12Y150") != 162 || seatsOf("Y76") != 76 || seatsOf("") != 0 {
		t.Error("configuration seats")
	}
}
