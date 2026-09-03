package settle

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/bsp"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
)

func ticketedRecord(loc, agent, acct, serial string, price int64) *pnr.PNR {
	dep := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	return &pnr.PNR{
		RecordLocator: loc, Status: pnr.StatusTicketed, Origin: pnr.Origin{Party: agent},
		Passengers: []pnr.Passenger{{Ref: 1, Surname: "SMITH", Given: "JOHN", Title: "MR"}},
		Segments: []pnr.Segment{{Ref: 1, Type: pnr.SegmentAir, Carrier: "BA", FlightNum: "0117", Class: "Y", Depart: dep, DepartTime: "0900",
			WireDate: "26NOV", Board: "LHR", Off: "JFK", Status: "HK", Seats: 1, FareBasis: "YOW"}},
		Tickets: []pnr.Ticket{{Number: pnr.TicketNumber{AirlineCode: acct, Serial: serial}, PaxRef: 1, IssuedAt: dep.AddDate(0, 0, -20), IssuedBy: agent,
			Coupons: []pnr.Coupon{{Number: 1, SegmentRef: 1, Status: pnr.CouponOpen}}}},
		Pricing: &pnr.Pricing{Currency: "USD", Base: price, Taxes: price / 10, Total: price + price/10},
	}
}

// The plan reads the agents' books where it has them, and the carriers'
// books where it does not: a record the carrier holds that was sold
// through an agent whose book is elsewhere is reported from the carrier's
// copy and counted unverified; one both sides hold is matched; one the
// agent holds and the carrier does not is unreported.
func TestPlanGathersFromBothSidesAndReconciles(t *testing.T) {
	ctx := context.Background()
	agent := store.NewMem()
	carrier := store.NewMem()
	both := ticketedRecord("BOTH01", "1G", "125", "2400000001", 50000)
	agent.CreatePNR(ctx, both, nil)
	carrierCopy := *both
	carrierCopy.RecordLocator = "BACOPY"
	carrierCopy.Locators = []pnr.ExternalLocator{{Owner: "1G", Value: "BOTH01"}}
	carrier.CreatePNR(ctx, &carrierCopy, nil)
	agent.CreatePNR(ctx, ticketedRecord("AGENT1", "1G", "125", "2400000002", 30000), nil)   // agent only
	carrier.CreatePNR(ctx, ticketedRecord("FILL01", "1S", "125", "2400000003", 20000), nil) // sold through 1S, whose book is elsewhere
	carrier.CreatePNR(ctx, ticketedRecord("DIRECT", "BA", "125", "2400000004", 10000), nil) // a direct sale: no agent, not the plan's
	carrier.CreatePNR(ctx, ticketedRecord("OTHER1", "1S", "999", "2400000005", 10000), nil) // another airline's document

	p := &Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	sum, err := p.Run(ctx, day, []Agent{{Designator: "1G", Store: agent}, {Designator: "1S"}}, []Airline{{Designator: "BA", Accounting: "125", Store: carrier}})
	if err != nil {
		t.Fatal(err)
	}
	st, ok := sum.Statements["BA"]
	if !ok || st.Transactions != 3 {
		t.Fatalf("BA: %+v", sum)
	}
	if st.Matched != 1 || st.Unreported != 1 || st.Unverified != 1 || st.Unknown != 1 {
		t.Errorf("reconciliation matched %d unreported %d unverified %d unknown %d", st.Matched, st.Unreported, st.Unverified, st.Unknown)
	}
	if st.Gross != 55000+33000+22000 {
		t.Errorf("gross %d", st.Gross)
	}
	if len(st.File.Offices) != 2 {
		t.Errorf("one office per agent: %d", len(st.File.Offices))
	}
	// The file parses back to what the plan said.
	f := st.File
	var gross int64
	n := 0
	for _, o := range f.Offices {
		n += len(o.Transactions)
		gross += o.OfficeTotals().Gross
	}
	if n != 3 || gross != st.Gross {
		t.Errorf("file %d transactions gross %d", n, gross)
	}
	if _, err := bsp.CheckDigit("1252400000001"); err != nil {
		t.Fatal(err)
	}
	// Merging two machines' views adds the sums and remembers the holder.
	v1 := sum.AsView()
	other := &Summary{Day: day, Statements: map[string]Statement{"BA": {Airline: "BA", Transactions: 2, Gross: 100}, "AA": {Airline: "AA", Transactions: 1, Gross: 50}}}
	merged := Merge(day, map[string]View{"http://a": v1, "http://b": other.AsView()})
	if merged.Airlines != 2 || merged.Transactions != 6 || merged.Gross != st.Gross+150 || merged.Statements["AA"].Peer != "http://b" {
		t.Errorf("merged %+v", merged)
	}
}

