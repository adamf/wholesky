package revenue

import (
	"testing"

	"github.com/adamf/jetway/pkg/pnr"
)

func TestLedgerCreditsLegsWithTheirShare(t *testing.T) {
	l := New()
	r := &pnr.PNR{
		Segments: []pnr.Segment{
			{Type: pnr.SegmentAir, Carrier: "DL", OperatingCarrier: "OO", FlightNum: "3991", Board: "SFO", Off: "SEA"},
			{Type: pnr.SegmentAir, Carrier: "DL", FlightNum: "0100", Board: "SEA", Off: "ATL"},
		},
		Pricing: &pnr.Pricing{Currency: "USD", Total: 60000, Passengers: []pnr.PassengerPricing{
			{Ref: 1, Segments: []int64{20000, 30000}, Taxes: 4000, Total: 54000},
			{Ref: 2, Segments: []int64{2000, 3000}, Taxes: 1000, Total: 6000},
		}},
	}
	l.Record(r)
	// The marketed leg is credited to the operator that flies it.
	if got := l.Sum([]string{Key("OO", "3991", "SFO")}); got != 20000+2000+2000+500 {
		t.Errorf("OO3991 share: %d", got)
	}
	if got := l.Sum([]string{Key("DL", "100", "SEA")}); got != 30000+3000+2000+500 {
		t.Errorf("DL100 share: %d", got)
	}
	if l.Total() != 60000 || l.Sum([]string{Key("DL", "3991", "SFO")}) != 0 {
		t.Errorf("total %d, marketed key should hold nothing", l.Total())
	}
	l.Record(&pnr.PNR{Segments: r.Segments}) // unpriced adds nothing
	if l.Total() != 60000 {
		t.Errorf("an unpriced record changed the total")
	}
}
