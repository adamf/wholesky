package sim

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/gateway"
)

// Settlement across machines: the distribution system's machine settles
// the agent's book, the region settles its carriers' books, and the core
// merges the two views and proxies for a file it does not hold.
func TestSettlementFederatesAcrossMachines(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreAddr := freeAddr(t)
	coreURL := "http://" + coreAddr
	core, err := BootCore(ctx, m, Options{Console: coreAddr, Log: log}, "127.0.0.1")
	if err != nil {
		t.Fatalf("core: %v", err)
	}
	defer core.Sim.Stop()
	gdsAddr := freeAddr(t)
	g, err := BootGDS(ctx, m, Options{Log: log, SettleEvery: 200 * time.Millisecond}, coreURL, "http://"+gdsAddr, "1G")
	if err != nil {
		t.Fatalf("gds: %v", err)
	}
	defer g.Sim.Stop()
	serveMux(t, ctx, gdsAddr, g.Mux)
	regAddr := freeAddr(t)
	r, err := BootRegion(ctx, m, Options{Log: log, SettleEvery: 200 * time.Millisecond}, coreURL, "http://"+regAddr, 0, 1)
	if err != nil {
		t.Fatalf("region: %v", err)
	}
	defer r.Sim.Stop()
	serveMux(t, ctx, regAddr, r.Mux)

	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	waitFor("every subscriber to dial the core", func() bool { return len(core.Sim.Switch.LivePeers()) >= len(m.Carriers)+1 })

	c := m.Carriers[0]
	f := r.Sim.Flights[c.Designator][0]
	res, err := g.Sim.GDS.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: "SETTLED", Given: "ANN", Title: "MS"}},
		Segments: []gateway.BookingSegment{{Carrier: f.Carrier, FlightNum: f.Number, Class: "Y",
			Date: strings.ToUpper(g.Sim.BookingDate.Format("02Jan")), Board: f.From, Off: f.To, Seats: 1}},
		Agent: "test", Channel: "test",
	})
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	waitFor("the booking to settle", func() bool { ok, _ := settledIn(ctx, g.Sim.GDSStore, res.PNR.RecordLocator); return ok })
	if _, err := g.Sim.GDS.IssueTickets(ctx, res.PNR.RecordLocator, gateway.IssueOptions{AirlineCode: g.Sim.accountingCode(c.Designator), IssuedBy: "1G"}); err != nil {
		t.Fatal(err)
	}
	waitFor("the carrier to hold the ticket", func() bool {
		recs, _ := r.Sim.Tenants[c.Designator].Store.ListPNRs(ctx, 1000)
		for _, rec := range recs {
			if len(rec.Tickets) > 0 {
				return true
			}
		}
		return false
	})

	// Nobody calls the plan here: each machine's own loop runs it.
	waitFor("both shards to settle on their own", func() bool {
		gs, rs := g.Sim.Settlement(), r.Sim.Settlement()
		return gs != nil && gs.Transactions == 1 && rs != nil && rs.Transactions == 1
	})
	gs, rs := g.Sim.Settlement(), r.Sim.Settlement()
	if gs == nil || gs.Transactions != 1 || gs.Unverified != 0 {
		t.Errorf("the agent's machine: %+v", gs)
	}
	if rs == nil || rs.Transactions != 1 || rs.Unverified != 1 || rs.Matched != 0 {
		t.Errorf("the region, from the carrier's copy alone: %+v", rs)
	}
	// The core merges what the shards answer and proxies for the file.
	if n := core.refreshSettlement(&http.Client{Timeout: 5 * time.Second}); n != 2 {
		t.Fatalf("shards answering: %d", n)
	}
	cs := core.Sim.Settlement()
	if cs == nil || cs.Transactions != 2 || cs.Airlines != 1 {
		t.Fatalf("core merge: %+v", cs)
	}
	rec := httptest.NewRecorder()
	core.Sim.serveHOT(rec, httptest.NewRequest("GET", "/settlement/"+c.Designator+".hot", nil))
	if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "BFH") {
		t.Errorf("core proxy for the file: %d %q", rec.Code, rec.Body.String()[:min(60, rec.Body.Len())])
	}
}
