package sim

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
)

// The demand model's whole point is that every lifecycle path gets walked:
// interline itineraries exist, tickets get issued, bookings get cancelled,
// parties divide, and NDC orders arrive over HTTP. Driving travellers
// synchronously with a fixed seed, all of it must show up in the GDS store.
func TestDemandWalksEveryLifecyclePath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()

	var carriers []string
	for code := range s.Flights {
		if len(s.Flights[code]) > 0 {
			carriers = append(carriers, code)
		}
	}
	for i := 0; i < 120; i++ {
		s.placeDemand(ctx, rand.New(rand.NewSource(int64(i))), carriers, i)
		if s.DemInterline.Load() > 0 && s.DemTicketed.Load() > 0 &&
			s.DemCancelled.Load() > 0 && s.DemSplit.Load() > 0 && s.DemNDC.Load() > 0 {
			break
		}
	}
	t.Logf("booked=%d failed=%d interline=%d ndc=%d ticketed=%d cancelled=%d split=%d",
		s.DemBooked.Load(), s.DemFailed.Load(), s.DemInterline.Load(), s.DemNDC.Load(),
		s.DemTicketed.Load(), s.DemCancelled.Load(), s.DemSplit.Load())

	for name, n := range map[string]int64{
		"interline bookings": s.DemInterline.Load(),
		"ndc orders":         s.DemNDC.Load(),
		"tickets issued":     s.DemTicketed.Load(),
		"cancellations":      s.DemCancelled.Load(),
		"divisions":          s.DemSplit.Load(),
	} {
		if n == 0 {
			t.Errorf("demand never produced %s; a lifecycle path is going unwalked", name)
		}
	}

	// The counters must be backed by the store, not just incremented: an
	// interline record really spans two carriers, and a ticketed record
	// really holds documents.
	recs, err := s.GDSStore.ListPNRs(ctx, 10000)
	if err != nil {
		t.Fatal(err)
	}
	var sawInterline, sawTicketed, sawCancelled bool
	statuses := map[string]int{}
	for _, r := range recs {
		statuses[string(r.Status)]++
		cs := map[string]bool{}
		for _, sg := range r.Segments {
			if sg.Type == pnr.SegmentAir {
				cs[sg.Carrier] = true
			}
		}
		if len(cs) >= 2 {
			sawInterline = true
		}
		if len(r.Tickets) > 0 {
			sawTicketed = true
		}
		if r.Status == pnr.StatusCancelled {
			sawCancelled = true
		}
	}
	if !sawInterline {
		t.Error("no record in the store spans two carriers")
	}
	if !sawTicketed {
		t.Error("no record in the store holds a ticket")
	}
	if !sawCancelled {
		s.demMu.Lock()
		locs := append([]string(nil), s.DemCancelledLocs...)
		s.demMu.Unlock()
		for _, l := range locs {
			r, err := s.GDSStore.GetPNR(ctx, l)
			if err != nil {
				t.Logf("cancelled locator %s: store says %v", l, err)
				continue
			}
			t.Logf("cancelled locator %s: status=%s version=%d", l, r.Status, r.Version)
			evs, _ := s.GDSStore.Events(ctx, r.ID)
			for _, e := range evs {
				t.Logf("   %s: %s", e.Type, e.Detail)
			}
		}
		t.Errorf("no record in the store is cancelled; statuses: %v of %d records", statuses, len(recs))
	}
}
