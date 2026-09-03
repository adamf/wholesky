package sim

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/bsp"
	"github.com/adamf/jetway/pkg/gateway"
)

// The plan settles the day the agents sold: every airline with a ticketed
// sale is handed a HOT that parses back to the same documents, the sums
// agree with the plan's, and the carrier's own book reconciles -- every
// document the plan reports, the carrier holds.
func TestSettlementHandsEachAirlineAHOTThatReconciles(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	// The trimmed world is not filled; sell and ticket a few seats so there
	// is something to settle.
	c := s.Manifest.Carriers[0]
	var locs []string
	for i := 0; i < 3 && i < len(s.Flights[c.Designator]); i++ {
		res, err := s.Book(ctx, s.Flights[c.Designator][i], "Y", 0, "SETTLE"+string(rune('A'+i)))
		if err != nil {
			t.Fatal(err)
		}
		locs = append(locs, res.PNR.RecordLocator)
	}
	settleAll(t, s, locs)
	for _, l := range locs {
		if _, err := s.GDS.IssueTickets(ctx, l, gateway.IssueOptions{AirlineCode: s.accountingCode(c.Designator), IssuedBy: "1G"}); err != nil {
			t.Fatal(err)
		}
	}
	quiesce(t, s)
	// The carrier hears its ticket numbers over the wire (SSR TKNE); wait
	// until its book holds them before the plan reconciles against it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		held, _ := s.Tenants[c.Designator].Store.ListPNRs(ctx, 1000)
		n := 0
		for _, r := range held {
			n += len(r.Tickets)
		}
		if n >= len(locs) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the carrier holds %d tickets, want %d", n, len(locs))
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.Settle(ctx)
	sum := s.Settlement()
	if sum == nil || sum.Airlines == 0 || sum.Transactions == 0 {
		t.Fatalf("nothing settled: %+v", sum)
	}
	if sum.Gross <= 0 || sum.Remittance <= 0 || sum.Commission >= 0 {
		t.Errorf("sums: gross %d remittance %d commission %d", sum.Gross, sum.Remittance, sum.Commission)
	}
	if sum.Unreported != 0 {
		t.Errorf("%d documents the plan reports are unknown to their carriers", sum.Unreported)
	}
	if sum.Matched == 0 {
		t.Errorf("no document matched a carrier's book: %+v", sum)
	}
	// Pick the airline with the most and read its file back.
	var best string
	for code, st := range sum.Statements {
		if best == "" || st.Transactions > sum.Statements[best].Transactions {
			best = code
		}
	}
	rec := httptest.NewRecorder()
	s.serveHOT(rec, httptest.NewRequest("GET", "/settlement/"+best+".hot", nil))
	if rec.Code != 200 {
		t.Fatalf("HOT for %s: %d", best, rec.Code)
	}
	body := rec.Body.String()
	for _, l := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if len(l) != bsp.RecordLen {
			t.Fatalf("record of %d characters: %q", len(l), l)
		}
	}
	f, err := bsp.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	st := sum.Statements[best]
	n := 0
	var gross int64
	for _, o := range f.Offices {
		n += len(o.Transactions)
		tot := o.OfficeTotals()
		gross += tot.Gross
	}
	if n != st.Transactions || gross != st.Gross {
		t.Errorf("file for %s: %d transactions gross %d, plan says %d and %d", best, n, gross, st.Transactions, st.Gross)
	}
	if f.Airline != s.accountingCode(best) || f.BSP != "WSK" {
		t.Errorf("file header: %+v", f)
	}
	tx := f.Offices[0].Transactions[0]
	if tx.Code != bsp.TransSale || len(tx.Segments) == 0 || tx.Passenger == "" || len(tx.Payments) != 1 || tx.Payments[0].Type != bsp.PaymentCash {
		t.Errorf("first transaction: %+v", tx)
	}
	// The carriers bill each other for codeshare coupons off the same books.
	s.Bill(ctx)
	if b := s.Billing(); b == nil || b.Prorate < 0 {
		t.Fatalf("billing: %+v", b)
	}
	rec = httptest.NewRecorder()
	s.serveBilling(rec, httptest.NewRequest("GET", "/billing.json", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "\"invoices_detail\"") {
		t.Errorf("billing view: %d %s", rec.Code, rec.Body.String()[:min(200, rec.Body.Len())])
	}
	rec = httptest.NewRecorder()
	s.serveSettlement(rec, httptest.NewRequest("GET", "/settlement.json", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "\"airlines_detail\"") {
		t.Errorf("summary: %d %s", rec.Code, rec.Body.String()[:min(200, rec.Body.Len())])
	}
}
