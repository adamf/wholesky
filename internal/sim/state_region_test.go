package sim

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

// A region has no volume of its own: what its restart must not forget is
// kept at the core, under the region's name, and comes back when the region
// boots again against the same core.
func TestRegionStateLivesAtTheCore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreAddr := freeAddr(t)
	core, err := BootCore(ctx, m, Options{Console: coreAddr, Log: log, StateFile: filepath.Join(t.TempDir(), "core.json")}, "127.0.0.1")
	if err != nil {
		t.Fatalf("core: %v", err)
	}
	defer core.Sim.Stop()
	regAddr := freeAddr(t)
	r, err := BootRegion(ctx, m, Options{Log: log}, "http://"+coreAddr, "http://"+regAddr, 0, 1)
	if err != nil {
		t.Fatalf("region: %v", err)
	}
	serveMux(t, ctx, regAddr, r.Mux)
	code := m.Carriers[0].Designator
	_, token, err := r.Sim.Airline.Take(code, "Adam")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if b, ok := core.Sim.peerState("region0"); ok && len(b) > 0 && containsAll(string(b), code, "Adam") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the region's state never reached the core")
		}
		time.Sleep(100 * time.Millisecond)
	}
	r.Sim.Stop()

	regAddr2 := freeAddr(t)
	r2, err := BootRegion(ctx, m, Options{Log: log}, "http://"+coreAddr, "http://"+regAddr2, 0, 1)
	if err != nil {
		t.Fatalf("region again: %v", err)
	}
	defer r2.Sim.Stop()
	seat, held := r2.Sim.Airline.Seat(code)
	if !held || seat.Holder != "Adam" || !r2.Sim.Airline.Authorised(code, token) {
		t.Fatalf("seat after the region's restart: %+v held %v", seat, held)
	}
}