// A refunded document is reported twice: the sale, and a refund with every
// amount reversed and dated when the money went back, so the plan's totals
// net to nothing and the commission the agent kept comes back.
func TestRefundedDocumentIsReportedAsARefund(t *testing.T) {
	ctx := context.Background()
	agent := store.NewMem()
	rec := ticketedRecord("REFUND", "1G", "125", "2400000009", 50000)
	at := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	rec.Tickets[0].RefundedAt = &at
	rec.Tickets[0].Coupons[0].Status = pnr.CouponRefunded
	agent.CreatePNR(ctx, rec, nil)
	p := &Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	sum, err := p.Run(ctx, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), []Agent{{Designator: "1G", Store: agent}}, []Airline{{Designator: "BA", Accounting: "125"}})
	if err != nil {
		t.Fatal(err)
	}
	st := sum.Statements["BA"]
	if st.Transactions != 2 || st.Refunds != 1 || sum.Refunds != 1 {
		t.Fatalf("sale and refund: %+v", st)
	}
	if st.Gross != 0 || st.Remittance != 0 || st.Commission != 0 {
		t.Errorf("a same-day refund nets to nothing: gross %d remittance %d commission %d", st.Gross, st.Remittance, st.Commission)
	}
	var sale, refund *bsp.Transaction
	for i := range st.File.Offices[0].Transactions {
		tx := &st.File.Offices[0].Transactions[i]
		if tx.Code == bsp.TransRefund {
			refund = tx
		} else {
			sale = tx
		}
	}
	if sale == nil || refund == nil {
		t.Fatal("both transactions present")
	}
	if refund.Fare != -sale.Fare || refund.Total != -sale.Total || refund.Taxes[0].Amount != -sale.Taxes[0].Amount || !refund.Issued.Equal(at) || refund.Document != sale.Document {
		t.Errorf("refund reverses the sale:\n sale %+v\n refund %+v", sale, refund)
	}
	if refund.CommissionAmount != -sale.CommissionAmount || refund.Payments[0].Remittance != -sale.Payments[0].Remittance {
		t.Errorf("commission recalled and remittance reversed: sale %d/%d refund %d/%d", sale.CommissionAmount, sale.Payments[0].Remittance, refund.CommissionAmount, refund.Payments[0].Remittance)
	}
	// A refund dated after the day is not in this day's file.
	late := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	rec.Tickets[0].RefundedAt = &late
	agent.UpdatePNR(ctx, rec, rec.Version, nil)
	sum, _ = p.Run(ctx, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), []Agent{{Designator: "1G", Store: agent}}, []Airline{{Designator: "BA", Accounting: "125"}})
	if sum.Statements["BA"].Transactions != 1 {
		t.Errorf("a later refund reported early: %+v", sum.Statements["BA"])
	}
}

// A reissued document is reported with the one it replaced behind it and
// the old document's value as its form of payment: an even exchange
// collects nothing new in cash and earns no new commission.
func TestExchangedDocumentCarriesItsOriginalIssue(t *testing.T) {
	ctx := context.Background()
	agent := store.NewMem()
	rec := ticketedRecord("EXCHNG", "1G", "125", "2400000011", 50000)
	old := rec.Tickets[0]
	old.Coupons[0].Status = pnr.CouponExchanged
	from := old.Number
	newT := pnr.Ticket{Number: pnr.TicketNumber{AirlineCode: "125", Serial: "2400000012"}, PaxRef: 1, IssuedAt: old.IssuedAt.AddDate(0, 0, 2), IssuedBy: "1G",
		Coupons: []pnr.Coupon{{Number: 1, SegmentRef: 1, Status: pnr.CouponOpen}}, ExchangedFrom: &from}
	rec.Tickets = []pnr.Ticket{old, newT}
	agent.CreatePNR(ctx, rec, nil)
	p := &Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	sum, err := p.Run(ctx, time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC), []Agent{{Designator: "1G", Store: agent}}, []Airline{{Designator: "BA", Accounting: "125"}})
	if err != nil {
		t.Fatal(err)
	}
	st := sum.Statements["BA"]
	if st.Transactions != 2 {
		t.Fatalf("the old sale and the reissue: %+v", st)
	}
	var reissue *bsp.Transaction
	for i := range st.File.Offices[0].Transactions {
		if tx := &st.File.Offices[0].Transactions[i]; tx.Document == "1252400000012" {
			reissue = tx
		}
	}
	if reissue == nil || reissue.OriginalDocument != "1252400000011" || !reissue.OriginalIssued.Equal(old.IssuedAt) || reissue.OriginalAgent == "" {
		t.Fatalf("reissue: %+v", reissue)
	}
	if len(reissue.Payments) != 2 || reissue.Payments[0].Type != bsp.PaymentExchange || reissue.Payments[0].Amount != reissue.Total || reissue.Payments[1].Type != bsp.PaymentCash || reissue.Payments[1].Amount != 0 || reissue.Payments[1].Remittance != 0 || reissue.CommissionAmount != 0 {
		t.Errorf("an even exchange remits nothing new: %+v", reissue.Payments)
	}
}

