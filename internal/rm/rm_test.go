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
