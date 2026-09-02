package sim

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func serveMux(t *testing.T, ctx context.Context, addr string, mux *http.ServeMux) {
	t.Helper()
	srv := &http.Server{Addr: addr, Handler: mux}
	go srv.ListenAndServe() //nolint:errcheck
	t.Cleanup(func() { srv.Close() })
}

// The world across machines, in one test process over loopback: a core with
// the switch and the instruments, one distribution system dialling in, and
// one region carrying every carrier. A booking placed at the GDS machine is
// answered by a tenant on the region machine through the core's switch; a
// departure on the region flies a plane on the core's globe; the core's
// fleet board carries every peer's rows and proxies their drill-downs; and
// a warp change on the core reaches the region on the next heartbeat.
func TestMultiMachineWorld(t *testing.T) {
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
	g, err := BootGDS(ctx, m, Options{Log: log}, coreURL, "http://"+gdsAddr, "1G")
	if err != nil {
		t.Fatalf("gds: %v", err)
	}
	defer g.Sim.Stop()
	serveMux(t, ctx, gdsAddr, g.Mux)

	regAddr := freeAddr(t)
	r, err := BootRegion(ctx, m, Options{Log: log}, coreURL, "http://"+regAddr, 0, 1)
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

	if len(core.Sim.GDSes) != 0 || len(core.Sim.Eye.Hubs) != len(gdsSlots) {
		t.Fatalf("core builds no GDS of its own and hubs every slot once: gdses=%d hubs=%v",
			len(core.Sim.GDSes), core.Sim.Eye.Hubs)
	}

	// Every carrier and the GDS hold sessions on the core's switch.
	want := len(m.Carriers) + 1
	waitFor("every subscriber to dial the core",
		func() bool { return len(core.Sim.Switch.LivePeers()) >= want })

	// A booking at the GDS machine settles: the sell crossed two machines,
	// the tenant answered, the reply crossed back.
	c := m.Carriers[0]
	f := r.Sim.Flights[c.Designator][0]
	res, err := g.Sim.GDS.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: "FEDERATED", Given: "ANN", Title: "MS"}},
		Segments: []gateway.BookingSegment{{
			Carrier: f.Carrier, FlightNum: f.Number, Class: "Y",
			Date:  strings.ToUpper(g.Sim.BookingDate.Format("02Jan")),
			Board: f.From, Off: f.To, Seats: 1}},
		Agent: "test", Channel: "test",
	})
	if err != nil {
		t.Fatalf("Book across machines: %v", err)
	}
	waitFor("the cross-machine booking to settle", func() bool {
		ok, _ := settledIn(ctx, g.Sim.GDSStore, res.PNR.RecordLocator)
		return ok
	})
	recs, _ := r.Sim.Tenants[c.Designator].Store.FindPNRsByFlight(ctx,
		f.Carrier+f.Number, strings.ToUpper(g.Sim.BookingDate.Format("02Jan")), 10)
	if len(recs) == 0 {
		t.Error("the region's tenant holds no copy of the cross-machine booking")
	}

	// A departure on the region flies a plane on the core's globe: the MVT
	// transits the core's switch, and the switch's bus is the sky.
	tn := r.Sim.Tenants[c.Designator]
	if err := tn.Depart(ctx, f, time.Now().UTC().Truncate(24*time.Hour), "WTEST01", 0); err != nil {
		t.Fatalf("Depart: %v", err)
	}
	waitFor("the movement to reach the core's globe",
		func() bool { return core.Sim.Eye.Airborne() > 0 })

	// The core's board carries every peer's rows and proxies drill-downs.
	waitFor("the federated fleet to include the region's carriers", func() bool {
		resp, err := http.Get(coreURL + "/fleet/nodes.json")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		return strings.Contains(string(b), `"`+c.Designator+`"`) &&
			strings.Contains(string(b), `"1G"`)
	})
	resp, err := http.Get(coreURL + "/fleet/node/" + c.Designator + "/detail.json")
	if err != nil {
		t.Fatalf("proxied drill-down: %v", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), "records") {
		t.Errorf("proxied drill-down = %d %s", resp.StatusCode, body)
	}

	// The node consoles the fleet page links to are proxied to whichever
	// machine runs them: a tenant console through the core answers.
	resp, err = http.Get(coreURL + "/node/" + c.Designator + "/api/status")
	if err != nil {
		t.Fatalf("proxied console: %v", err)
	}
	body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("proxied console = %d %.80s", resp.StatusCode, body)
	}

	// Time control propagates on the heartbeat.
	if err := core.Sim.SetWarp(300); err != nil {
		t.Fatal(err)
	}
	waitFor("the region to follow the core's warp",
		func() bool { return r.Sim.clock.Warp() == 300 })

	// The federated bookings reach the core's instruments: the poll folds
	// peer totals into the stats collector the panel serves.
	waitFor("the booking to reach the core's stats", func() bool {
		resp, err := http.Get(coreURL + "/stats/data.json")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		var out struct {
			Totals map[string]int64 `json:"totals"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil {
			return false
		}
		return out.Totals["bookings"] >= 1
	})

	// And the drill-through finds the booking from the core, via the GDS
	// machine's shard endpoint.
	waitFor("the federated drill-through to find the booking", func() bool {
		recs := core.Sim.Eye.FlightPNRs(f.Carrier+f.Number, f.From)
		for _, rec := range recs {
			if rec.Locator == res.PNR.RecordLocator {
				return true
			}
		}
		return false
	})
}

// A region flies its slice of the day the way the single box does: a
// departure reaches its datalink provider, which reports it to the airline
// over the core's switch, and the movement comes out the other side.
func TestRegionFliesItsDay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreAddr := freeAddr(t)
	coreURL := "http://" + coreAddr
	core, err := BootCore(ctx, m, Options{Console: coreAddr, Log: log, Warp: 60}, "127.0.0.1")
	if err != nil {
		t.Fatalf("core: %v", err)
	}
	defer core.Sim.Stop()
	regAddr := freeAddr(t)
	r, err := BootRegion(ctx, m, Options{Log: log, Warp: 60}, coreURL, "http://"+regAddr, 0, 1)
	if err != nil {
		t.Fatalf("region: %v", err)
	}
	defer r.Sim.Stop()
	serveMux(t, ctx, regAddr, r.Mux)
	deadline := time.Now().Add(30 * time.Second)
	for len(core.Sim.Switch.LivePeers()) < len(m.Carriers)+2 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if r.Sim.DSP == nil {
		t.Fatal("the region runs no datalink provider")
	}
	go r.Sim.FlyDay(ctx)
	deadline = time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		msgs, _ := r.Sim.DSP.Store.ListMessages(ctx, store.MessageFilter{Limit: 5})
		if len(msgs) > 0 && core.Sim.Movements.Load() > 0 {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	msgs, _ := r.Sim.DSP.Store.ListMessages(ctx, store.MessageFilter{Limit: 5})
	t.Fatalf("after 40s at warp 60 the region's datalink provider holds %d messages and the core saw %d movements (departures issued %d, report failures %d)", len(msgs), core.Sim.Movements.Load(), r.Sim.Departures.Load(), r.Sim.reportErrs.Load())
}
