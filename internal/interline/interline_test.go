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

// Who gets what on a ticket three carriers fly. United tickets a journey
// with a through fare over its own flight and Lufthansa's, and a separate
// fare on a carrier it has nothing but an interline agreement with. Each
// coupon is worth its part of its own fare component. Lufthansa, in a
// joint venture with United, bills its coupon whole; the stranger bills
// its coupon less nine per cent; a codeshare partner would bill less
// seven.
func TestEachCouponIsBilledByItsComponentAndItsTie(t *testing.T) {
	ctx := context.Background()
	m := &world.Manifest{
		Carriers: []world.Carrier{{Designator: "UA"}, {Designator: "LH"}, {Designator: "A3"}, {Designator: "OO"}},
		Flights: []world.Flight{
			{Carrier: "UA", Number: "0900", From: "ORD", To: "FRA", KM: 7000},
			{Carrier: "LH", Number: "1280", From: "FRA", To: "ATH", KM: 1800},
			{Carrier: "A3", Number: "0350", From: "ATH", To: "JTR", KM: 220},
			{Carrier: "OO", Number: "5500", From: "JTR", To: "HER", KM: 150, Marketing: "UA", MarketingNumber: "5500"},
		},
	}
	dep := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	seg := func(ref int, carrier, num, from, to string) pnr.Segment {
		return pnr.Segment{Ref: ref, Type: pnr.SegmentAir, Carrier: carrier, FlightNum: num, Board: from, Off: to, Depart: dep, WireDate: "26NOV", Status: "HK", Seats: 1}
	}
	rec := &pnr.PNR{RecordLocator: "THREEC", Origin: pnr.Origin{Party: "1G"},
		Passengers: []pnr.Passenger{{Ref: 1, Surname: "JONES", Given: "ANN"}},
		Segments:   []pnr.Segment{seg(1, "UA", "0900", "ORD", "FRA"), seg(2, "LH", "1280", "FRA", "ATH"), seg(3, "A3", "0350", "ATH", "JTR"), seg(4, "UA", "5500", "JTR", "HER")},
		Tickets: []pnr.Ticket{{Number: pnr.TicketNumber{AirlineCode: "016", Serial: "2400000009"}, PaxRef: 1, IssuedAt: dep.AddDate(0, 0, -10),
			Coupons: []pnr.Coupon{{Number: 1, SegmentRef: 1}, {Number: 2, SegmentRef: 2}, {Number: 3, SegmentRef: 3}, {Number: 4, SegmentRef: 4}}}},
		// A through fare of 880.00 ORD-ATH divided by miles, 700.00 and
		// 180.00; 90.00 for the island hop; 50.00 for the codeshare.
		Pricing: &pnr.Pricing{Currency: "USD", Base: 102000, Taxes: 9000, Total: 111000,
			Passengers: []pnr.PassengerPricing{{Ref: 1, Segments: []int64{70000, 18000, 9000, 5000}, Base: 102000, Total: 111000}}},
	}
	book := store.NewMem()
	book.CreatePNR(ctx, rec, nil)
	acct := func(d string) string { return map[string]string{"UA": "016", "LH": "220", "A3": "390", "OO": "412"}[d] }
	ties := map[string]struct {
		rate int
		tie  string
	}{"LH": {0, "joint venture"}, "A3": {900, "interline"}, "OO": {700, "codeshare"}}
	p := &Plan{ServiceCharge: 900, Accounting: acct, Charge: func(ticketed, flown string) (int, string) {
		if ticketed != "UA" {
			t.Errorf("the ticketing carrier is %s", ticketed)
		}
		return ties[flown].rate, ties[flown].tie
	}}
	sum, err := p.Run(ctx, dep, m, []Book{{Name: "1G", Store: book}})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Invoices != 3 || sum.Coupons != 3 {
		t.Fatalf("three carriers bill United one coupon each: %+v", sum)
	}
	for flown, want := range map[string]Line{
		"LH": {Coupon: 2, Flight: "LH1280", Sold: "LH1280", Tie: "joint venture", Prorate: 18000, ServiceCharge: 0, Net: 18000},
		"A3": {Coupon: 3, Flight: "A30350", Sold: "A30350", Tie: "interline", Prorate: 9000, ServiceCharge: 810, Net: 8190},
		"OO": {Coupon: 4, Flight: "OO5500", Sold: "UA5500", Tie: "codeshare", Prorate: 5000, ServiceCharge: 350, Net: 4650},
	} {
		inv := sum.ByPair[PairKey(flown, "UA")]
		if inv == nil || len(inv.Lines) != 1 {
			t.Fatalf("%s bills United: %+v", flown, sum.ByPair)
		}
		l := inv.Lines[0]
		l.Document, l.Sector = "", ""
		if l != want {
			t.Errorf("%s: %+v, want %+v", flown, l, want)
		}
	}
	// A record priced without the detail is prorated whole by miles: the
	// same ticket then gives Lufthansa 1800 of 9170 kilometres.
	rec2 := *rec
	rec2.RecordLocator = "NODETL"
	rec2.Tickets = []pnr.Ticket{rec.Tickets[0]}
	rec2.Tickets[0].Number.Serial = "2400000010"
	rec2.Pricing = &pnr.Pricing{Currency: "USD", Base: 102000, Taxes: 9000, Total: 111000}
	plain := store.NewMem()
	plain.CreatePNR(ctx, &rec2, nil)
	sum2, err := p.Run(ctx, dep, m, []Book{{Name: "1G", Store: plain}})
	if err != nil {
		t.Fatal(err)
	}
	if l := sum2.ByPair[PairKey("LH", "UA")].Lines[0]; l.Prorate != 102000*1800/9170 {
		t.Errorf("by miles Lufthansa's coupon is %d, want %d", l.Prorate, 102000*1800/9170)
	}
}
