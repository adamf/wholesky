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

// ventures are the immunised joint ventures: the carriers that price and
// schedule a group of routes together and share its revenue. From the US
// Department of Transportation's immunity orders and the European
// Commission's decisions: Star's across the Atlantic (United, Air Canada
// and the Lufthansa group) and Pacific (United and ANA); SkyTeam's
// (Delta, Air France, KLM and Virgin Atlantic; Delta and Korean);
// oneworld's (American, British Airways, Iberia and Finnair; American
// and Japan Airlines; Qantas and American).
var ventures = [][]string{
	{"UA", "AC", "LH", "LX", "OS", "SN"},
	{"UA", "NH"},
	{"DL", "AF", "KL", "VS"},
	{"DL", "KE"},
	{"AA", "BA", "IB", "AY"},
	{"AA", "JL"},
	{"AA", "QF"},
}

// Book is the world's partnerships.
type Book struct {
	codeshare map[string]bool // "marketing/operating"
	alliance  map[string]string
	members   map[string][]string
	venture   map[string]bool // "a/b", both orders
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
	for _, v := range ventures {
		for _, x := range v {
			for _, y := range v {
				if x != y {
					b.venture[x+"/"+y] = true
				}
			}
		}
	}
	return b
}

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
