package rm

import (
	"path/filepath"
	"testing"

	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// The frames, hand-listed: each lead time in days and the frame it falls
// in. The first frame takes everything from 63 days out; the last is the
// day of departure.
func TestFrameOf(t *testing.T) {
	for lead, want := range map[int]int{
		152: 0, 63: 0, 62: 1, 56: 1, 55: 2, 49: 2, 48: 3, 42: 3, 41: 4, 35: 4,
		34: 5, 30: 5, 29: 6, 24: 6, 23: 7, 21: 7, 20: 8, 17: 8, 16: 9, 14: 9,
		13: 10, 10: 10, 9: 11, 7: 11, 6: 12, 5: 12, 4: 13, 3: 13, 2: 14, 1: 14, 0: 15,
	} {
		if got := FrameOf(lead); got != want {
			t.Errorf("%d days out is frame %d, want %d", lead, got, want)
		}
	}
	if Frames != len(frameFrom) {
		t.Errorf("%d frames and %d boundaries", Frames, len(frameFrom))
	}
}

// Every advance purchase day the tariff files begins a frame, so a class
// is on sale for all of a frame or for none of it. A record that mixed
// the two would say a class sold little when it could not be bought.
func TestEveryAdvancePurchaseDayBeginsAFrame(t *testing.T) {
	apt := func(code string, lat, lon float64) world.Airport {
		return world.Airport{IATA: code, Country: "United States", TZ: "America/New_York", Lat: lat, Lon: lon}
	}
	m := &world.Manifest{
		Airports: []world.Airport{apt("BNA", 36.1, -86.7), apt("DCA", 38.9, -77.0)},
		Flights:  []world.Flight{{Carrier: "DL", Number: "2554", From: "BNA", To: "DCA", KM: 904}},
	}
	seen := 0
	for _, f := range tariff.FromManifest(m).Fares("DL", "BNA", "DCA") {
		ap := f.Rule.AdvancePurchaseDays
		if ap == 0 {
			continue
		}
		seen++
		if from := FrameFrom(FrameOf(ap)); from != ap {
			t.Errorf("class %s wants %d days and its frame begins at %d", f.Class, ap, from)
		}
	}
	if seen < 8 {
		t.Errorf("%d fares with advance purchase", seen)
	}
}

func TestMemoryKeepsItsRecordsAcrossARestart(t *testing.T) {
	fare := func(c string) float64 { return map[string]float64{"Y": 30000, "Q": 10000}[c] }
	m := New()
	// One number written two ways is one flight.
	if Key("UA", "0123", "ORD", "Y") != Key("UA", "123", "ORD", "Y") {
		t.Error("a flight number's leading zeros made two flights of one")
	}
	c := m.Cabin(Key("UA", "0123", "ORD", "Y"), []string{"Y", "Q"})
	for d := 0; d < Warm; d++ {
		if c.Learned() {
			t.Fatalf("learned after %d departures; a flight flies %d on the fixed limits", d, Warm)
		}
		l := c.Begin()
		l.Expose(3, "Q", 10)
		l.Sale(3, "Q", "Q", 20, 200000)
		l.Expose(14, "Y", 4)
		l.Sale(14, "Y", "Y", 6, 180000)
		c.Record(l)
	}
	if !c.Learned() {
		t.Fatalf("not learned after %d departures", Warm)
	}
	m.SetDay(41)
	path := filepath.Join(t.TempDir(), "state.json.rm")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	back := New()
	if err := back.Load(path); err != nil {
		t.Fatal(err)
	}
	if day, ok := back.Day(); !ok || day != 41 || back.Len() != 1 {
		t.Fatalf("read back day %d (%v) and %d cabins", day, ok, back.Len())
	}
	got := back.Peek(Key("UA", "123", "ORD", "Y"))
	if got == nil || got.Departures() != Warm {
		t.Fatalf("the record did not come back: %+v", got)
	}
	// 20 seats of Q and 6 of Y a departure, each on sale for all of its
	// frame: the demand to come from the start is those.
	want := c.ToCome(0, 1, fare)
	for i, d := range got.ToCome(0, 1, fare) {
		if d != want[i] {
			t.Errorf("read back %+v, want %+v", d, want[i])
		}
	}
	if want[0].Mean < 5.99 || want[0].Mean > 6.01 || want[1].Mean < 19.99 || want[1].Mean > 20.01 {
		t.Errorf("demand to come %+v, want 6 of Y and 20 of Q", want)
	}
	// Other classes in the cabin: the record is started again.
	if again := back.Cabin(Key("UA", "123", "ORD", "Y"), []string{"Y", "B", "Q"}); again.Departures() != 0 {
		t.Error("a record for other classes was kept")
	}
	// A file that is not there is an empty memory.
	if err := New().Load(filepath.Join(t.TempDir(), "none")); err != nil {
		t.Error(err)
	}
}

// A cabin of 150 seats holds 150 bookings at each departure, and 12 or
// 18 of them stay away by turns. The show-up rates are 0.92 and 0.88, so
// the mean is 0.90. Their sample deviation over 8 departures is
// sqrt(8*0.0004/7) = 0.0214, which is under the 0.0245 of 150
// independent passengers, sqrt(0.9*0.1/150), so the cabin has 0.0245.
//
// With a denied boarding at 2 fares and an empty seat at 1, the cost is
// least where the cabin overflows on about 1 flight in 3: 0.458
// deviations over the mean, a show-up rate of 0.9112, which 150 seats
// meet at 164.6 bookings. The whole limits on each side cost 4.44 seats'
// worth at 164 and 4.42 at 165, so the limit is 165. At 20 fares it is
// 159.
func TestACabinSetsItsLimitFromWhoCame(t *testing.T) {
	m := New()
	m.Departures = 1e12 // remembered in full, so the figures are the ones worked above
	key := Key("UA", "123", "ORD", "Y")
	c := m.Cabin(key, []string{"Y", "Q"})
	for d := 0; d < Warm-1; d++ {
		c.Show(150, 12+6*(d%2))
	}
	if _, _, ok := c.ShowRate(150); ok {
		t.Fatalf("a show-up rate after %d departures; a flight flies %d before it has one", Warm-1, Warm)
	}
	if got := c.Authorize(150, 0, DeniedCost, 1); got != 150 {
		t.Fatalf("with no record of who came the cabin was authorised %d", got)
	}
	c.Show(150, 12)
	c.Show(150, 18)
	mean, dev, ok := c.ShowRate(150)
	if !ok || mean < 0.8999 || mean > 0.9001 || dev < 0.02449 || dev > 0.02450 {
		t.Fatalf("show-up rate %.4f, deviation %.5f, %v; want 0.90 and 0.02449", mean, dev, ok)
	}
	if got := c.Authorize(150, 0, DeniedCost, 1); got != 165 {
		t.Errorf("authorised %d, want 165", got)
	}
	if got := c.Limit(150); got != 165 {
		t.Errorf("the limit in force is %d, want 165", got)
	}
	// Another aircraft on the day has its own seats and no more.
	if got := c.Limit(120); got != 120 {
		t.Errorf("a cabin of 120 seats was authorised %d under a limit set for 150", got)
	}
	// The limit goes with the cabin across a restart, and so does who
	// came.
	path := filepath.Join(t.TempDir(), "state.json.rm")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	back := New()
	back.Departures = 1e12
	if err := back.Load(path); err != nil {
		t.Fatal(err)
	}
	got := back.Peek(key)
	if got == nil || got.Limit(150) != 165 {
		t.Fatalf("the limit did not come back: %+v", got)
	}
	if m2, d2, ok := got.ShowRate(150); !ok || m2 != mean || d2 != dev {
		t.Errorf("read back a show-up rate of %.4f and %.5f, want %.4f and %.5f", m2, d2, mean, dev)
	}
	// A carrier that counts a denied boarding dearer overbooks less.
	if wary := got.Authorize(150, 0, 20, 1); wary != 159 {
		t.Errorf("at 20 fares the cabin was authorised %d, want 159", wary)
	}
	// A record that says half the passengers stay away is not believed
	// past a fifth over the seats.
	wild := m.Cabin(Key("UA", "9", "ORD", "Y"), []string{"Y"})
	for d := 0; d < Warm; d++ {
		wild.Show(100, 50)
	}
	if got := wild.Authorize(100, 0, DeniedCost, 1); got != 120 {
		t.Errorf("a cabin whose record says half stay away was authorised %d, want 120", got)
	}
	// The classes of a cabin change and its booking record starts again.
	// Who came does not depend on the classes.
	again := back.Cabin(key, []string{"Y", "B", "Q"})
	if again.Departures() != 0 {
		t.Error("a booking record for other classes was kept")
	}
	if m3, _, ok := again.ShowRate(150); !ok || m3 != mean || again.Limit(150) != 159 {
		t.Errorf("the record of who came was lost with the classes: %.4f %v, limit %d", m3, ok, again.Limit(150))
	}
}

// JetBlue and Southwest say they do not overbook.
func TestWhoOverbooks(t *testing.T) {
	for carrier, want := range map[string]bool{"UA": true, "DL": true, "NK": true, "B6": false, "WN": false, "wn": false} {
		if got := Overbooks(carrier); got != want {
			t.Errorf("%s overbooks: %v, want %v", carrier, got, want)
		}
	}
}
