// Package tariff is the world's fare filing: what every carrier charges on
// every market it sells, derived from the schedule rather than filed,
// because the real filings are ATPCO's and licensed.
//
// The level is measured. The US Department of Transportation's Consumer
// Airfare Report gives the average one-way fare paid in the thousand
// largest domestic markets; over the four quarters to March 2026 it is
// 153 dollars plus 7.74 cents a mile, weighted by passengers, and the
// carrier with the lowest fare in a market averaged 75 per cent of the
// market's fare (internal/refdata holds the report; a test fits the line
// again). A market's fares here are set so that what travellers pay on
// average comes out near that line, scaled by region and by the kind of
// carrier.
//
// The structure is the industry's. Each market has a ladder of booking
// classes, each with a fare basis, an amount and rules, and the ratios
// between the rungs and their advance purchase days follow the ladders
// Peter Belobaba's MIT lectures print: a full fare three and a half
// times the lowest, sold to the last minute and refundable; discounts
// that want three, seven, fourteen and twenty-one days' notice and
// cannot be refunded. A network carrier's cheapest international fares
// are round-trip fares with a Saturday night, the oldest fence there
// is; a low-cost carrier's ladder has none. A carrier sells a through
// fare between any two points it connects over its own network, a
// little under what a nonstop costs, and lets its partners' flights ride
// that fare. Some fares are filed for the carrier's own channels only,
// and some for a contract. Taxes are the United States' and the United
// Kingdom's as they stood in 2026; other countries' are not modelled.
package tariff

import (
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/refdata"
	"github.com/adamf/wholesky/internal/world"
)

// The fit to the Consumer Airfare Report, 2025 Q2 to 2026 Q1: the average
// one-way fare in a market is FareBase dollars plus FarePerMile a mile.
const (
	FareBase    = 153.0
	FarePerMile = 0.0774
	kmPerMile   = 1.609344
	// LowestShare is what the carrier with the lowest average fare in a
	// market charged, as a share of the market's average.
	LowestShare = 0.749
)

// mixRatio is the average fare travellers pay over the lowest regular
// fare a network carrier files. It is measured, not chosen: the world's
// travellers (internal/market) buy a filled day over the ladder, under
// the class limits each flight's own record sets (internal/rm), and the
// ratio is the one at which what they paid averages the report's line.
// The test of internal/fill holds it there, and says which way to move
// it when the ladder, the limits or the travellers change. It was 1.53
// when every flight sold under the fixed limits below: carriers that
// know their flights fill more seats at lower fares. It was 1.275 before
// any traveller stayed away from a flight and before any cabin was sold
// past its seats.
const mixRatio = 1.335

// MixRatio is the average fare travellers pay over the lowest regular
// fare a network carrier files.
func MixRatio() float64 { return mixRatio }

// MarketFare is the average one-way fare the report's line gives for a
// market of a length, in dollars.
func MarketFare(km int) float64 { return FareBase + FarePerMile*float64(km)/kmPerMile }

// regionFare scales the US line to another part of the world. Fares
// within Europe and within Asia are lower for the distance, because
// low-cost carriers hold more of those markets and sectors are short.
// Estimates: no other country publishes what the United States does.
var regionFare = map[string]float64{
	refdata.NorthAmerica: 1.00,
	refdata.Europe:       0.60,
	refdata.Asia:         0.55,
	refdata.MiddleEast:   0.75,
	refdata.LatinAmerica: 0.70,
	refdata.Africa:       0.90,
	refdata.Oceania:      0.80,
}

// step is one rung of the ladder: the class, the basis, the fare as a
// share of full fare Y, and the rule.
type step struct {
	class, basis string
	share        float64
	rule         fare.Rule
	// restricted marks the rungs that are round-trip fares with a
	// Saturday night on a network carrier's international markets.
	restricted bool
	// web marks the rung filed for the carrier's own channels.
	web bool
}

var usd = func(cents int64) fare.Money { return fare.Money{Amount: cents, Currency: "USD"} }

// fullOverLowest is full fare over the lowest regular fare (V): the
// ladders in Belobaba's lectures run from 3.3 to 4.1.
const fullOverLowest = 3.6

