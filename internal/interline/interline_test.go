package interline

import (
	"context"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/world"
)

// A ticket sold by AS with two coupons: AS's own metal SEA-PDX and a
// codeshare SEA-GEG that OO flies as OO3000. The fare prorates by mileage,
// OO bills AS the codeshare coupon's share less nine per cent, and the
// same record held in two books is billed once.
func TestCodeshareCouponIsProratedAndBilledOnce(t *testing.T) {
	ctx := context.Background()
	m := &world.Manifest{
		Carriers: []world.Carrier{{Designator: "AS"}, {Designator: "OO"}},
		Flights: []world.Flight{
			{Carrier: "AS", Number: "0500", From: "SEA", To: "PDX", KM: 200},
			{Carrier: "OO", Number: "3000", From: "SEA", To: "GEG", KM: 400, Marketing: "AS", MarketingNumber: "3000"},
		},
	}
	dep := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	rec := &pnr.PNR{RecordLocator: "CODESH", Origin: pnr.Origin{Party: "1G"},
		Passengers: []pnr.Passenger{{Ref: 1, Surname: "SMITH", Given: "JOHN"}},
		Segments: []pnr.Segment{
			{Ref: 1, Type: pnr.SegmentAir, Carrier: "AS", FlightNum: "0500", Board: "SEA", Off: "PDX", Depart: dep, WireDate: "26NOV", Status: "HK", Seats: 1},
			{Ref: 2, Type: pnr.SegmentAir, Carrier: "AS", FlightNum: "3000", Board: "SEA", Off: "GEG", Depart: dep, WireDate: "26NOV", Status: "HK", Seats: 1},
		},
		Tickets: []pnr.Ticket{{Number: pnr.TicketNumber{AirlineCode: "027", Serial: "2400000001"}, PaxRef: 1, IssuedAt: dep.AddDate(0, 0, -10),
			Coupons: []pnr.Coupon{{Number: 1, SegmentRef: 1, Status: pnr.CouponOpen}, {Number: 2, SegmentRef: 2, Status: pnr.CouponOpen}}}},
		Pricing: &pnr.Pricing{Currency: "USD", Base: 30000, Taxes: 3000, Total: 33000},
	}
	agent, carrier := store.NewMem(), store.NewMem()
	agent.CreatePNR(ctx, rec, nil)
	copy := *rec
	copy.RecordLocator = "ASCOPY"
	carrier.CreatePNR(ctx, &copy, nil)

	acct := func(d string) string { return map[string]string{"AS": "027", "OO": "412"}[d] }
	p := &Plan{ServiceCharge: 900, Accounting: acct}
	sum, err := p.Run(ctx, dep, m, []Book{{Name: "1G", Store: agent}, {Name: "AS", Store: carrier}})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Invoices != 1 || sum.Coupons != 1 {
		t.Fatalf("one invoice for one codeshare coupon: %+v", sum)
	}
	inv := sum.ByPair[PairKey("OO", "AS")]
	if inv == nil || len(inv.Lines) != 1 {
		t.Fatalf("OO bills AS: %+v", sum.ByPair)
	}
	l := inv.Lines[0]
	// 300.00 over 200 and 400 km: the codeshare coupon is 200.00; 9% ISC is 18.00.
	if l.Prorate != 20000 || l.ServiceCharge != 1800 || l.Net != 18200 || l.Flight != "OO3000" || l.Sold != "AS3000" || l.Sector != "SEA-GEG" || l.Coupon != 2 {
		t.Errorf("line: %+v", l)
	}
	if inv.Net != 18200 || sum.Net != 18200 || sum.Prorate != 20000 {
		t.Errorf("totals: %+v", sum)
	}
	// Merging two machines' views adds and remembers the holder.
	merged := Merge(dep, map[string]View{"http://a": sum.AsView(), "http://b": sum.AsView()})
	if merged.Coupons != 2 || merged.Net != 36400 || merged.ByPair[PairKey("OO", "AS")].Peer == "" {
		t.Errorf("merged: %+v", merged)
	}
}
