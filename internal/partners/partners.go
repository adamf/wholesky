// Package partners is who works with whom: the three alliances, the
// joint ventures inside them, and the terms carriers settle on when one
// flies a passenger the other ticketed.
//
// The ladder of cooperation has five rungs and each changes money. Two
// carriers with nothing but an interline agreement each sell their own
// fare for their own flight, and the carrier that flew a coupon bills
// the carrier that ticketed it the coupon's share less an interline
// service charge of nine per cent, the multilateral default. Partners in
// an alliance file through fares over each other's flights, which is
// what took a quarter off the price of an interline journey when the
// alliances began (Brueckner and Whalen, 2000) and seven per cent in
// the data to 2016 (Brueckner and Singer, 2019); they settle under
// special prorate agreements with a lower charge. Codeshare partners pay
// the carrier that sold the seat a commission. Partners in a joint
// venture with antitrust immunity pool the revenue of a group of routes
// and charge each other nothing. The membership lists are the alliances'
// own (internal/refdata). The service charge is from one secondary
// source and the commission is an estimate: no agreement is public.
package partners

import (
	"sort"
	"strings"

	"github.com/adamf/jetway/pkg/prorate"

	"github.com/adamf/wholesky/internal/refdata"
)

// Tie is how closely two carriers work together.
type Tie int

const (
	// None is two carriers with nothing between them but the multilateral
	// interline agreement.
	None Tie = iota
	// Alliance is two members of one alliance.
	Alliance
	// Codeshare is one carrier selling the other's flight under its own
	// code.
	Codeshare
	// Venture is two carriers in one immunised joint venture.
	Venture
)

func (t Tie) String() string {
	return [...]string{"interline", "alliance", "codeshare", "joint venture"}[t]
}

// JointVenture is an immunised joint venture: carriers that price and
// schedule a group of routes together and share what the routes earn.
type JointVenture struct {
	Name    string   `json:"name"`
	Parties []string `json:"parties"`
	// A and B are the two ends of the routes it covers, each a part of
	// the world or a country: a flight between the two, either way, is in
	// the venture.
	A, B string `json:"-"`
	// Scope says the same in words.
	Scope string `json:"scope"`
}

// ventures are the immunised joint ventures, from the US Department of
// Transportation's immunity orders and the European Commission's
// decisions: Star's across the Atlantic (United, Air Canada and the
// Lufthansa group) and to Japan (United and ANA); SkyTeam's (Delta, Air
// France, KLM and Virgin Atlantic; Delta and Korean); oneworld's
// (American, British Airways, Iberia and Finnair; American and Japan
// Airlines; American and Qantas).
//
// What the routes earn is pooled and shared, so that a partner does not
// care which of them carries a passenger (Brueckner and Singer, 2019).
// The Commission describes Star's and oneworld's Atlantic ventures as
// sharing revenue and SkyTeam's as sharing profit; all are taken to
// share revenue here. The formula is not public for any of them. Each
// partner's share of the pool is taken to be its share of the seats
// flown a kilometre on the routes, which is an estimate.
var ventures = []JointVenture{
	{Name: "Atlantic Plus-Plus", Parties: []string{"UA", "AC", "LH", "LX", "OS", "SN"}, A: refdata.NorthAmerica, B: refdata.Europe, Scope: "North America to Europe"},
	{Name: "United and ANA", Parties: []string{"UA", "NH"}, A: refdata.NorthAmerica, B: "Japan", Scope: "North America to Japan"},
	{Name: "SkyTeam transatlantic", Parties: []string{"DL", "AF", "KL", "VS"}, A: refdata.NorthAmerica, B: refdata.Europe, Scope: "North America to Europe"},
	{Name: "Delta and Korean", Parties: []string{"DL", "KE"}, A: refdata.NorthAmerica, B: refdata.Asia, Scope: "North America to Asia"},
	{Name: "oneworld transatlantic", Parties: []string{"AA", "BA", "IB", "AY"}, A: refdata.NorthAmerica, B: refdata.Europe, Scope: "North America to Europe"},
	{Name: "American and Japan Airlines", Parties: []string{"AA", "JL"}, A: refdata.NorthAmerica, B: "Japan", Scope: "North America to Japan"},
	{Name: "American and Qantas", Parties: []string{"AA", "QF"}, A: refdata.NorthAmerica, B: refdata.Oceania, Scope: "North America to Oceania"},
}

// Covers reports whether a flight between two airports is one of the
// venture's routes. Each airport is given by its part of the world and
// its country.
func (v JointVenture) Covers(fromRegion, fromCountry, toRegion, toCountry string) bool {
	at := func(place, region, country string) bool { return place == region || place == country }
	return (at(v.A, fromRegion, fromCountry) && at(v.B, toRegion, toCountry)) ||
		(at(v.B, fromRegion, fromCountry) && at(v.A, toRegion, toCountry))
}