var ladder = []step{
	{"F", "FOW", 2.50, fare.Rule{Refundable: true}, false, false},
	{"J", "JOW", 1.67, fare.Rule{Refundable: true}, false, false},
	{"C", "COW", 1.39, fare.Rule{Refundable: true}, false, false},
	{"D", "DA3", 1.11, fare.Rule{AdvancePurchaseDays: 3, ChangeFee: usd(15000)}, false, false},
	{"Y", "YOW", 1.00, fare.Rule{Refundable: true}, false, false},
	{"B", "BA3", 2.80 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 3, ChangeFee: usd(7500)}, false, false},
	{"M", "MA3", 2.00 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 3, ChangeFee: usd(7500)}, false, false},
	{"H", "HA7", 1.70 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 7, ChangeFee: usd(9900)}, false, false},
	{"Q", "QA7", 1.45 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 7, ChangeFee: usd(9900)}, false, false},
	{"K", "KA14", 1.25 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 14, ChangeFee: usd(12500)}, false, false},
	{"L", "LA14", 1.12 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 14, ChangeFee: usd(12500)}, true, false},
	{"V", "VA21", 1.00 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 21, ChangeFee: usd(15000)}, true, false},
	{"S", "SA21WEB", 0.90 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 21, ChangeFee: usd(15000)}, true, true},
	{"N", "NA30", 0.80 / fullOverLowest, fare.Rule{AdvancePurchaseDays: 30, NoChanges: true}, true, false},
}

// The share of booked travellers who do not come for the flight. The
// Government Accountability Office's report on denied boarding
// (GAO-20-191, 2019) has no-show rates at about 10 percent of the
// bookings held at departure, down from 12 to 15, because more tickets
// are sold with restrictions. No public source gives the rate by fare.
// The 2 rates here are an estimate: a traveller on a fare that can be
// refunded stays away twice as often as one on a fare that cannot, and
// the average over the classes travellers buy is about 0.09. The
// passengers who miss a connection are counted apart.
const (
	noShowFlexible   = 0.17
	noShowRestricted = 0.085
)

// NoShow is the share of the travellers booked in a class who do not
// come for the flight.
func NoShow(class string) float64 {
	for _, s := range ladder {
		if s.class == class {
			if s.rule.Refundable {
				return noShowFlexible
			}
			return noShowRestricted
		}
	}
	return noShowRestricted
}

// Ladder is the booking classes in fare order, highest first, for the
// revenue management ladder and for a seller choosing a cheaper class.
func Ladder() []string {
	out := make([]string, 0, len(ladder))
	for _, s := range ladder {
		out = append(out, s.class)
	}
	return out
}

// Share is a class's fare as a share of full fare Y.
func Share(class string) float64 {
	for _, s := range ladder {
		if s.class == class {
			return s.share
		}
	}
	return 0
}

type place struct {
	lat, lon        float64
	country, region string
}

// Synthetic is a fare.Tariff computed from the schedule.
type Synthetic struct {
	// Partners, when set, names the carriers whose flights may ride a
	// carrier's through fares: its alliance and codeshare partners. A
	// connection between two carriers that are not partners is paid for
	// with a fare for each flight, which costs more.
	Partners func(carrier string) []string
	// Surcharge, when set, is a carrier's own fee on a segment, in cents:
	// the fuel surcharge a network carrier adds to a long flight.
	Surcharge func(carrier string, km int, international bool) int64
	// WebOnly, when set, says a carrier keeps its lowest fares to its own
	// channels: the content it withholds from the distribution systems.
	WebOnly func(carrier string) bool
	// Corporate is the discount a contract takes off the middle of the
	// ladder, as a share; zero files no contract fares. The account code
	// that buys them is CorporateAccount.
	Corporate float64

	mu         sync.RWMutex
	km         map[string]int                 // carrier/from/to -> great-circle km of a flown leg
	legs       map[string]map[string][]string // carrier -> from -> the points it flies to
	airports   []string
	places     map[string]place
	mult       map[string]float64
	marketMult map[string]float64 // carrier/from/to
	through    map[string]*via    // carrier/from/to -> how it connects, nil when it does not
}

// CorporateAccount is the account code the tariff's contract fares are
// filed for.
const CorporateAccount = "CORP"

type via struct {
	points []string
	km     int
}

