// Package tariff is the world's fare filing: what every carrier charges on
// every market it flies, derived from the schedule rather than filed,
// because the real filings are ATPCO's and licensed.
//
// The structure is the real one -- a ladder of booking classes per market,
// each with a fare basis, an amount and rules -- and the numbers are
// invented from distance and carrier, labelled synthetic like the seat
// counts. Full fare Y sells to the last minute and refunds; each step down
// the ladder is cheaper, wants more days' notice, and costs more to
// change. Taxes are shaped like a US domestic ticket: a percentage excise,
// a segment fee, a security fee per ticket, and a facility charge per
// enplanement.
package tariff

import (
	"hash/fnv"
	"strings"

	"github.com/adamf/jetway/pkg/fare"

	"github.com/adamf/wholesky/internal/world"
)

// step is one rung of the ladder: the class, the basis suffix, the share
// of full fare, and the rule.
type step struct {
	class, basis string
	share        float64
	rule         fare.Rule
}

var usd = func(cents int64) fare.Money { return fare.Money{Amount: cents, Currency: "USD"} }

var ladder = []step{
	{"F", "FOW", 3.2, fare.Rule{Refundable: true}},
	{"J", "JOW", 2.4, fare.Rule{Refundable: true}},
	{"C", "COW", 2.2, fare.Rule{Refundable: true}},
	{"D", "DHE3", 1.8, fare.Rule{AdvancePurchaseDays: 3, ChangeFee: usd(15000)}},
	{"Y", "YOW", 1.0, fare.Rule{Refundable: true}},
	{"B", "BHE3", 0.80, fare.Rule{AdvancePurchaseDays: 3, ChangeFee: usd(7500)}},
	{"M", "MHE7", 0.65, fare.Rule{AdvancePurchaseDays: 7, ChangeFee: usd(7500)}},
	{"H", "HLE14", 0.55, fare.Rule{AdvancePurchaseDays: 14, ChangeFee: usd(9900)}},
	{"Q", "QLX14", 0.45, fare.Rule{AdvancePurchaseDays: 14, ChangeFee: usd(9900)}},
	{"K", "KLX21", 0.38, fare.Rule{AdvancePurchaseDays: 21, ChangeFee: usd(12500)}},
	{"L", "LLX21", 0.32, fare.Rule{AdvancePurchaseDays: 21, MinStayDays: 2, ChangeFee: usd(12500)}},
	{"V", "VLX30", 0.28, fare.Rule{AdvancePurchaseDays: 30, ChangeFee: usd(15000)}},
	{"S", "SLX30", 0.28, fare.Rule{AdvancePurchaseDays: 30, ChangeFee: usd(15000)}},
	{"N", "NLX45", 0.24, fare.Rule{AdvancePurchaseDays: 45, MinStayDays: 3, ChangeFee: usd(20000)}},
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

// Synthetic is a fare.Tariff computed from the schedule's distances.
type Synthetic struct {
	km       map[string]int // carrier/from/to -> great-circle km
	airports []string
}

// FromManifest builds the tariff for every market a carrier flies.
func FromManifest(m *world.Manifest) *Synthetic {
	t := &Synthetic{km: map[string]int{}}
	for _, f := range m.Flights {
		k := f.Carrier + "/" + f.From + "/" + f.To
		if _, ok := t.km[k]; !ok {
			t.km[k] = f.KM
		}
		if f.Marketing != "" && f.Marketing != f.Carrier {
			mk := f.Marketing + "/" + f.From + "/" + f.To
			if _, ok := t.km[mk]; !ok {
				t.km[mk] = f.KM
			}
		}
	}
	for _, a := range m.Airports {
		t.airports = append(t.airports, a.IATA)
	}
	return t
}

// fullFare is the Y one-way in cents: a terminal charge plus a distance
// rate, shaded by carrier so a low-cost carrier's ladder sits below a
// legacy's on the same market.
func fullFare(carrier string, km int) int64 {
	h := fnv.New32a()
	h.Write([]byte(carrier))
	factor := 0.85 + float64(h.Sum32()%40)/100 // 0.85 .. 1.24
	cents := (8900 + int64(float64(km)*11.5)) * 100 / 100
	return int64(float64(cents) * factor)
}

// Fares implements fare.Tariff.
func (t *Synthetic) Fares(carrier, origin, destination string) []fare.Fare {
	km, ok := t.km[strings.ToUpper(carrier+"/"+origin+"/"+destination)]
	if !ok {
		return nil
	}
	full := fullFare(strings.ToUpper(carrier), km)
	out := make([]fare.Fare, 0, len(ladder))
	for _, s := range ladder {
		out = append(out, fare.Fare{
			Carrier: strings.ToUpper(carrier), Origin: strings.ToUpper(origin), Destination: strings.ToUpper(destination),
			Class: s.class, Basis: s.basis, OneWay: usd(int64(float64(full)*s.share) / 100 * 100), Rule: s.rule,
		})
	}
	return out
}

// TaxesFor implements fare.Tariff: the shape of a US domestic ticket.
func (t *Synthetic) TaxesFor(carrier string) []fare.Tax {
	return []fare.Tax{
		{Code: "US", Kind: fare.PercentOfBase, Percent: 7.5},
		{Code: "ZP", Kind: fare.PerSegment, Amount: usd(450)},
		{Code: "AY", Kind: fare.PerTicket, Amount: usd(560)},
		{Code: "XF", Kind: fare.PerEnplanement, Amount: usd(450), Airports: t.airports},
	}
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
