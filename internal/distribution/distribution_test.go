package distribution

import (
	"math/rand"
	"testing"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/refdata"
)

// What a 400-dollar round trip of four segments costs to sell, against
// the record: on the carrier's own site a dollar and a half and the card
// fee; through an agency in the United States under a full-content deal
// 2.30 a segment; for a carrier without one, 3.41 and more. The General
// Accounting Office put a distribution system's fees at 11 dollars a
// ticket in 2002.
func TestCostOfSaleByChannel(t *testing.T) {
	network := Default(equipment.Network)
	sale := Sale{Segments: 4, Fare: 40000, Paid: 47500, Region: refdata.NorthAmerica, Home: true}
	sale.Channel = market.Direct
	if c := network.CostOf(sale); c.System != 0 || c.Commission != 0 || c.Own != 150 || c.Payment != 950 || c.Total() != 1100 {
		t.Errorf("direct: %+v", c)
	}
	sale.Channel = market.Agency
	if c := network.CostOf(sale); c.System != 4*230 || c.Commission != 0 || c.Own != 25 {
		t.Errorf("US agency under a full-content deal: %+v", c)
	}
	mid := Policy{Sells: map[string]bool{market.Agency: true}}
	if c := mid.CostOf(sale); c.System != 4*340 || c.Total() < 2000 || c.Total() > 2600 {
		t.Errorf("US agency without one: %+v", c)
	}
	// A travel management company is paid two per cent of the fare on top.
	sale.Channel = market.TMC
	if c := network.CostOf(sale); c.Commission != 800 {
		t.Errorf("the override to a travel management company: %+v", c)
	}
	// Outside North America the fee is higher, lower at home than abroad,
	// and an agency is still paid a commission in much of the world.
	sale.Channel, sale.Region = market.Agency, refdata.Asia
	home := network.CostOf(sale)
	sale.Home = false
	abroad := network.CostOf(sale)
	if home.System >= abroad.System || home.System <= 4*230 || home.Commission != 1200 {
		t.Errorf("Asia: at home %+v, abroad %+v", home, abroad)
	}
	sale.Region = refdata.Europe
	if c := network.CostOf(sale); c.Commission != 0 {
		t.Errorf("Europe pays no base commission: %+v", c)
	}
}

func TestPoliciesByKindOfCarrier(t *testing.T) {
	ulcc, lcc, net := Default(equipment.UltraLowCost), Default(equipment.LowCost), Default(equipment.Network)
	if !ulcc.Offered(market.Direct) || ulcc.Offered(market.OTA) || ulcc.Offered(market.TMC) {
		t.Error("an ultra-low-cost carrier sells on its own site alone")
	}
	if !lcc.Offered(market.OTA) || lcc.Offered(market.Agency) {
		t.Error("a low-cost carrier sells through online agencies and not through travel agents")
	}
	if !net.Offered(market.Agency) || !net.Offered(market.TMC) || !net.FullContent {
		t.Error("a network carrier sells everywhere")
	}
	if f := net.Favour(market.TMC, 40000); f != 960 {
		t.Errorf("two points of override on a 400-dollar fare are worth %d cents of display", f)
	}
	if f := lcc.Favour(market.OTA, 40000); f != 0 {
		t.Errorf("no override, no favour: %d", f)
	}
}

// The agencies of a world: each has a number that checks, the same
// systems give the same agencies, every kind is sold in every region,
// and a draw follows the weights.
func TestAgencies(t *testing.T) {
	as := Agencies([]string{"1S", "1G", "1A"})
	again := Agencies([]string{"1A", "1G", "1S"})
	if len(as) != 16 || len(again) != len(as) {
		t.Fatalf("%d agencies", len(as))
	}
	seen := map[string]bool{}
	systems := map[string]int{}
	for i, a := range as {
		if a != again[i] {
			t.Errorf("the same systems gave %+v and %+v", a, again[i])
		}
		if !Valid(a.Code) || seen[a.Code] {
			t.Errorf("%s has the number %s", a.Name, a.Code)
		}
		seen[a.Code] = true
		systems[a.System]++
		if a.Kind == market.TMC && a.Fee != feeTMC || a.Kind == market.Agency && a.Fee != feeAgent {
			t.Errorf("%s charges %d", a.Name, a.Fee)
		}
		if a.Connect != (a.ConnectShare > 0) {
			t.Errorf("%s: connect %v, share %.2f", a.Name, a.Connect, a.ConnectShare)
		}
	}
	if len(systems) != 3 {
		t.Errorf("the agencies book through %v", systems)
	}
	// 1234567 is 7 x 176366 and 5 over.
	if !Valid("12345675") || Valid("12345670") || Valid("1234") || Valid("ABCDEFG1") {
		t.Error("an agency number is seven digits and their remainder over seven")
	}
	rng := rand.New(rand.NewSource(1))
	got := map[string]int{}
	for i := 0; i < 20000; i++ {
		a, ok := Pick(rng, as, market.OTA, refdata.Europe)
		if !ok || a.Kind != market.OTA || (a.Region != "" && a.Region != refdata.Europe) {
			t.Fatalf("an online agency for Europe: %+v", a)
		}
		got[a.Name]++
	}
	// 0.40, 0.30, 0.15 and Europe's 0.15 of one: Meridian has two in five.
	if share := float64(got["Meridian Travel"]) / 20000; share < 0.38 || share > 0.42 || got["Vela Reisen"] == 0 || got["Skylantern"] != 0 {
		t.Errorf("draws for Europe: %v", got)
	}
	for _, kind := range []string{market.OTA, market.TMC, market.Agency} {
		for _, r := range []string{refdata.NorthAmerica, refdata.Africa, "", "Atlantis"} {
			if a, ok := Pick(rng, as, kind, r); !ok || a.Kind != kind {
				t.Errorf("no %s for %q", kind, r)
			}
		}
	}
	if _, ok := Pick(rng, nil, market.OTA, ""); ok {
		t.Error("a world with no agencies gave one")
	}
}

// What a direct connection changes: the distribution system is not paid,
// the vendor is paid about a tenth, and the agency gets two dollars a
// ticket. Through a distribution system, half the fee goes on to the
// agency.
func TestDirectConnectionAndIncentive(t *testing.T) {
	network := Default(equipment.Network)
	sale := Sale{Channel: market.OTA, Segments: 2, Fare: 30000, Paid: 34000, Region: refdata.NorthAmerica, Home: true}
	through := network.CostOf(sale)
	if through.System != 460 || through.Incentive != 230 || through.Commission != 300 {
		t.Errorf("through a distribution system: %+v", through)
	}
	sale.Channel, sale.Kind = NDC, market.OTA
	direct := network.CostOf(sale)
	if direct.System != 460*15/100 || direct.Incentive != 0 || direct.Commission != 200+300 || direct.Total() >= through.Total() {
		t.Errorf("over a direct connection: %+v against %+v", direct, through)
	}
	if lcc := Default(equipment.LowCost); !lcc.DirectFirst || !lcc.Connect || lcc.FullContent || network.DirectFirst || !network.Connect || Default(equipment.UltraLowCost).Connect {
		t.Error("who offers a direct connection and who keeps seats for its own site")
	}
}
