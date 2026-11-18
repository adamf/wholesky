package partners

import (
	"testing"

	"github.com/adamf/jetway/pkg/prorate"
)

func TestTiesAndTerms(t *testing.T) {
	b := New([]string{"UA", "LH", "AC", "TP", "AA", "BA", "DL", "AF", "FR", "OO"}, [][2]string{{"UA", "OO"}})
	for _, c := range []struct {
		x, y string
		want Tie
	}{{"UA", "LH", Venture}, {"UA", "TP", Alliance}, {"UA", "OO", Codeshare}, {"OO", "UA", Codeshare},
		{"UA", "AA", None}, {"AA", "BA", Venture}, {"DL", "AF", Venture}, {"FR", "LH", None}, {"UA", "UA", Venture}} {
		if got := b.Between(c.x, c.y); got != c.want {
			t.Errorf("%s and %s: %v, want %v", c.x, c.y, got, c.want)
		}
	}
	if b.Alliance("LH") != "Star Alliance" || b.Alliance("FR") != "" || len(b.Members("Star Alliance")) != 4 || len(b.Alliances()) != 3 {
		t.Errorf("alliances: %v %v", b.Alliances(), b.Members("Star Alliance"))
	}
	of := b.Of("UA")
	if len(of) != 4 || of[0] != "AC" || of[3] != "TP" {
		t.Errorf("the carriers whose flights ride United's fares: %v", of)
	}
	if len(b.Of("FR")) != 0 {
		t.Error("a carrier in no alliance has no partners")
	}
	// What each tie charges, through the proration: a coupon worth 500
	// flown by another carrier on United's ticket.
	coupons := func(flown string) []prorate.Coupon {
		return []prorate.Coupon{{Number: 1, Carrier: "UA", Distance: 1000}, {Number: 2, Carrier: flown, Distance: 1000}}
	}
	for _, c := range []struct {
		flown  string
		charge int64
	}{{"AA", 4500}, {"TP", 3000}, {"OO", 3500}, {"LH", 0}} {
		d, err := prorate.Divide(100000, "UA", coupons(c.flown), func(carrier string) *prorate.Terms { return b.Terms("UA", carrier) }, InterlineCharge)
		if err != nil || d.Shares[1].Value != 50000 || d.Shares[1].Charge != c.charge {
			t.Errorf("a coupon flown by %s on United's ticket: %+v (%v), want a charge of %d", c.flown, d.Shares[1], err, c.charge)
		}
	}
	// Where a stranded passenger goes first.
	if !(b.Rank("UA", "UA") < b.Rank("UA", "LH") && b.Rank("UA", "LH") < b.Rank("UA", "OO") && b.Rank("UA", "OO") < b.Rank("UA", "TP") && b.Rank("UA", "TP") < b.Rank("UA", "AA")) {
		t.Error("rebooking order: own flights, joint venture, codeshare, alliance, anyone")
	}
}

// A joint venture pools what its routes earn and shares it by the seats
// each partner flew. United flies 60 per cent of the seats across the
// Atlantic and sells 70 per cent of the pool: it pays the difference to
// Lufthansa, and the two answers add to nothing.
func TestJointVenturePool(t *testing.T) {
	got := Pool(map[string]int64{"UA": 700_000_00, "LH": 300_000_00}, map[string]float64{"UA": 6e6, "LH": 4e6})
	if got["UA"] != -100_000_00 || got["LH"] != 100_000_00 {
		t.Errorf("60 per cent of the seats and 70 of the sales: %v", got)
	}
	// Three partners, with a pool that does not divide: still nothing
	// left over.
	three := Pool(map[string]int64{"AA": 1_000_01, "BA": 500_00, "IB": 0}, map[string]float64{"AA": 1, "BA": 1, "IB": 1})
	if three["AA"]+three["BA"]+three["IB"] != 0 || three["IB"] <= 0 || three["AA"] >= 0 {
		t.Errorf("three partners: %v", three)
	}
	// A party that flew nothing there is not in the pool, and one party
	// alone has nobody to share with.
	if alone := Pool(map[string]int64{"UA": 100, "LH": 50}, map[string]float64{"UA": 5, "LH": 0}); len(alone) != 0 {
		t.Errorf("one party flying: %v", alone)
	}
	// Which flights are a venture's.
	b := New([]string{"UA", "LH", "NH", "AA", "FR"}, nil)
	vs := b.Ventures()
	if len(vs) != 2 || vs[0].Name != "Atlantic Plus-Plus" || vs[1].Name != "United and ANA" {
		t.Fatalf("ventures with two parties here: %+v", vs)
	}
	atlantic, japan := vs[0], vs[1]
	for _, c := range []struct {
		v                      JointVenture
		fromR, fromC, toR, toC string
		want                   bool
	}{
		{atlantic, "North America", "United States", "Europe", "Germany", true},
		{atlantic, "Europe", "Germany", "North America", "Canada", true},
		{atlantic, "Europe", "Germany", "Europe", "France", false},
		{atlantic, "North America", "United States", "Asia", "Japan", false},
		{japan, "North America", "United States", "Asia", "Japan", true},
		{japan, "Asia", "Japan", "North America", "United States", true},
		{japan, "North America", "United States", "Asia", "China", false},
	} {
		if got := c.v.Covers(c.fromR, c.fromC, c.toR, c.toC); got != c.want {
			t.Errorf("%s covers %s to %s: %v", c.v.Name, c.fromC, c.toC, got)
		}
	}
}