// FromManifest builds the tariff for every market a carrier flies.
func FromManifest(m *world.Manifest) *Synthetic {
	t := &Synthetic{km: map[string]int{}, legs: map[string]map[string][]string{}, places: map[string]place{}, through: map[string]*via{}, Corporate: 0.15}
	for _, a := range m.Airports {
		t.airports = append(t.airports, a.IATA)
		t.places[a.IATA] = place{a.Lat, a.Lon, a.Country, refdata.Region(a.Country, a.TZ)}
	}
	t.add(m.Flights)
	return t
}

func (t *Synthetic) add(flights []world.Flight) {
	leg := func(carrier, from, to string, km int) {
		k := carrier + "/" + from + "/" + to
		if _, ok := t.km[k]; ok {
			return
		}
		t.km[k] = km
		if t.legs[carrier] == nil {
			t.legs[carrier] = map[string][]string{}
		}
		t.legs[carrier][from] = append(t.legs[carrier][from], to)
	}
	for _, f := range flights {
		leg(f.Carrier, f.From, f.To, f.KM)
		if f.Marketing != "" && f.Marketing != f.Carrier {
			leg(f.Marketing, f.From, f.To, f.KM)
		}
	}
	t.through = map[string]*via{}
}

// AddFlights files fares for markets the tariff did not know: a joined
// world's flights, so this world's sellers can price them.
func (t *Synthetic) AddFlights(flights []world.Flight) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.add(flights)
}

// AddAirports tells the tariff where airports it did not know are.
func (t *Synthetic) AddAirports(airports []world.Airport) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, a := range airports {
		if _, ok := t.places[a.IATA]; !ok {
			t.airports = append(t.airports, a.IATA)
			t.places[a.IATA] = place{a.Lat, a.Lon, a.Country, refdata.Region(a.Country, a.TZ)}
		}
	}
}

// clampMultiplier keeps a fare multiplier within what a fare can bear:
// a tenth of the filing to ten times it. Zero and below restore the
// filing; a number past the band is the band's edge, not an overflow.
func clampMultiplier(m float64) float64 {
	switch {
	case m <= 0:
		return m
	case m < 0.1:
		return 0.1
	case m > 10:
		return 10
	}
	return m
}

// SetMultiplier scales every fare a carrier files, from now on; 0 or a
// negative value restores the filing.
func (t *Synthetic) SetMultiplier(carrier string, mult float64) {
	mult = clampMultiplier(mult)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.mult == nil {
		t.mult = map[string]float64{}
	}
	if mult <= 0 {
		delete(t.mult, strings.ToUpper(carrier))
		return
	}
	t.mult[strings.ToUpper(carrier)] = mult
}

// Multiplier is a carrier's current pricing multiplier, 1 when unset.
func (t *Synthetic) Multiplier(carrier string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if m, ok := t.mult[strings.ToUpper(carrier)]; ok {
		return m
	}
	return 1
}

// SetMarketMultiplier scales one carrier's fares on one market -- a sale
// on a route a competitor has cut, a premium where it has the route to
// itself -- on top of the carrier's own; 0 restores the filing there.
func (t *Synthetic) SetMarketMultiplier(carrier, origin, destination string, mult float64) {
	mult = clampMultiplier(mult)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.marketMult == nil {
		t.marketMult = map[string]float64{}
	}
	k := strings.ToUpper(carrier + "/" + origin + "/" + destination)
	if mult <= 0 {
		delete(t.marketMult, k)
		return
	}
	t.marketMult[k] = mult
}

// MarketMultiplier is a carrier's multiplier on one market alone, 1 when
// unset.
func (t *Synthetic) MarketMultiplier(carrier, origin, destination string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if v, ok := t.marketMult[strings.ToUpper(carrier+"/"+origin+"/"+destination)]; ok {
		return v
	}
	return 1
}

// EffectiveMultiplier is what a carrier's fares on a market are scaled by
// now: the carrier's multiplier times the market's.
func (t *Synthetic) EffectiveMultiplier(carrier, origin, destination string) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	m := 1.0
	if v, ok := t.mult[strings.ToUpper(carrier)]; ok {
		m = v
	}
	if v, ok := t.marketMult[strings.ToUpper(carrier+"/"+origin+"/"+destination)]; ok {
		m *= v
	}
	return m
}

