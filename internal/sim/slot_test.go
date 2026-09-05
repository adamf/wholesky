package sim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/atfm"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/wholesky/internal/dayplan"
)

// The Network Manager's slot, end to end: the ANSP issues a SAM for a
// regulated flight over the AFTN, the carrier's operations centre hears it
// through jetway's Ground seam, and the panel shows the CTOT with the
// regulation and its cause. The day plan itself is built at boot and
// knows every flight.
func TestSlotReachesTheOperationsCentre(t *testing.T) {
	s := bootWorld(t, Options{})
	ctx := context.Background()
	if s.Fate() == nil || s.Fate().Summary.Flights == 0 {
		t.Fatal("the world has no day plan")
	}
	code := s.Manifest.Carriers[0].Designator
	f := s.Flights[code][0]
	tn := s.Tenants[code]
	res, err := s.Book(ctx, f, "Y", 0, "SLOT")
	if err != nil {
		t.Fatal(err)
	}
	settleAll(t, s, []string{res.PNR.RecordLocator})
	quiesce(t, s)
	fate := dayplan.Flight{DepDelay: 40, ArrDelay: 40, ATFM: 40, CTOT: f.DepMin + 40 + dayplan.TaxiMin,
		Regulation: "KJFKA26A", Cause: atfm.NewCause(atfm.CauseWeather, 'A')}
	if s.ANSP == nil {
		t.Skip("this world runs no ANSP")
	}
	if err := s.ANSP.Slot(ctx, s.carriers[code], f, s.BookingDate, fate); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		if tn.SlotsHeard()["SAM"] >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("operations never heard the slot: %v", tn.SlotsHeard())
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := tn.SendPNL(ctx, f, s.BookingDate); err != nil {
		t.Fatal(err)
	}
	key := dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(s.BookingDate.Format("02Jan")), Board: f.From}
	deadline = time.Now().Add(20 * time.Second)
	for {
		if _, err := tn.DCS.Flight(key); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the name list never reached the airport")
		}
		time.Sleep(50 * time.Millisecond)
	}
	sum, ok := tn.Summarise(f.Carrier+f.Number, f.From)
	if !ok || !strings.Contains(sum.Slot, "CTOT "+hhmm(fate.CTOT)) || !strings.Contains(sum.Slot, "KJFKA26A") || !strings.Contains(sum.Slot, "WA 84") {
		t.Fatalf("panel slot line %q (ok %v)", sum.Slot, ok)
	}
	// A slot for a flight that is not ours is counted, not filed.
	other := f
	other.Number = "9999"
	if err := s.ANSP.Slot(ctx, s.carriers[code], other, s.BookingDate, fate); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if n := tn.SlotsHeard()["SAM"]; n != 2 {
		t.Errorf("slots heard %d, want 2", n)
	}
}
