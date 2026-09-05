package tariff

import (
	"errors"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/world"
)

func TestLadderPricesFromDistanceAndCloses(t *testing.T) {
	m := &world.Manifest{
		Airports: []world.Airport{{IATA: "BNA"}, {IATA: "DCA"}},
		Flights:  []world.Flight{{Carrier: "WN", Number: "2554", From: "BNA", To: "DCA", KM: 904}, {Carrier: "OO", Number: "3991", From: "SFO", To: "SEA", KM: 1092, Marketing: "DL", MarketingNumber: "3991"}},
	}
	tf := FromManifest(m)
	fares := tf.Fares("WN", "BNA", "DCA")
	if len(fares) != len(ladder) || fares[4].Class != "Y" || fares[4].Basis != "YOW" {
		t.Fatalf("ladder: %+v", fares)
	}
	y := fares[4].OneWay.Amount
	if y < 15000 || y > 30000 {
		t.Errorf("a 900km full fare should be a couple of hundred dollars: %d", y)
	}
	for i := 1; i < len(fares); i++ {
		if fares[i].OneWay.Amount > fares[i-1].OneWay.Amount && fares[i-1].Class != "D" {
			t.Errorf("the ladder should descend: %s %d after %s %d", fares[i].Class, fares[i].OneWay.Amount, fares[i-1].Class, fares[i-1].OneWay.Amount)
		}
	}
	// The marketing carrier has the market too.
	if len(tf.Fares("DL", "SFO", "SEA")) == 0 || len(tf.Fares("DL", "BNA", "DCA")) != 0 {
		t.Errorf("codeshare markets belong to the marketing carrier as well")
	}
	dep := time.Date(2025, 11, 26, 8, 0, 0, 0, time.UTC)
	// Booked six weeks out, K sells; booked the day before, only the full fares do.
	if q, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "K", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -42)}); err != nil || q.Passengers[0].Segments[0].Basis != "KLX21" {
		t.Errorf("K six weeks out: %v %v", q, err)
	}
	var nf *fare.ErrNoFare
	if _, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "K", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -1)}); !errors.As(err, &nf) {
		t.Errorf("K the day before should fail advance purchase: %v", err)
	}
	if _, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "Y", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.Add(-time.Hour)}); err != nil {
		t.Errorf("Y sells to the last minute: %v", err)
	}
	au := Authorizations("Y", 174)
	if au[0].Class != "Y" || au[0].Authorized != 174 || au[len(au)-1].Class != "N" || au[len(au)-1].Authorized != 31 {
		t.Errorf("authorisations: %+v", au)
	}
}

// The forecast fed to EMSR-b closes the deep discounts on a 737 while
// full fare sells to the seat and the ladder nests.
func TestForecastClosesTheDeepDiscounts(t *testing.T) {
	fc := Forecast("Y", 174)
	if len(fc) != 10 || fc[0].Class != "Y" || fc[0].Fare != 1 || fc[9].Class != "N" {
		t.Fatalf("forecast: %+v", fc)
	}
	var total float64
	for _, c := range fc {
		total += c.Mean
	}
	if total < 190 || total > 193 {
		t.Errorf("demand a tenth above the cabin: %.1f", total)
	}
	lv := inventory.EMSRb(174, fc)
	if lv[0].Class != "Y" || lv[0].Authorized != 174 {
		t.Errorf("Y to the seat: %+v", lv)
	}
	for i := 1; i < len(lv); i++ {
		if lv[i].Authorized > lv[i-1].Authorized {
			t.Errorf("does not nest: %+v", lv)
		}
	}
	if n := lv[len(lv)-1]; n.Class != "N" || n.Authorized > 17 {
		t.Errorf("the deepest discount is nearly shut: %+v", lv)
	}
	if b := lv[1]; b.Class != "B" || b.Authorized < 130 {
		t.Errorf("the class under full fare stays wide open: %+v", lv)
	}
	if c := Forecast("C", 48); len(c) != 3 || c[0].Class != "J" {
		t.Errorf("business cabin: %+v", c)
	}
}

// The forecaster reads the curve: at the start of the selling window the
// pickup forecast is the baseline; at the door it is what was sold; a
// flight selling behind its curve reopens the deep discount.
func TestPickupFollowsTheBookingCurve(t *testing.T) {
	base := Forecast("Y", 174)
	start := Pickup("Y", 174, nil, 1)
	for i := range base {
		if start[i] != base[i] {
			t.Fatalf("at the start of the window the forecast is the baseline: %+v vs %+v", start[i], base[i])
		}
	}
	sold := map[string]int{"Y": 10, "B": 12, "K": 20}
	door := Pickup("Y", 174, sold, 0)
	for _, c := range door {
		if float64(sold[c.Class]) != c.Mean || c.StdDev != 0 {
			t.Errorf("at the door the forecast is what sold: %+v", c)
		}
	}
	// Behind the curve: halfway to departure with a quarter of the seats
	// sold, the deepest discount has room again.
	behind := inventory.EMSRb(174, Pickup("Y", 174, map[string]int{"Y": 8, "M": 10, "Q": 12, "N": 10}, 0.5))
	static := inventory.EMSRb(174, Forecast("Y", 174))
	var nBehind, nStatic int
	for _, l := range behind {
		if l.Class == "N" {
			nBehind = l.Authorized
		}
	}
	for _, l := range static {
		if l.Class == "N" {
			nStatic = l.Authorized
		}
	}
	if nBehind <= nStatic {
		t.Errorf("behind the curve N should reopen: %d vs static %d", nBehind, nStatic)
	}
	// Priced, the fares are money: Y at the market's full fare, N at its share.
	priced := PickupPriced("Y", 174, nil, 1, 20000)
	if priced[0].Class != "Y" || priced[0].Fare != 20000 || priced[len(priced)-1].Fare != 0.24*20000 {
		t.Errorf("priced fares: %v %v", priced[0], priced[len(priced)-1])
	}
	// A class sold outside the mix is still demand.
	odd := Pickup("Y", 10, map[string]int{"Z": 2}, 0.5)
	found := false
	for _, c := range odd {
		if c.Class == "Z" && c.Mean == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("an off-ladder class sold is not forecast: %+v", odd)
	}
}

// A carrier's pricing decision scales its whole ladder and nobody else's.
func TestMultiplierScalesOneCarriersFares(t *testing.T) {
	m := &world.Manifest{Flights: []world.Flight{
		{Carrier: "BA", From: "LHR", To: "JFK", KM: 5550}, {Carrier: "VS", From: "LHR", To: "JFK", KM: 5550},
	}}
	tf := FromManifest(m)
	base := tf.Fares("BA", "LHR", "JFK")[0].OneWay
	other := tf.Fares("VS", "LHR", "JFK")[0].OneWay
	tf.SetMultiplier("ba", 1.5)
	if got := tf.Fares("BA", "LHR", "JFK")[0].OneWay; got.Amount != base.Amount*3/2 && got.Amount < base.Amount*14/10 {
		t.Errorf("BA full fare %v after a 1.5 multiplier on %v", got, base)
	}
	if got := tf.Fares("VS", "LHR", "JFK")[0].OneWay; got.Amount != other.Amount {
		t.Errorf("VS moved: %v -> %v", other, got)
	}
	tf.SetMultiplier("BA", 0)
	if got := tf.Fares("BA", "LHR", "JFK")[0].OneWay; got.Amount != base.Amount || tf.Multiplier("BA") != 1 {
		t.Errorf("filing not restored: %v", got)
	}
}