// Multipliers is every multiplier set, for a machine that must hold the
// same prices: by carrier, and by carrier/origin/destination.
func (t *Synthetic) Multipliers() (carriers, markets map[string]float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	carriers, markets = map[string]float64{}, map[string]float64{}
	for k, v := range t.mult {
		carriers[k] = v
	}
	for k, v := range t.marketMult {
		markets[k] = v
	}
	return carriers, markets
}

// carrierFare is where a carrier's ladder sits against the market's
// average: a network carrier a little above it, a low-cost carrier well
// below, an ultra-low-cost carrier at half. The ultra-low-cost level is
// measured as mixRatio is: at half, what such a carrier's travellers pay
// comes to the report's lowest-fare share, 0.749 of the market, because
// it sells its dearer classes on its fuller flights. Each carrier is a
// few per cent off its kind, so two carriers of a kind do not file the
// same fare.
func carrierFare(carrier string) float64 { return Level(carrier) }

// Level is where a carrier's fares sit at a multiplier of one, as a
// share of the market's average: what a pricing desk compares when it
// asks who is under whom.
func Level(carrier string) float64 {
	h := fnv.New32a()
	h.Write([]byte(carrier))
	spread := 0.95 + float64(h.Sum32()%11)/100 // 0.95 .. 1.05
	switch equipment.ModelOf(carrier) {
	case equipment.LowCost:
		return 0.80 * spread
	case equipment.UltraLowCost:
		return 0.50 * spread
	}
	return 1.06 * spread
}

func (t *Synthetic) regionOf(origin, destination string) (factor float64, international, longHaul bool) {
	a, b := t.places[origin], t.places[destination]
	international = a.country != b.country
	if a.region == b.region {
		f, ok := regionFare[a.region]
		if !ok {
			f = 0.8
		}
		return f, international, false
	}
	return 1, international, true
}

// BaseFare is the market's reference fare in cents: the lowest regular
// fare a network carrier would file there before anybody moved a price.
// A traveller's willingness to pay is measured from it.
func (t *Synthetic) BaseFare(origin, destination string, km int) int64 {
	t.mu.RLock()
	f, _, _ := t.regionOf(strings.ToUpper(origin), strings.ToUpper(destination))
	t.mu.RUnlock()
	return int64(MarketFare(km)*f/mixRatio) * 100
}

// Distance is the great-circle kilometres between two airports the
// tariff knows, or zero.
func (t *Synthetic) Distance(a, b string) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.distance(strings.ToUpper(a), strings.ToUpper(b))
}

func (t *Synthetic) distance(a, b string) int {
	pa, ok1 := t.places[a]
	pb, ok2 := t.places[b]
	if !ok1 || !ok2 {
		return 0
	}
	const r = 6371.0
	p1, p2 := pa.lat*math.Pi/180, pb.lat*math.Pi/180
	dp, dl := (pb.lat-pa.lat)*math.Pi/180, (pb.lon-pa.lon)*math.Pi/180
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return int(2 * r * math.Asin(math.Min(1, math.Sqrt(h))))
}

// Miles is the ticketed mileage between two airports, for the pricing
// that divides a through fare by it.
func (t *Synthetic) Miles(a, b string) int { return int(float64(t.Distance(a, b)) / kmPerMile) }

// connects works out whether and how a carrier sells a market it does
// not fly nonstop: over a point it flies to from the origin, with its own
// flight or a partner's onward, or a partner's flight in and its own out.
// Called with the write lock held or after taking it.
func (t *Synthetic) connects(carrier, origin, destination string) *via {
	key := carrier + "/" + origin + "/" + destination
	if v, ok := t.through[key]; ok {
		return v
	}
	var partners []string
	if t.Partners != nil {
		partners = t.Partners(carrier)
	}
	flies := func(c, from, to string) bool {
		_, ok := t.km[c+"/"+from+"/"+to]
		return ok
	}
	seen := map[string]bool{}
	var points []string
	consider := func(first string, onward []string) {
		for _, h := range t.legs[first][origin] {
			if seen[h] || h == destination {
				continue
			}
			for _, c := range onward {
				if (first == carrier || c == carrier) && flies(c, h, destination) {
					seen[h] = true
					points = append(points, h)
					break
				}
			}
		}
	}
	consider(carrier, append([]string{carrier}, partners...))
	for _, p := range partners {
		consider(p, []string{carrier})
	}
	var v *via
	if len(points) > 0 {
		sort.Strings(points)
		if len(points) > 12 {
			points = points[:12]
		}
		v = &via{points: points, km: t.distance(origin, destination)}
	}
	if len(t.through) > 400000 {
		t.through = map[string]*via{}
	}
	t.through[key] = v
	return v
}

