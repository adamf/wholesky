package tariff

import (
	"errors"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/fare"

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
