package refdata

import (
	"testing"
	"time"
)

// The public series are the publishers' own: the first and last rows, and
// the known extremes, are where the publishers put them.
func TestJetFuelSeries(t *testing.T) {
	s := JetFuel()
	if len(s) < 8000 || s[0].Date != time.Date(1990, 4, 2, 0, 0, 0, 0, time.UTC) || s[0].Value != 0.55 {
		t.Fatalf("%d days from %v at %v", len(s), s[0].Date, s[0].Value)
	}
	// The Gulf Coast daily high of 2008 came with Hurricane Ike.
	hi, at := 0.0, time.Time{}
	for _, p := range s {
		if p.Date.Year() == 2008 && p.Value > hi {
			hi, at = p.Value, p.Date
		}
	}
	if at.Month() != time.September || hi < 4.5 || hi > 5.0 {
		t.Errorf("2008 high %.2f on %v", hi, at)
	}
	// A weekend takes the last trading day's price; before the series
	// there is none.
	fri, ok1 := JetFuelOn(time.Date(2019, 6, 14, 0, 0, 0, 0, time.UTC))
	sun, ok2 := JetFuelOn(time.Date(2019, 6, 16, 12, 0, 0, 0, time.UTC))
	if !ok1 || !ok2 || fri != sun || fri < 1.5 || fri > 2.2 {
		t.Errorf("June 2019: %v %v", fri, sun)
	}
	if _, ok := JetFuelOn(time.Date(1985, 1, 1, 0, 0, 0, 0, time.UTC)); ok {
		t.Error("a price before the series begins")
	}
	if b := Brent(); len(b) < 400 || b[0].Date.Year() != 1987 {
		t.Errorf("Brent: %d months from %v", len(b), b[0].Date)
	}
}

func TestConsumerAirfareReport(t *testing.T) {
	c := CityPairs()
	if len(c) != 4000 {
		t.Fatalf("%d city-pair rows, want four quarters of 1,000", len(c))
	}
	// The largest market of the report: Miami and New York.
	top := c[0]
	for _, m := range c {
		if m.Year == 2026 && m.Quarter == 1 && m.Passengers > top.Passengers {
			top = m
		}
		if m.Fare <= 0 || m.Miles <= 0 || m.Passengers <= 0 || m.LargestShare <= 0 || m.LargestShare > 1 {
			t.Fatalf("a row that is not a market: %+v", m)
		}
	}
	if top.City1 != "Miami, FL (Metropolitan Area)" || top.City2 != "New York City, NY (Metropolitan Area)" || top.Miles != 1118 {
		t.Errorf("largest market of 2026 Q1: %+v", top)
	}
	if a := AirportPairs(); len(a) < 7000 || len(a[0].City1) != 3 {
		t.Errorf("airport pairs: %d rows, first %+v", len(a), a[0])
	}
}

// Every aircraft type has what the economy needs of it, and the numbers
// are in the range of their kind.
func TestAircraftTable(t *testing.T) {
	for _, code := range []string{"AT7", "E75", "73H", "320", "321", "789", "77W"} {
		a, ok := AircraftType(code)
		if !ok {
			t.Fatalf("the world flies %s and the table has no row for it", code)
		}
		if a.Seats <= 0 || a.MTOW <= 0 || a.BurnPerKM <= 0 || a.LTOFuel <= 0 || a.Lease10 <= 0 || a.Maintenance <= 0 || a.TurnFull <= 0 || a.Basis == "" {
			t.Errorf("%s is incomplete: %+v", code, a)
		}
	}
	a320, _ := AircraftType("A320")
	// 1,000 nautical miles on an A320: EUROCONTROL's 6,192 kg.
	if kg := a320.LTOFuel + a320.BurnFixed + a320.BurnPerKM*1852; kg < 6100 || kg > 6300 {
		t.Errorf("A320 over 1,000 nm burns %.0f kg, the source says 6,192", kg)
	}
	for _, a := range AircraftTypes() {
		perSeatKM := a.BurnPerKM / float64(a.Seats) * 1000
		if perSeatKM < 12 || perSeatKM > 35 {
			t.Errorf("%s burns %.1f g per seat-km in cruise", a.IATA, perSeatKM)
		}
	}
}

func TestAirportsAlliancesAndRegions(t *testing.T) {
	lhr, ok := Airport("lhr")
	if !ok || lhr.SlotTier != "S" || lhr.Charges != 68.22 || lhr.MCT != [4]int{60, 60, 60, 60} {
		t.Errorf("Heathrow: %+v", lhr)
	}
	if ams, _ := Airport("AMS"); ams.MCT[3] != 50 || ams.MCTBasis != "X" {
		t.Errorf("Amsterdam's station standard is in the manual: %+v", ams)
	}
	if _, ok := Airport("XXX"); ok {
		t.Error("facts about an airport that is not in the table")
	}
	if Alliance("LH") != "Star Alliance" || Alliance("ba") != "oneworld" || Alliance("DL") != "SkyTeam" || Alliance("FR") != "" {
		t.Error("alliances")
	}
	for _, c := range []struct{ country, tz, want string }{
		{"United States", "America/New_York", NorthAmerica}, {"Brazil", "America/Sao_Paulo", LatinAmerica},
		{"Germany", "Europe/Berlin", Europe}, {"Qatar", "Asia/Qatar", MiddleEast}, {"Japan", "Asia/Tokyo", Asia},
		{"Kenya", "Africa/Nairobi", Africa}, {"Australia", "Australia/Sydney", Oceania}, {"Spain", "Atlantic/Canary", Europe},
	} {
		if got := Region(c.country, c.tz); got != c.want {
			t.Errorf("%s (%s) is in %s, want %s", c.country, c.tz, got, c.want)
		}
	}
}
