package host

import (
	"testing"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/world"
)

// The cabins a carrier's aircraft have. A network carrier's narrow-bodies
// have a front cabin of two-and-two rows: 12 seats on an A320, 16 on a
// 737-800 and 20 on an A321. A low-cost carrier's have one cabin. A
// wide-body has a business cabin whoever flies it, and the inventory a
// tenant sells from has the same cabins the filler fills.
func TestCabinsByKindOfCarrier(t *testing.T) {
	var network, lowCost string
	for _, c := range []string{"AA", "UA", "DL", "LH", "BA", "AF"} {
		if equipment.ModelOf(c) == equipment.Network {
			network = c
			break
		}
	}
	for _, c := range []string{"WN", "U2", "FR", "B6"} {
		if equipment.ModelOf(c) != equipment.Network {
			lowCost = c
			break
		}
	}
	if network == "" || lowCost == "" {
		t.Fatalf("no carrier of each kind: %q, %q", network, lowCost)
	}
	for _, c := range []struct {
		code       string
		seats      int
		front, all int // a network carrier's front cabin and whole aircraft
	}{{"320", 180, 12, 174}, {"73H", 174, 16, 166}, {"321", 220, 20, 210}} {
		n := Cabins(world.Flight{Carrier: network, Equipment: c.code, Seats: c.seats}, 0)
		if n["C"] != c.front || n["C"]+n["Y"] != c.all || len(n) != 2 {
			t.Errorf("%s's %s: %v, want %d at the front of %d", network, c.code, n, c.front, c.all)
		}
		l := Cabins(world.Flight{Carrier: lowCost, Equipment: c.code, Seats: c.seats}, 0)
		if len(l) != 1 || l["Y"] != c.seats {
			t.Errorf("%s's %s: %v, want one cabin of %d", lowCost, c.code, l, c.seats)
		}
	}
	wide := Cabins(world.Flight{Carrier: lowCost, Equipment: "77W", Seats: 360}, 0)
	if wide["C"] == 0 || wide["C"] != Cabins(world.Flight{Carrier: network, Equipment: "77W", Seats: 360}, 0)["C"] {
		t.Errorf("a 777 has a business cabin whoever flies it: %v", wide)
	}
	// The inventory a tenant sells from, and a leg it markets and another
	// carrier flies: the operating carrier's cabins.
	own := world.Flight{Carrier: network, Number: "0100", From: "ORD", To: "DCA", Equipment: "320", Seats: 180}
	sold := world.Flight{Carrier: lowCost, Number: "0200", From: "DCA", To: "BOS", Equipment: "320", Seats: 180, Marketing: network, MarketingNumber: "5200"}
	capacity := capacityFor(network, []world.Flight{own, sold}, 0)
	if c, ok := capacity(network, "100", "26NOV", "ORD"); !ok || c["C"] != 12 || c["Y"] != 162 {
		t.Errorf("the tenant's own A320: %v", c)
	}
	if c, ok := capacity(network, "5200", "26NOV", "DCA"); !ok || c["C"] != 0 || c["Y"] != 180 {
		t.Errorf("the leg it markets on a low-cost carrier's A320: %v", c)
	}
}