// Fares implements fare.Tariff.
func (t *Synthetic) Fares(carrier, origin, destination string) []fare.Fare {
	carrier, origin, destination = strings.ToUpper(carrier), strings.ToUpper(origin), strings.ToUpper(destination)
	t.mu.RLock()
	km, nonstop := t.km[carrier+"/"+origin+"/"+destination]
	cached, known := t.through[carrier+"/"+origin+"/"+destination]
	t.mu.RUnlock()
	var routing fare.Routing
	var carriers []string
	level := 1.0
	if !nonstop {
		if !known {
			t.mu.Lock()
			cached = t.connects(carrier, origin, destination)
			t.mu.Unlock()
		}
		if cached == nil || cached.km == 0 {
			return nil
		}
		// A connection is sold a little under a nonstop over the same
		// distance: nonstop fares ran 8 per cent above connecting ones on
		// the Atlantic.
		km, level = cached.km, 0.92
		routing = fare.Routing{Via: cached.points}
		if t.Partners != nil {
			carriers = t.Partners(carrier)
		}
	}
	t.mu.RLock()
	region, international, longHaul := t.regionOf(origin, destination)
	t.mu.RUnlock()
	lowest := MarketFare(km) * region * carrierFare(carrier) / mixRatio * level * t.EffectiveMultiplier(carrier, origin, destination)
	full := lowest * fullOverLowest
	network := equipment.ModelOf(carrier) == equipment.Network
	web := t.WebOnly != nil && t.WebOnly(carrier)
	out := make([]fare.Fare, 0, len(ladder)+2)
	for _, s := range ladder {
		f := fare.Fare{Carrier: carrier, Origin: origin, Destination: destination, Class: s.class, Basis: s.basis,
			OneWay: usd(int64(full*s.share) * 100), Rule: s.rule, Routing: routing, Directional: true}
		f.Rule.Carriers = carriers
		if !international {
			// Within a country a connection is not a place to stay.
			f.Rule.MaxStopHours = 4
		}
		if s.restricted && network && international && longHaul {
			f.RoundTrip = true
			f.Rule.SaturdayNight = true
			f.Rule.UnitCarriers = carriers
			f.Basis = s.class + "LX" + f.Basis[2:]
		}
		if s.web || (web && s.restricted) {
			f.Rule.Sale.Channels = []string{"direct", "ndc"}
		}
		out = append(out, f)
		if t.Corporate > 0 && network && (s.class == "B" || s.class == "M") {
			// The contract's fare: the same class and rules at a discount,
			// sold to the account alone.
			c := f
			c.Basis = s.class + "CORP"
			c.OneWay = usd(int64(full*s.share*(1-t.Corporate)) * 100)
			c.Rule.Sale.Accounts = []string{CorporateAccount}
			out = append(out, c)
		}
	}
	return out
}

// TaxesFor implements fare.Tariff: the shape of a US domestic ticket in
// 2026. TaxesOn is asked instead when the itinerary is known.
func (t *Synthetic) TaxesFor(carrier string) []fare.Tax {
	return []fare.Tax{
		{Code: "US", Kind: fare.PercentOfBase, Percent: 7.5},
		{Code: "ZP", Kind: fare.PerSegment, Amount: usd(530)},
		{Code: "AY", Kind: fare.PerTicket, Amount: usd(560)},
		{Code: "XF", Kind: fare.PerEnplanement, Amount: usd(450), Airports: t.airports},
	}
}

