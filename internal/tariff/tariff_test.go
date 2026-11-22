package tariff

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/world"
)

func usAirport(code string, lat, lon float64) world.Airport {
	return world.Airport{IATA: code, Country: "United States", TZ: "America/New_York", Lat: lat, Lon: lon}
}

// The fare line is the report's. Fit the average one-way fare against
// distance over the Consumer Airfare Report's four quarters, weighted by
// passengers, and hold the package's constants against the fit; the same
// for the share of the market's fare the lowest-fare carrier charges.
func TestTheFareLineIsFittedToTheReport(t *testing.T) {
	var sw, sx, sy, sxx, sxy, low float64
	for _, m := range refdata.CityPairs() {
		w, x, y := m.Passengers, float64(m.Miles), m.Fare
		sw += w
		sx += w * x
		sy += w * y
		sxx += w * x * x
		sxy += w * x * y
		low += w * m.LowestFare / m.Fare
	}
	b := (sw*sxy - sx*sy) / (sw*sxx - sx*sx)
	a := (sy - b*sx) / sw
	t.Logf("fit: fare = %.1f + %.4f a mile; the lowest-fare carrier charges %.3f of the market's fare", a, b, low/sw)
	if math.Abs(a-FareBase) > 2 || math.Abs(b-FarePerMile) > 0.002 || math.Abs(low/sw-LowestShare) > 0.01 {
		t.Errorf("the constants are %.1f + %.4f a mile and %.3f; the report gives %.1f + %.4f and %.3f", FareBase, FarePerMile, LowestShare, a, b, low/sw)
	}
	// Miami to New York, 1,118 miles: the report's fare was 238 dollars
	// in the first quarter of 2026 and the line gives 240.
	if f := MarketFare(1799); f < 232 || f > 246 {
		t.Errorf("the line at 1,118 miles: %.0f", f)
	}
}

func TestLadderPricesFromDistanceAndCloses(t *testing.T) {
	m := &world.Manifest{
		Airports: []world.Airport{usAirport("BNA", 36.1, -86.7), usAirport("DCA", 38.9, -77.0), usAirport("SFO", 37.6, -122.4), usAirport("SEA", 47.4, -122.3)},
		Flights:  []world.Flight{{Carrier: "WN", Number: "2554", From: "BNA", To: "DCA", KM: 904}, {Carrier: "OO", Number: "3991", From: "SFO", To: "SEA", KM: 1092, Marketing: "DL", MarketingNumber: "3991"}},
	}
	tf := FromManifest(m)
	fares := tf.Fares("WN", "BNA", "DCA")
	if len(fares) != len(ladder) || fares[4].Class != "Y" || fares[4].Basis != "YOW" {
		t.Fatalf("ladder: %+v", fares)
	}
	// 562 miles: the market averages 196 dollars. A low-cost carrier's
	// lowest regular fare is about 120 and its full fare 3.6 times that.
	y, v := fares[4].OneWay.Amount, fares[11].OneWay.Amount
	if fares[11].Class != "V" || v < 10800 || v > 13800 || y < 3*v || y > 4*v {
		t.Errorf("lowest regular fare %d, full fare %d", v, y)
	}
	for i := 1; i < len(fares); i++ {
		if fares[i].OneWay.Amount > fares[i-1].OneWay.Amount && fares[i-1].Class != "D" {
			t.Errorf("the ladder should descend: %s %d after %s %d", fares[i].Class, fares[i].OneWay.Amount, fares[i-1].Class, fares[i-1].OneWay.Amount)
		}
	}
	// The marketing carrier has the market too, and a network carrier
	// files its contract fares beside the public ones.
	dl := tf.Fares("DL", "SFO", "SEA")
	if len(dl) != len(ladder)+2 || len(tf.Fares("DL", "BNA", "DCA")) != 0 {
		t.Errorf("codeshare markets belong to the marketing carrier as well: %d fares", len(dl))
	}
	// A network carrier's ladder sits above a low-cost carrier's.
	if net, low := carrierFare("DL"), carrierFare("WN"); net <= low || carrierFare("FR") >= low {
		t.Errorf("carrier levels: network %.2f, low-cost %.2f, ultra-low-cost %.2f", net, low, carrierFare("FR"))
	}
	dep := time.Date(2025, 11, 26, 8, 0, 0, 0, time.UTC)
	// Booked six weeks out, K sells; booked the day before, only the full fares do.
	if q, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "K", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -42)}); err != nil || q.Passengers[0].Segments[0].Basis != "KA14" {
		t.Errorf("K six weeks out: %v %v", q, err)
	}
	var nf *fare.ErrNoFare
	if _, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "K", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -1)}); !errors.As(err, &nf) {
		t.Errorf("K the day before should fail advance purchase: %v", err)
	}
	q, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "WN", Origin: "BNA", Destination: "DCA", Class: "Y", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.Add(-time.Hour)})
	if err != nil {
		t.Fatalf("Y sells to the last minute: %v", err)
	}
	// A domestic US ticket in 2026: 7.5 per cent, 5.30 a segment, 5.60
	// security, 4.50 facility charge.
	if want := q.Base.Scale(0.075).Amount + 530 + 560 + 450; q.Taxes.Amount != want {
		t.Errorf("US domestic taxes %d, want %d", q.Taxes.Amount, want)
	}
	au := Authorizations("Y", 174)
	if au[0].Class != "Y" || au[0].Authorized != 174 || au[len(au)-1].Class != "N" || au[len(au)-1].Authorized != 31 {
		t.Errorf("authorisations: %+v", au)
	}
}