// Pool shares what a venture's routes earned. revenue is what each party
// earned on them and capacity the seats each flew a kilometre there.
// Each party's due is its share of the capacity times the whole pool,
// and what comes back is that less what it earned itself: more than
// nothing for a party that flew the seats and sold fewer of them, less
// for one that sold more than its share. The answers add to nothing.
func Pool(revenue map[string]int64, capacity map[string]float64) map[string]int64 {
	var pool int64
	var seats float64
	parties := make([]string, 0, len(capacity))
	for c, k := range capacity {
		if k > 0 {
			parties = append(parties, c)
			seats += k
			pool += revenue[c]
		}
	}
	out := map[string]int64{}
	if len(parties) < 2 || seats == 0 {
		return out
	}
	sort.Strings(parties)
	var given int64
	for i, c := range parties {
		due := int64(float64(pool) * capacity[c] / seats)
		if i == len(parties)-1 {
			due = pool - given // the last takes the rounding
		}
		given += due
		out[c] = due - revenue[c]
	}
	return out
}

// Book is the world's partnerships.
type Book struct {
	codeshare map[string]bool // "marketing/operating"
	alliance  map[string]string
	members   map[string][]string
	venture   map[string]bool // "a/b", both orders
	ventures  []JointVenture
}

// New builds the book for the carriers a world has. codeshares are the
// pairs the schedule holds, marketing carrier then operating carrier.
func New(carriers []string, codeshares [][2]string) *Book {
	b := &Book{codeshare: map[string]bool{}, alliance: map[string]string{}, members: map[string][]string{}, venture: map[string]bool{}}
	for _, c := range carriers {
		c = strings.ToUpper(c)
		if a := refdata.Alliance(c); a != "" {
			b.alliance[c] = a
			b.members[a] = append(b.members[a], c)
		}
	}
	for a := range b.members {
		sort.Strings(b.members[a])
	}
	for _, p := range codeshares {
		b.codeshare[strings.ToUpper(p[0])+"/"+strings.ToUpper(p[1])] = true
	}
	in := map[string]bool{}
	for _, c := range carriers {
		in[strings.ToUpper(c)] = true
	}
	for _, v := range ventures {
		here := 0
		for _, x := range v.Parties {
			if in[x] {
				here++
			}
			for _, y := range v.Parties {
				if x != y {
					b.venture[x+"/"+y] = true
				}
			}
		}
		if here >= 2 {
			b.ventures = append(b.ventures, v)
		}
	}
	return b
}

// Ventures is the joint ventures with two or more parties in this world.
func (b *Book) Ventures() []JointVenture { return b.ventures }

// Alliance is the alliance a carrier is in, or "".
func (b *Book) Alliance(carrier string) string { return b.alliance[strings.ToUpper(carrier)] }

// Members is an alliance's carriers in this world.
func (b *Book) Members(alliance string) []string { return b.members[alliance] }

// Alliances is the alliances with a member in this world.
func (b *Book) Alliances() []string {
	out := make([]string, 0, len(b.members))
	for a := range b.members {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// Between is the closest tie two carriers have.
func (b *Book) Between(x, y string) Tie {
	x, y = strings.ToUpper(x), strings.ToUpper(y)
	switch {
	case x == y:
		return Venture
	case b.venture[x+"/"+y] && b.alliance[x] != "" && b.alliance[x] == b.alliance[y]:
		return Venture
	case b.codeshare[x+"/"+y] || b.codeshare[y+"/"+x]:
		return Codeshare
	case b.alliance[x] != "" && b.alliance[x] == b.alliance[y]:
		return Alliance
	}
	return None
}

// Of is the carriers whose flights may ride a carrier's through fares:
// its alliance and its codeshare partners in this world.
func (b *Book) Of(carrier string) []string {
	carrier = strings.ToUpper(carrier)
	seen := map[string]bool{}
	var out []string
	for _, m := range b.members[b.alliance[carrier]] {
		if m != carrier && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for k := range b.codeshare {
		p := strings.SplitN(k, "/", 2)
		var other string
		switch carrier {
		case p[0]:
			other = p[1]
		case p[1]:
			other = p[0]
		}
		if other != "" && !seen[other] {
			seen[other] = true
			out = append(out, other)
		}
	}
	sort.Strings(out)
	return out
}

// The service charges, in hundredths of a percent of the coupon's value.
// Nine per cent is the multilateral default and five to seven the usual
// range under a special prorate agreement, from one vendor's glossary; a
// joint venture charges nothing. Seven per cent for a codeshare
// commission is an estimate inside the range.
const (
	InterlineCharge = 900
	AllianceCharge  = 600
	CodeshareCharge = 700
)

// Terms is what the carrier that ticketed a coupon and the carrier that
// flew it have agreed, for jetway's prorate.Divide: straight rate
// proration with the service charge of their tie. flown is the carrier
// that flew; ticketed the one whose document it is.
func (b *Book) Terms(ticketed, flown string) *prorate.Terms {
	switch b.Between(ticketed, flown) {
	case Venture:
		return &prorate.Terms{ServiceCharge: -1}
	case Codeshare:
		return &prorate.Terms{ServiceCharge: CodeshareCharge}
	case Alliance:
		return &prorate.Terms{ServiceCharge: AllianceCharge}
	}
	return nil
}

// Rank orders the carriers a disrupted passenger can be moved to: the
// carrier's own flights, then its joint venture partners, whose seats
// are in effect its own, then its codeshare partners, then its
// alliance, then anyone with an interline agreement. The order is
// commercial practice; IATA's Resolution 735d only says the passenger
// must be carried.
func (b *Book) Rank(carrier, other string) int {
	if strings.EqualFold(carrier, other) {
		return 0
	}
	switch b.Between(carrier, other) {
	case Venture:
		return 1
	case Codeshare:
		return 2
	case Alliance:
		return 3
	}
	return 4
}