// TaxesOn implements fare.Taxer: the taxes an itinerary carries, from
// where it goes. Within the United States it is the 7.5 per cent excise,
// 5.30 dollars a segment, the 5.60 dollar security fee and a 4.50
// dollar facility charge at each departure, as Airlines for America
// lists them for 2026; to or from the United States the 23.40 dollar
// international tax in place of the excise. From the United Kingdom it
// is Air Passenger Duty at the reduced rates of April 2026: 8 pounds
// domestic, 15 to 2,000 miles, 102 beyond. Other countries' taxes are
// not modelled. A carrier's own surcharge is added where it has one.
func (t *Synthetic) TaxesOn(segs []fare.Segment) []fare.Tax {
	if len(segs) == 0 {
		return nil
	}
	t.mu.RLock()
	first := t.places[strings.ToUpper(segs[0].Origin)]
	usa, abroad := false, false
	var surcharge int64
	for _, s := range segs {
		a, b := t.places[strings.ToUpper(s.Origin)], t.places[strings.ToUpper(s.Destination)]
		if a.country == "United States" || b.country == "United States" {
			usa = true
		}
		if a.country != b.country {
			abroad = true
		}
		if t.Surcharge != nil {
			surcharge += t.Surcharge(strings.ToUpper(s.Carrier), t.distance(strings.ToUpper(s.Origin), strings.ToUpper(s.Destination)), a.country != b.country)
		}
	}
	last := strings.ToUpper(segs[len(segs)-1].Destination)
	trip := t.distance(strings.ToUpper(segs[0].Origin), last)
	t.mu.RUnlock()
	var out []fare.Tax
	switch {
	case usa && !abroad:
		out = t.TaxesFor(segs[0].Carrier)
	case usa:
		out = []fare.Tax{{Code: "US", Kind: fare.PerTicket, Amount: usd(2340)}, {Code: "AY", Kind: fare.PerTicket, Amount: usd(560)}}
	case first.country == "United Kingdom":
		duty := int64(1900) // 15 pounds
		switch {
		case !abroad:
			duty = 1000
		case float64(trip)/kmPerMile > 2000:
			duty = 13000
		}
		out = []fare.Tax{{Code: "GB", Kind: fare.PerTicket, Amount: usd(duty)}}
	}
	if surcharge > 0 {
		out = append(out, fare.Tax{Code: "YQ", Kind: fare.PerTicket, Amount: usd(surcharge), CarrierImposed: true})
	}
	return out
}

// Forecast is the revenue management forecaster's view of a cabin: the
// demand still to come by class, with the fare each sells at, for EMSR-b
// to set the authorisations from. It is the forecast for a flight that
// has no record of its own yet (internal/rm): a new service in its first
// week, or a world whose carriers keep no record. The class mix is the
// one the world's travellers buy in on the fixed limits below, measured
// over a filled day: about one in eleven at full fare, one in eight in
// the three classes under it, the rest down the discounts. The total is a
// little above the cabin -- a flight with less demand than seats needs no
// managing -- and the spread a Poisson's and a quarter, because parties
// travel together. Only the fare ratios matter to the method, so the
// ladder's shares of full fare stand in for the market's fares.
func Forecast(compartment string, seats int) []inventory.ClassDemand {
	type mix struct {
		class string
		share float64
	}
	var m []mix
	switch compartment {
	case "F":
		m = []mix{{"F", 1}}
	case "C":
		// The front cabin's mix, measured the same way: most of its
		// travellers take the fare they can get back.
		m = []mix{{"J", 0.01}, {"C", 0.62}, {"D", 0.37}}
	default:
		m = []mix{{"Y", .092}, {"B", .004}, {"M", .049}, {"H", .077}, {"Q", .158}, {"K", .179}, {"L", .150}, {"V", .055}, {"S", .083}, {"N", .153}}
	}
	total := float64(seats) * 1.1
	out := make([]inventory.ClassDemand, 0, len(m))
	for _, x := range m {
		mean := total * x.share
		out = append(out, inventory.ClassDemand{Class: x.class, Fare: Share(x.class), Mean: mean, StdDev: math.Sqrt(mean) * 1.25})
	}
	return out
}

// Pickup is the forecaster reading the booking curve: what a cabin has
// sold so far by class, plus the share of its baseline demand still to
// come. The share is the caller's -- how much of the selling window is
// left before departure, one at the start and nought at the door -- and
// the spread is only on what is to come, because what is sold is known.
// Total demand for the cabin is then sold plus pickup: the additive pickup
// method, the textbook one. A flight selling ahead of its curve forecasts
// more and protects harder; one selling behind forecasts less and the
// discounts reopen, which is what a revenue manager watching the curve
// would do by hand.
func Pickup(compartment string, seats int, sold map[string]int, remaining float64) []inventory.ClassDemand {
	return PickupPriced(compartment, seats, sold, remaining, 0)
}

