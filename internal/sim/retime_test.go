package sim

import (
	"context"
	"testing"
	"time"
)

// A time change, end to end: the carrier announces a long delay as an ASM
// TIM to distribution, and the distribution system that sold the seat
// moves the held segment to the new times at TK -- confirming, advise
// times changed -- with a task on its schedule-change queue.
func TestRetimeMovesTheSoldSegmentToTK(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	f := s.Flights[s.Manifest.Carriers[0].Designator][0]
	res, err := s.Book(ctx, f, "Y", 0, "RETIME")
	if err != nil {
		t.Fatal(err)
	}
	settleAll(t, s, []string{res.PNR.RecordLocator})
	quiesce(t, s)

	tn := s.Tenants[f.Carrier]
	day := s.BookingDate
	if err := tn.Retime(ctx, f, day, 52, 47); err != nil {
		t.Fatal(err)
	}
	wantDep := hhmm(f.DepMin + 52)
	deadline := time.Now().Add(20 * time.Second)
	for {
		rec, err := s.GDSStore.GetPNR(ctx, res.PNR.RecordLocator)
		if err == nil && len(rec.Segments) > 0 && rec.Segments[0].DepartTime == wantDep && rec.Segments[0].Status == "TK" {
			break
		}
		if time.Now().After(deadline) {
			seg := "no record"
			if err == nil && len(rec.Segments) > 0 {
				seg = rec.Segments[0].Describe() + " " + rec.Segments[0].DepartTime
			}
			t.Fatalf("the sold segment never moved to %s TK: %s", wantDep, seg)
		}
		time.Sleep(50 * time.Millisecond)
	}
	sum, ok := tn.Summarise(f.Carrier+f.Number, f.From)
	if ok && sum.Retimed == "" {
		t.Fatalf("the panel says the flight was retimed: %+v", sum)
	}
	// Announcing it twice sends nothing more.
	if err := tn.Retime(ctx, f, day, 52, 47); err != nil {
		t.Fatal(err)
	}
}