// A carrier sells a through fare between two points it connects, a little
// under a nonstop over the same distance and well under its two local
// fares added up; a partner's flight may ride it; and a carrier that is
// no partner gets no through fare, so the connection costs the two local
// fares.
func TestThroughFaresAndPartners(t *testing.T) {
	m := &world.Manifest{
		Airports: []world.Airport{usAirport("DEN", 39.9, -104.7), usAirport("ORD", 42.0, -87.9), usAirport("BOS", 42.4, -71.0), usAirport("MIA", 25.8, -80.3)},
		Flights: []world.Flight{
			{Carrier: "UA", From: "DEN", To: "ORD", KM: 1430}, {Carrier: "UA", From: "ORD", To: "BOS", KM: 1390},
			{Carrier: "AC", From: "ORD", To: "MIA", KM: 1930}, {Carrier: "AA", From: "ORD", To: "MIA", KM: 1930},
		},
	}
	tf := FromManifest(m)
	tf.Partners = func(c string) []string {
		switch c {
		case "UA":
			return []string{"AC"}
		case "AC":
			return []string{"UA"}
		}
		return nil
	}
	low := func(fs []fare.Fare) int64 {
		for _, f := range fs {
			if f.Class == "V" {
				return f.OneWay.Amount
			}
		}
		return 0
	}
	thru := tf.Fares("UA", "DEN", "BOS")
	if len(thru) == 0 || len(thru[0].Routing.Via) != 1 || thru[0].Routing.Via[0] != "ORD" {
		t.Fatalf("UA's through fare Denver to Boston by way of Chicago: %+v", thru)
	}
	locals := low(tf.Fares("UA", "DEN", "ORD")) + low(tf.Fares("UA", "ORD", "BOS"))
	if got := low(thru); got >= locals*8/10 || got < locals*5/10 {
		t.Errorf("through fare %d against two local fares of %d", got, locals)
	}
	// Priced as one fare component over both flights.
	dep := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	segs := []fare.Segment{
		{Carrier: "UA", Origin: "DEN", Destination: "ORD", Class: "V", Depart: dep, Arrive: dep.Add(150 * time.Minute)},
		{Carrier: "UA", Origin: "ORD", Destination: "BOS", Class: "V", Depart: dep.Add(4 * time.Hour)},
	}
	q, err := fare.Price(tf, fare.Request{Segments: segs, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -40)})
	if err != nil || len(q.Components) != 1 || q.Base.Amount != low(thru) {
		t.Fatalf("the connection prices on the through fare: %v %v", q, err)
	}
	// Denver to Miami: United to Chicago, then its partner Air Canada.
	joint := tf.Fares("UA", "DEN", "MIA")
	if len(joint) == 0 || len(joint[0].Rule.Carriers) != 1 || joint[0].Rule.Carriers[0] != "AC" {
		t.Fatalf("a joint fare over the partner's flight: %+v", joint)
	}
	segs[1] = fare.Segment{Carrier: "AC", Origin: "ORD", Destination: "MIA", Class: "V", Depart: dep.Add(4 * time.Hour)}
	onPartner, err := fare.Price(tf, fare.Request{Segments: segs, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -40)})
	if err != nil || len(onPartner.Components) != 1 {
		t.Fatalf("United and its partner on one fare: %v %v", onPartner, err)
	}
	// The same journey with American, no partner of United's: two fares,
	// and more money. Alliances took a quarter off interline fares when
	// they began.
	segs[1].Carrier = "AA"
	apart, err := fare.Price(tf, fare.Request{Segments: segs, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -40)})
	if err != nil || len(apart.Components) != 2 || apart.Base.Amount <= onPartner.Base.Amount*115/100 {
		t.Errorf("two carriers that are not partners: %d in %d fares, against %d with a partner", apart.Base.Amount, len(apart.Components), onPartner.Base.Amount)
	}
	if len(tf.Fares("AA", "DEN", "MIA")) != 0 {
		t.Error("American sells Denver to Miami and flies only Chicago to Miami")
	}
}

