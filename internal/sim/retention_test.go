package sim

import (
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
)

// A record stays while any segment is still to fly or flew within the
// window; it goes once its last segment is hours behind the clock. A
// record without a dated segment is never guessed at.
func TestRetentionKeepsUntilFlown(t *testing.T) {
	day := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	now := day.Add(15 * time.Hour) // 1500z
	// The schedule knows 2 flights: one that left at 08:00 and one that
	// leaves at 20:00.
	schedule := func(sg pnr.Segment) (int, bool) {
		switch sg.FlightNum {
		case "0800":
			return 8 * 60, true
		case "2000":
			return 20 * 60, true
		}
		return 0, false
	}
	keep := keepUntilFlown(now.Add(-flownAfter), schedule)
	seg := func(hhmm string) pnr.Segment { return pnr.Segment{Depart: day, DepartTime: hhmm} }
	cases := []struct {
		name string
		p    pnr.PNR
		want bool
	}{
		{"departed at dawn", pnr.PNR{Segments: []pnr.Segment{seg("0600")}}, false},
		{"departed just now", pnr.PNR{Segments: []pnr.Segment{seg("1430")}}, true},
		{"departs tonight", pnr.PNR{Segments: []pnr.Segment{seg("2100")}}, true},
		{"connection: first leg flown, second tonight", pnr.PNR{Segments: []pnr.Segment{seg("0600"), seg("1900")}}, true},
		{"connection: both flown", pnr.PNR{Segments: []pnr.Segment{seg("0600"), seg("0900")}}, false},
		{"tomorrow", pnr.PNR{Segments: []pnr.Segment{{Depart: day.Add(24 * time.Hour), DepartTime: "0600"}}}, true},
		{"no segments", pnr.PNR{}, true},
		{"undated segment", pnr.PNR{Segments: []pnr.Segment{{DepartTime: "0600"}}}, true},
		// A booking made on the day has a date and no time. It has
		// not flown until the schedule says its flight has. It was once
		// taken as leaving at 00:00 and dropped at 03:00.
		{"no time, a flight the schedule does not know", pnr.PNR{Segments: []pnr.Segment{{Depart: day}}}, true},
		{"no time, a flight that leaves tonight", pnr.PNR{Segments: []pnr.Segment{{Depart: day, FlightNum: "2000"}}}, true},
		{"no time, a flight that left this morning", pnr.PNR{Segments: []pnr.Segment{{Depart: day, FlightNum: "0800"}}}, false},
		{"no time, the morning flight and the evening one", pnr.PNR{Segments: []pnr.Segment{{Depart: day, FlightNum: "0800"}, {Depart: day, FlightNum: "2000"}}}, true},
	}
	for _, tc := range cases {
		p := tc.p
		if got := keep(&p); got != tc.want {
			t.Errorf("%s: keep=%v, want %v", tc.name, got, tc.want)
		}
	}
	if d := departureOf(seg("0930"), nil); !d.Equal(day.Add(9*time.Hour + 30*time.Minute)) {
		t.Errorf("departureOf = %v", d)
	}
}