// PickupPriced is Pickup with the fares in money: each class's share of
// the market's full fare, in minor units, so the ladder's bid prices can
// be held against what a connecting passenger actually pays. A zero full
// fare leaves the fares as shares of one.
func PickupPriced(compartment string, seats int, sold map[string]int, remaining float64, fullFare int64) []inventory.ClassDemand {
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 1 {
		remaining = 1
	}
	base := Forecast(compartment, seats)
	price := func(class string) float64 {
		if fullFare > 0 {
			return Share(class) * float64(fullFare)
		}
		return Share(class)
	}
	for i := range base {
		base[i].Fare = price(base[i].Class)
	}
	return PickupFrom(base, sold, remaining, price)
}

// PickupFrom is the pickup forecast on any baseline: for each class, what
// it has sold plus the share of the baseline's demand still to come, at
// the baseline's fare, with the baseline's spread on what is to come. The
// baseline is the fixed mix of Forecast, or a flight's own record of its
// booking window (internal/rm), in which case the fares are the marginal
// revenues the record gives and a route with more late demand protects
// more. fare prices a class the baseline does not name.
func PickupFrom(base []inventory.ClassDemand, sold map[string]int, remaining float64, fare func(class string) float64) []inventory.ClassDemand {
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 1 {
		remaining = 1
	}
	out := make([]inventory.ClassDemand, 0, len(base))
	seen := map[string]bool{}
	for _, c := range base {
		pickup := c.Mean * remaining
		out = append(out, inventory.ClassDemand{Class: c.Class, Fare: c.Fare, Mean: float64(sold[c.Class]) + pickup, StdDev: c.StdDev * math.Sqrt(remaining)})
		seen[c.Class] = true
	}
	// A class sold that the baseline does not name -- an override, an odd
	// booking -- is demand all the same, at its ladder fare.
	for class, n := range sold {
		if !seen[class] && n > 0 {
			out = append(out, inventory.ClassDemand{Class: class, Fare: fare(class), Mean: float64(n)})
		}
	}
	return out
}

// FullFare is the market's Y one-way in minor units as the tariff files it,
// or zero when the tariff has no fare for the market.
func FullFare(t fare.Tariff, carrier, origin, destination string) int64 {
	if t == nil {
		return 0
	}
	for _, f := range t.Fares(carrier, origin, destination) {
		if f.Class == "Y" {
			return f.OneWay.Amount
		}
	}
	return 0
}

// AverageFare is what a traveller pays a carrier on a market on average,
// in minor units: its lowest regular fare times the travellers' mix of
// classes. It is the figure a projection values a passenger at.
func AverageFare(t fare.Tariff, carrier, origin, destination string) int64 {
	return int64(float64(FullFare(t, carrier, origin, destination)) / fullOverLowest * mixRatio)
}

// Authorizations is the class ladder's nested authorisation levels for a
// cabin of the given seats: the share of the cabin each class and those
// below it may take. Full fare takes the cabin; each cheaper class is held
// to less, so the last seats go at the higher fares. Classes the cabin
// does not sell are left off.
func Authorizations(compartment string, seats int) []struct {
	Class      string
	Authorized int
} {
	type lvl = struct {
		Class      string
		Authorized int
	}
	var shares []struct {
		class string
		share float64
	}
	switch compartment {
	case "F":
		shares = []struct {
			class string
			share float64
		}{{"F", 1}}
	case "C":
		shares = []struct {
			class string
			share float64
		}{{"J", 1}, {"C", 0.9}, {"D", 0.6}}
	default:
		shares = []struct {
			class string
			share float64
		}{{"Y", 1}, {"B", 0.95}, {"M", 0.85}, {"H", 0.75}, {"Q", 0.62}, {"K", 0.50}, {"L", 0.38}, {"V", 0.28}, {"S", 0.28}, {"N", 0.18}}
	}
	out := make([]lvl, 0, len(shares))
	for _, s := range shares {
		out = append(out, lvl{Class: s.class, Authorized: int(float64(seats)*s.share + 0.5)})
	}
	return out
}