// Where the carrier's copy of a record prices the passenger above what the
// agent reported, the carrier debits the agent the difference; below, it
// credits. Each memo names the document it corrects, and the file carries
// it in the handbook's layout and reads it back.
func TestPlanRaisesMemosWhereTheCarrierPricesDifferently(t *testing.T) {
	ctx := context.Background()
	agent := store.NewMem()
	carrier := store.NewMem()
	copyOf := func(r *pnr.PNR, loc string, price int64) *pnr.PNR {
		c := *r
		c.RecordLocator = loc
		c.Locators = []pnr.ExternalLocator{{Owner: "1G", Value: r.RecordLocator}}
		c.Pricing = &pnr.Pricing{Currency: "USD", Base: price, Taxes: price / 10, Total: price + price/10}
		return &c
	}
	under := ticketedRecord("UNDER1", "1G", "125", "2400000011", 50000) // agent reported 55000; carrier says 66000
	over := ticketedRecord("OVER01", "1G", "125", "2400000012", 50000)  // carrier says 44000
	same := ticketedRecord("SAME01", "1G", "125", "2400000013", 50000)  // agree
	for _, r := range []*pnr.PNR{under, over, same} {
		agent.CreatePNR(ctx, r, nil)
	}
	carrier.CreatePNR(ctx, copyOf(under, "BAUND1", 60000), nil)
	carrier.CreatePNR(ctx, copyOf(over, "BAOVR1", 40000), nil)
	carrier.CreatePNR(ctx, copyOf(same, "BASAM1", 50000), nil)

	p := &Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	day := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	sum, err := p.Run(ctx, day, []Agent{{Designator: "1G", Store: agent}}, []Airline{{Designator: "BA", Accounting: "125", Store: carrier}})
	if err != nil {
		t.Fatal(err)
	}
	st := sum.Statements["BA"]
	if st.Memos != 2 || st.MemoAmount != 11000-11000 || st.Transactions != 5 {
		t.Fatalf("memos %d net %d transactions %d", st.Memos, st.MemoAmount, st.Transactions)
	}
	if sum.Memos != 2 {
		t.Errorf("summary memos %d", sum.Memos)
	}
	var adm, acm *bsp.Transaction
	for _, o := range st.File.Offices {
		for i := range o.Transactions {
			switch o.Transactions[i].Code {
			case bsp.TransADM:
				adm = &o.Transactions[i]
			case bsp.TransACM:
				acm = &o.Transactions[i]
			}
		}
	}
	if adm == nil || adm.RelatedDocument != "1252400000011" || adm.Fare != 11000 || adm.MemoReason != bsp.MemoFareDifference {
		t.Errorf("debit memo %+v", adm)
	}
	if acm == nil || acm.RelatedDocument != "1252400000012" || acm.Fare != -11000 {
		t.Errorf("credit memo %+v", acm)
	}
	// The written file carries the memos' related documents back.
	var buf bytes.Buffer
	if err := st.File.Write(&buf); err != nil {
		t.Fatal(err)
	}
	back, err := bsp.Parse(&buf)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, o := range back.Offices {
		for _, tx := range o.Transactions {
			if (tx.Code == bsp.TransADM || tx.Code == bsp.TransACM) && tx.RelatedDocument != "" {
				found++
			}
		}
	}
	if found != 2 {
		t.Errorf("memos read back with their related documents: %d", found)
	}
}
