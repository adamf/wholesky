package distribution

import (
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