// A network carrier's cheapest long-haul fares are round-trip fares with
// a Saturday night; a low-cost carrier's are not. A web fare is for the
// carrier's own channels, a contract fare for its account, and a
// carrier's surcharge rides the ticket as its own money.
func TestFencesChannelsAndSurcharges(t *testing.T) {
	uk := world.Airport{IATA: "LHR", Country: "United Kingdom", TZ: "Europe/London", Lat: 51.47, Lon: -0.45}
	m := &world.Manifest{
		Airports: []world.Airport{uk, usAirport("JFK", 40.64, -73.78), {IATA: "EDI", Country: "United Kingdom", TZ: "Europe/London", Lat: 55.95, Lon: -3.37}},
		Flights: []world.Flight{{Carrier: "BA", From: "LHR", To: "JFK", KM: 5540}, {Carrier: "BA", From: "JFK", To: "LHR", KM: 5540},
			{Carrier: "DY", From: "LHR", To: "JFK", KM: 5540}, {Carrier: "BA", From: "LHR", To: "EDI", KM: 530}},
	}
	tf := FromManifest(m)
	tf.Surcharge = func(carrier string, km int, intl bool) int64 {
		if carrier == "BA" && km > 4000 {
			return 12000
		}
		return 0
	}
	by := func(fs []fare.Fare, basis string) *fare.Fare {
		for i := range fs {
			if fs[i].Basis == basis {
				return &fs[i]
			}
		}
		return nil
	}
	ba, dy := tf.Fares("BA", "LHR", "JFK"), tf.Fares("DY", "LHR", "JFK")
	if v := by(ba, "VLX21"); v == nil || !v.RoundTrip || !v.Rule.SaturdayNight {
		t.Errorf("British Airways' lowest transatlantic fare is a round trip with a Saturday night: %+v", ba)
	}
	if v := by(dy, "VA21"); v == nil || v.RoundTrip || v.Rule.SaturdayNight {
		t.Errorf("a low-cost carrier's ladder has no Saturday night: %+v", dy)
	}
	if v := by(tf.Fares("BA", "LHR", "EDI"), "VA21"); v == nil || v.RoundTrip {
		t.Error("a domestic fare was fenced like a long-haul one")
	}
	if s := by(ba, "SLX21WEB"); s == nil || len(s.Rule.Sale.Channels) != 2 {
		t.Errorf("the web fare is filed for the carrier's own channels: %+v", s)
	}
	corp, pub := by(ba, "MCORP"), by(ba, "MA3")
	if corp == nil || pub == nil || corp.OneWay.Amount >= pub.OneWay.Amount || corp.Rule.Sale.Accounts[0] != CorporateAccount {
		t.Errorf("the contract fare: %+v against %+v", corp, pub)
	}
	// One way to New York in full fare from London: the duty for a flight
	// over 2,000 miles and the carrier's surcharge, which is its own.
	dep := time.Date(2026, 6, 10, 11, 0, 0, 0, time.UTC)
	q, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "BA", Origin: "LHR", Destination: "JFK", Class: "Y", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -5)})
	if err != nil || q.CarrierFees.Amount != 12000 || q.Taxes.Amount != 2340+560+12000 {
		t.Errorf("London to New York: taxes %v of which the carrier's %v (%v)", q.Taxes, q.CarrierFees, err)
	}
	q, err = fare.Price(tf, fare.Request{Segments: []fare.Segment{{Carrier: "BA", Origin: "LHR", Destination: "EDI", Class: "Y", Depart: dep}}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -5)})
	if err != nil || q.Taxes.Amount != 1000 || q.CarrierFees.Amount != 0 {
		t.Errorf("London to Edinburgh: the domestic duty: %v (%v)", q.Taxes, err)
	}
	// With a Saturday night the round trip sells in V; home on Thursday it
	// does not.
	out := fare.Segment{Carrier: "BA", Origin: "LHR", Destination: "JFK", Class: "V", Depart: dep}
	back := fare.Segment{Carrier: "BA", Origin: "JFK", Destination: "LHR", Class: "V", Depart: dep.AddDate(0, 0, 7)}
	if _, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{out, back}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -40)}); err != nil {
		t.Errorf("a week in New York on the round-trip fare: %v", err)
	}
	back.Depart = dep.AddDate(0, 0, 1)
	var nf *fare.ErrNoFare
	if _, err := fare.Price(tf, fare.Request{Segments: []fare.Segment{out, back}, Passengers: []fare.PaxType{fare.Adult}, Purchased: dep.AddDate(0, 0, -40)}); !errors.As(err, &nf) || nf.Reason != "Saturday night stay required" {
		t.Errorf("home the next day: %v", err)
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
	if priced[0].Class != "Y" || priced[0].Fare != 20000 || priced[len(priced)-1].Fare != Share("N")*20000 {
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

// A market multiplier scales one carrier's fares on one market on top of
// the carrier's own, and nowhere else.
func TestMarketMultiplierIsOneMarketsAlone(t *testing.T) {
	m := &world.Manifest{Flights: []world.Flight{
		{Carrier: "BA", From: "LHR", To: "JFK", KM: 5550}, {Carrier: "BA", From: "LHR", To: "BOS", KM: 5250},
	}}
	tf := FromManifest(m)
	jfk, bos := tf.Fares("BA", "LHR", "JFK")[0].OneWay.Amount, tf.Fares("BA", "LHR", "BOS")[0].OneWay.Amount
	tf.SetMarketMultiplier("BA", "LHR", "JFK", 0.8)
	if got := tf.Fares("BA", "LHR", "JFK")[0].OneWay.Amount; got >= jfk || tf.EffectiveMultiplier("BA", "LHR", "JFK") != 0.8 {
		t.Errorf("JFK market not cut: %d from %d", got, jfk)
	}
	if got := tf.Fares("BA", "LHR", "BOS")[0].OneWay.Amount; got != bos {
		t.Errorf("BOS moved: %d from %d", got, bos)
	}
	tf.SetMultiplier("BA", 1.5)
	if e := tf.EffectiveMultiplier("BA", "LHR", "JFK"); e < 1.19 || e > 1.21 {
		t.Errorf("carrier and market multipliers compound: %.2f", e)
	}
	tf.SetMarketMultiplier("BA", "LHR", "JFK", 0)
	if tf.EffectiveMultiplier("BA", "LHR", "JFK") != 1.5 {
		t.Error("market restored to the carrier's filing")
	}
}

// Who stays away. A fare that can be refunded has twice the no-shows of
// one that cannot. Over the classes a main cabin sells, the average is
// 0.092*0.17 + 0.908*0.085 = 0.093 of bookings, against the "about 10
// percent" of the Government Accountability Office's report (GAO-20-191).
func TestNoShowByFare(t *testing.T) {
	for class, want := range map[string]float64{"F": 0.17, "J": 0.17, "C": 0.17, "Y": 0.17, "D": 0.085, "B": 0.085, "Q": 0.085, "N": 0.085, "?": 0.085} {
		if got := NoShow(class); got != want {
			t.Errorf("class %s: %.3f of bookings do not come, want %.3f", class, got, want)
		}
	}
	var booked, away float64
	for _, d := range Forecast("Y", 180) {
		booked += d.Mean
		away += d.Mean * NoShow(d.Class)
	}
	if rate := away / booked; rate < 0.09 || rate > 0.10 {
		t.Errorf("over a main cabin's classes %.4f of bookings do not come, want 0.093", rate)
	}
}
