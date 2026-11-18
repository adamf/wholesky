package distribution

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"sort"

	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/refdata"
)

// Agency is a seller that is not the carrier: an online travel agency, a
// travel management company, or the travel agents of a part of the world
// taken together.
//
// The agencies of a world are invented. Their names are nobody's, and
// what each earns comes from the industry's public figures and not from
// any company's. An agency books through one distribution system, which
// pays it an incentive for every segment out of the fee the carrier
// pays. A carrier may pay it a commission or an override on the fare. It
// may charge the traveller a fee of its own. An agency with a direct
// connection books at a carrier's own system where the carrier offers
// one, and the distribution system is paid nothing for that ticket.
type Agency struct {
	// Code is the agency's numeric code, eight digits with a check digit,
	// in the form of an IATA agency number.
	Code string `json:"code"`
	Name string `json:"name"`
	// Kind is market.OTA, market.TMC or market.Agency.
	Kind string `json:"kind"`
	// Region is the part of the world it sells in, or "" for all of it.
	Region string `json:"region,omitempty"`
	// System is the distribution system it books through.
	System string `json:"system"`
	// Weight is its share of its kind's sales where it sells.
	Weight float64 `json:"weight"`
	// Fee is what it charges the traveller for a ticket, in cents.
	Fee int64 `json:"fee"`
	// Connect says it has a direct connection to the carriers that offer
	// one, and ConnectShare how much of its sales on those carriers goes
	// that way.
	Connect      bool    `json:"connect,omitempty"`
	ConnectShare float64 `json:"connect_share,omitempty"`
}

// What the agencies charge travellers, in cents. The large online
// agencies charge nothing for a flight. A regional one charges five
// dollars. A travel management company's fee is paid by the traveller's
// employer for each transaction and is not in the price the traveller
// compares. A travel agent charges 25 dollars: the commission that
// reported to the US Congress in 2002 on travel distribution heard that
// travellers paid service fees up to that without going elsewhere, and
// would resist at 40. The first three are estimates.
const (
	feeRegionalOTA = 500
	feeTMC         = 1500
	feeAgent       = 2500
)

// connectShare is how much of a connected agency's sales on a carrier
// with a direct connection goes over it. Lufthansa Group said half of
// its indirect bookings were NDC by the middle of 2026. One carrier's
// figure, used for all: an estimate.
const connectShare = 0.5

// Agencies is the sellers of a world whose distribution systems are
// named: three online agencies that sell everywhere, one more in each of
// three regions, three travel management companies, and the travel
// agents of each region. The same systems give the same agencies.
func Agencies(systems []string) []Agency {
	if len(systems) == 0 {
		return nil
	}
	sys := append([]string(nil), systems...)
	sort.Strings(sys)
	var out []Agency
	add := func(name, kind, region string, weight float64, fee int64, connect bool) {
		a := Agency{Name: name, Kind: kind, Region: region, Weight: weight, Fee: fee, Connect: connect, System: sys[len(out)%len(sys)]}
		if connect {
			a.ConnectShare = connectShare
		}
		a.Code = code(name)
		out = append(out, a)
	}
	add("Meridian Travel", market.OTA, "", 0.40, 0, true)
	add("Farewise", market.OTA, "", 0.30, 0, true)
	add("Tripline", market.OTA, "", 0.15, 0, false)
	add("Vela Reisen", market.OTA, refdata.Europe, 0.15, feeRegionalOTA, false)
	add("Skylantern", market.OTA, refdata.Asia, 0.15, feeRegionalOTA, false)
	add("Andes Viajes", market.OTA, refdata.LatinAmerica, 0.15, feeRegionalOTA, false)
	add("Harcourt Business Travel", market.TMC, "", 0.40, feeTMC, true)
	add("Linden and Roe", market.TMC, "", 0.35, feeTMC, false)
	add("Corvus Travel Management", market.TMC, "", 0.25, feeTMC, false)
	for _, r := range []string{refdata.NorthAmerica, refdata.LatinAmerica, refdata.Europe, refdata.MiddleEast, refdata.Africa, refdata.Asia, refdata.Oceania} {
		add("Travel agents of "+r, market.Agency, r, 1, feeAgent, false)
	}
	return out
}

// code makes an agency's number from its name: seven digits and the
// remainder of them over seven, which is how an IATA agency number
// checks itself.
func code(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	n := 1000000 + h.Sum32()%9000000
	return fmt.Sprintf("%07d%d", n, n%7)
}

// Valid reports whether a string has the form of an agency's number.
func Valid(code string) bool {
	if len(code) != 8 {
		return false
	}
	n := 0
	for _, r := range code[:7] {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return int(code[7]-'0') == n%7
}

// Pick draws the agency of a kind that a traveller in a region books
// with, by the agencies' weights. ok is false when the world has none.
func Pick(rng *rand.Rand, agencies []Agency, kind, region string) (Agency, bool) {
	var total float64
	for _, a := range agencies {
		if a.Kind == kind && (a.Region == "" || a.Region == region) {
			total += a.Weight
		}
	}
	if total == 0 {
		// No agency of the kind sells here: any of the kind.
		for _, a := range agencies {
			if a.Kind == kind {
				total += a.Weight
			}
		}
		region = ""
		if total == 0 {
			return Agency{}, false
		}
	}
	u := rng.Float64() * total
	var last Agency
	for _, a := range agencies {
		if a.Kind != kind || !(a.Region == "" || a.Region == region || region == "") {
			continue
		}
		last = a
		if u < a.Weight {
			return a, true
		}
		u -= a.Weight
	}
	return last, true
}
