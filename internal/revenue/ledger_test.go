package revenue

import (
	"testing"

	"github.com/adamf/jetway/pkg/pnr"
)

// A record's fare goes to the legs that fly it and its taxes to nobody:
// they are the government's. A coupon one carrier flies on another's
// ticket is worth its share less the charge the two agreed, which the
// ticketing carrier keeps. A cancellation takes the money back out and
// leaves the booking fee paid.
func TestLedgerCreditsLegsWithTheirShare(t *testing.T) {
	l := New()
	l.Charge = func(ticketed, flown string) int { return 700 }
	l.Sale = func(r *pnr.PNR, ticketing string, segments int, fare, paid int64) Cost {
		return Cost{System: 230 * int64(segments), Payment: paid * 2 / 100, Own: 25, Commission: 600,
			Agent: "12345675", Incentive: 115 * int64(segments), Fee: 1500}
	}
	r := &pnr.PNR{
		RecordLocator: "ABC123",
		Origin:        pnr.Origin{Channel: "tmc"},
		Segments: []pnr.Segment{
			{Type: pnr.SegmentAir, Carrier: "DL", OperatingCarrier: "OO", FlightNum: "3991", Board: "SFO", Off: "SEA"},
			{Type: pnr.SegmentAir, Carrier: "DL", FlightNum: "0100", Board: "SEA", Off: "ATL"},
		},
		Pricing: &pnr.Pricing{Currency: "USD", Base: 55000, Taxes: 5000, CarrierFees: 1000, Total: 60000, Passengers: []pnr.PassengerPricing{
			{Ref: 1, Segments: []int64{20000, 30000}, Taxes: 4000, Total: 54000},
			{Ref: 2, Segments: []int64{2000, 3000}, Taxes: 1000, Total: 6000},
		}},
	}
	l.Record(r)
	// The marketed leg is credited to the operator that flies it, less
	// Delta's seven per cent.
	if got := l.Sum([]string{Key("OO", "3991", "SFO")}); got != 22000-1540 {
		t.Errorf("OO3991 share: %d", got)
	}
	// Delta's own leg: its fare and its half of the surcharge.
	if got := l.Sum([]string{Key("DL", "100", "SEA")}); got != 33000+500 {
		t.Errorf("DL100 share: %d", got)
	}
	if l.Sum([]string{Key("DL", "3991", "SFO")}) != 0 {
		t.Error("the marketed key should hold nothing")
	}
	dl, oo := l.AccountOf("DL"), l.AccountOf("OO")
	if dl.Flown != 33000 || dl.Kept != 1540+500 || dl.Fees != 1000 || dl.Taxes != 4000 || dl.Tickets != 2 || dl.Channels["tmc"] != 2 {
		t.Errorf("Delta's account: %+v", dl)
	}
	if dl.System != 230*4 || dl.Payment != 1200 || dl.Distribution() != 920+1200+25+600 {
		t.Errorf("what the sale cost Delta: %+v", dl)
	}
	// The agency that sold it: two tickets, the fare, half the booking
	// fee from its distribution system, Delta's commission, and its own
	// fee from each traveller.
	ag := l.Agents()["12345675"]
	if ag.Tickets != 2 || ag.Sales != 56000 || ag.Incentive != 460 || ag.Commission != 600 || ag.Fees != 3000 || ag.Income() != 4060 || ag.Carriers["DL"] != 2 {
		t.Errorf("the agency's account: %+v", ag)
	}
	if oo.Flown != 22000-1540 || oo.Paid != 1540 || oo.Tickets != 0 {
		t.Errorf("the operator's account: %+v", oo)
	}
	l.Record(r) // the same record again adds nothing
	l.Record(&pnr.PNR{Segments: r.Segments})
	if l.AccountOf("DL").Flown != 33000 {
		t.Error("a record counted twice, or an unpriced record counted")
	}
	// A return on another date is not today's revenue.
	l2 := New()
	l2.Wire = "26NOV"
	rt := *r
	rt.RecordLocator = "RT0001"
	rt.Segments = []pnr.Segment{
		{Type: pnr.SegmentAir, Carrier: "DL", FlightNum: "0100", Board: "SEA", Off: "ATL", WireDate: "26NOV"},
		{Type: pnr.SegmentAir, Carrier: "DL", FlightNum: "0101", Board: "ATL", Off: "SEA", WireDate: "03DEC"},
	}
	l2.Record(&rt)
	if l2.Sum([]string{Key("DL", "100", "SEA")}) != 22000+500 || l2.Sum([]string{Key("DL", "101", "ATL")}) != 0 {
		t.Errorf("flown today %d, the return %d", l2.Sum([]string{Key("DL", "100", "SEA")}), l2.Sum([]string{Key("DL", "101", "ATL")}))
	}
	// Cancelled: the fare goes back, the booking fee does not.
	if !l.Reverse("ABC123") || l.Reverse("ABC123") {
		t.Fatal("a record reverses once")
	}
	dl = l.AccountOf("DL")
	if l.Total() != 0 || dl.Flown != 0 || dl.Kept != 0 || dl.Tickets != 0 || dl.Payment != 0 || dl.System != 920 || dl.Own != 25 || l.Seats([]string{Key("DL", "100", "SEA")}) != 0 {
		t.Errorf("after the cancellation: total %d, Delta %+v", l.Total(), dl)
	}
	// The agency gives back the commission and its fee with the fare, and
	// keeps what the distribution system paid it for the booking.
	if ag := l.Agents()["12345675"]; ag.Tickets != 0 || ag.Sales != 0 || ag.Commission != 0 || ag.Fees != 0 || ag.Incentive != 460 {
		t.Errorf("the agency after the cancellation: %+v", ag)
	}
}
