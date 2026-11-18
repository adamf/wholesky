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
