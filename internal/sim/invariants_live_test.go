package sim

import (
	"testing"

	"github.com/adamf/jetway/pkg/inventory"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/wholesky/internal/host"
)

// The live check reads the seats the carriers hold and names a cabin that
// has confirmed more than it has, which Seed can produce because it trusts
// the book of record: a corrupted store is exactly what the gate is for.
func TestLiveInvariantsNameAnOversoldCabin(t *testing.T) {
	inv := inventory.New("ZZ", func(carrier, flight, date, board string) (map[string]int, bool) {
		return map[string]int{"Y": 2}, true
	})
	seg := pnr.Segment{Type: pnr.SegmentAir, Carrier: "ZZ", FlightNum: "0001", WireDate: "26NOV", Board: "AAA", Off: "BBB", Class: "Y", Seats: 1, Status: "HK"}
	inv.Seed(seg)
	inv.Seed(seg)
	tenants := map[string]*host.Tenant{"ZZ": {Inventory: inv}}
	v := checkInventories(tenants)
	if !v.OK() || v.Cabins != 1 || v.Sold != 2 {
		t.Fatalf("two of two is full, not oversold: %+v", v)
	}
	inv.Seed(seg)
	v = checkInventories(tenants)
	if v.OK() || len(v.Oversold) != 1 || v.Oversold[0].Sold != 3 || v.Oversold[0].Seats != 2 || v.Oversold[0].Carrier != "ZZ" {
		t.Fatalf("three of two must be named: %+v", v)
	}
	var all Invariants
	all.Merge(v)
	all.Merge(checkInventories(nil))
	if all.Shards != 2 || len(all.Oversold) != 1 {
		t.Fatalf("merge keeps the count and the violation: %+v", all)
	}
}
