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
	keep := keepUntilFlown(now.Add(-flownAfter))
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
		{"no time on the date", pnr.PNR{Segments: []pnr.Segment{{Depart: day}}}, false},
	}
	for _, tc := range cases {
		p := tc.p
		if got := keep(&p); got != tc.want {
			t.Errorf("%s: keep=%v, want %v", tc.name, got, tc.want)
		}
	}
	if d := departureOf(seg("0930")); !d.Equal(day.Add(9*time.Hour + 30*time.Minute)) {
		t.Errorf("departureOf = %v", d)
	}
}
