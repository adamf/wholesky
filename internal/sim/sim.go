// Package sim boots and runs a world.
//
// There is exactly one wiring: skyd runs it as a daemon, the tests run it in
// a process and assert on it. A test standing on its own copy of the boot
// would be testing the copy -- the lesson Jetway's scenario suite already
// paid for.
package sim

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/config"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/node"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"

	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// The GDS identity for phase one. There is exactly one so far; the design
// calls for five.
const (
	GDSDesignator = "1G"
	GDSAddress    = "LONDD1G"
)

// Options configure a boot.
type Options struct {
	// Carriers caps how many carriers run, largest first. Zero runs all.
	Carriers int
	// Console is the switch console address; empty skips the console.
	Console string
	// Capacity is seats per class per flight at every tenant.
	Capacity int
	// LinkWait bounds how long Boot waits for the fabric.
	LinkWait time.Duration
	Log      *slog.Logger
}

// Sim is a running world.
type Sim struct {
	Manifest    *world.Manifest
	Switch      *node.Node
	GDS         *gateway.Gateway
	GDSStore    store.Store
	Tenants     map[string]*host.Tenant
	Flights     map[string][]world.Flight
	BookingDate time.Time
	// Movements counts EvMovement events seen at the GDS: flights whose
	// departures and arrivals crossed the switch and were recognised.
	Movements atomic.Int64
	// gdsUp flips when the GDS's own client link is established. The switch
	// counting a session is not enough: there is a window where the listener
	// has accepted the connection but the client has not yet marked its link
	// usable, and a booking placed in that window sends into a link that is
	// "not up" -- recorded, undeliverable, and with no router behind this
	// sender, never retried. The first live run's two "free sale" bookings
	// were actually this.
	gdsUp atomic.Bool

	log    *slog.Logger
	cancel context.CancelFunc
}

// Boot stands the topology up and waits for every link.
func Boot(ctx context.Context, m *world.Manifest, opts Options) (*Sim, error) {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	if opts.Carriers > 0 && opts.Carriers < len(m.Carriers) {
		m.Carriers = m.Carriers[:opts.Carriers]
	}
	kept := map[string]bool{}
	for _, c := range m.Carriers {
		kept[c.Designator] = true
	}
	flights := map[string][]world.Flight{}
	for _, f := range m.Flights {
		if kept[f.Carrier] {
			flights[f.Carrier] = append(flights[f.Carrier], f)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Sim{
		Manifest: m, Tenants: map[string]*host.Tenant{}, Flights: flights,
		BookingDate: time.Now().UTC().AddDate(0, 0, 30),
		log:         log, cancel: cancel,
	}

	sw, err := buildSwitch(ctx, m, opts.Console, log)
	if err != nil {
		cancel()
		return nil, err
	}
	s.Switch = sw
	if err := sw.Start(ctx); err != nil {
		s.Stop()
		return nil, err
	}
	if opts.Console != "" {
		go func() {
			if err := sw.Serve(ctx, 10*time.Second); err != nil && ctx.Err() == nil {
				log.Error("switch console stopped", "err", err)
			}
		}()
	}

	capacity := opts.Capacity
	if capacity <= 0 {
		capacity = 100000
	}
	for _, c := range m.Carriers {
		t, err := host.Start(ctx, c, flights[c.Designator], host.Options{
			SwitchAddr:   sw.Addr("link-" + strings.ToLower(c.Designator)),
			WatchAddress: GDSAddress,
			Capacity:     capacity,
			BookingDate:  s.BookingDate,
			Log:          log,
		})
		if err != nil {
			s.Stop()
			return nil, fmt.Errorf("start carrier %s: %w", c.Designator, err)
		}
		s.Tenants[c.Designator] = t
	}

	gds, gdsStore, err := s.buildGDS(ctx, m, sw.Addr("link-gds"), log)
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.GDS, s.GDSStore = gds, gdsStore

	wait := opts.LinkWait
	if wait <= 0 {
		wait = 30 * time.Second
	}
	if err := s.waitForLinks(ctx, len(m.Carriers)+1, wait); err != nil {
		s.Stop()
		return nil, err
	}
	return s, nil
}

// Stop tears the world down.
func (s *Sim) Stop() {
	s.cancel()
	if s.Switch != nil {
		s.Switch.Close()
	}
}

// Book places one booking through the GDS and returns its locator.
func (s *Sim) Book(ctx context.Context, f world.Flight, class string, dayOffset int, surname string) (*gateway.BookResult, error) {
	date := s.BookingDate.AddDate(0, 0, dayOffset)
	return s.GDS.Book(ctx, &gateway.BookingRequest{
		Passengers: []gateway.BookingPassenger{{Surname: surname, Given: "SIM", Title: "MR"}},
		Segments: []gateway.BookingSegment{{
			Carrier: f.Carrier, FlightNum: f.Number, Class: class,
			Date:  strings.ToUpper(date.Format("02Jan")),
			Board: f.From, Off: f.To, Seats: 1,
		}},
		Agent: "wholesky", Channel: "sim",
	})
}

// Settled reports whether every air segment of a record has been answered.
func (s *Sim) Settled(ctx context.Context, locator string) (bool, error) {
	rec, err := s.GDSStore.GetPNR(ctx, locator)
	if err != nil {
		return false, err
	}
	for _, sg := range rec.Segments {
		if sg.Status == "HN" || sg.Status == "NN" {
			return false, nil
		}
	}
	return true, nil
}

// Demand generates bookings continuously at roughly perMinute, spread across
// carriers, cabins and a window of departure dates.
//
// This is a placeholder for the real demand model -- no booking curve, no
// channels, no parties larger than one -- but it is a load, not a smoke test:
// it runs until stopped and reports what settled. Randomness is seeded, so a
// run is as repeatable as its timing allows.
func (s *Sim) Demand(ctx context.Context, perMinute int, seed int64) {
	if perMinute <= 0 {
		return
	}
	rng := rand.New(rand.NewSource(seed))
	classes := []string{"Y", "Y", "Y", "M", "M", "J", "F"} // a rough cabin mix
	var carriers []string
	for code := range s.Flights {
		if len(s.Flights[code]) > 0 {
			carriers = append(carriers, code)
		}
	}
	if len(carriers) == 0 {
		return
	}
	// Deterministic carrier order for the seeded draws.
	for i := 1; i < len(carriers); i++ {
		for j := i; j > 0 && carriers[j] < carriers[j-1]; j-- {
			carriers[j], carriers[j-1] = carriers[j-1], carriers[j]
		}
	}

	var booked, failed atomic.Int64
	interval := time.Minute / time.Duration(perMinute)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	report := time.NewTicker(30 * time.Second)
	defer report.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-report.C:
			s.log.Info("demand", "booked", booked.Load(), "failed", failed.Load(),
				"movements", s.Movements.Load())
		case <-tick.C:
			n++
			code := carriers[rng.Intn(len(carriers))]
			fs := s.Flights[code]
			f := fs[rng.Intn(len(fs))]
			class := classes[rng.Intn(len(classes))]
			day := rng.Intn(3) - 1
			go func(i int) {
				_, err := s.Book(ctx, f, class, day, fmt.Sprintf("DEMAND%06d", i))
				if err != nil {
					failed.Add(1)
					s.log.Debug("booking refused", "flight", f.Carrier+f.Number, "class", class, "err", err)
					return
				}
				booked.Add(1)
			}(n)
		}
	}
}

// FlyDay walks the schedule against warped wall time, emitting departures and
// arrivals as they come due.
func (s *Sim) FlyDay(ctx context.Context, warp int) {
	if warp < 1 {
		warp = 1
	}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	start := time.Now()
	prev := 0
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		cur := int(time.Since(start).Minutes()*float64(warp)) % (24 * 60)
		if cur < prev {
			prev = 0 // the day wrapped; begin again
		}
		for code, t := range s.Tenants {
			for _, f := range s.Flights[code] {
				reg := fmt.Sprintf("SKY%03d", regHash(f)%1000)
				if f.DepMin > prev && f.DepMin <= cur {
					if err := t.Depart(ctx, f, day, reg, 0); err != nil {
						s.log.Debug("departure not sent", "flight", f.Carrier+f.Number, "err", err)
					}
				}
				if f.ArrMin > prev && f.ArrMin <= cur {
					if err := t.Arrive(ctx, f, day, reg, 0); err != nil {
						s.log.Debug("arrival not sent", "flight", f.Carrier+f.Number, "err", err)
					}
				}
			}
		}
		prev = cur
	}
}

func buildSwitch(ctx context.Context, m *world.Manifest, console string, log *slog.Logger) (*node.Node, error) {
	cfg := config.Default()
	cfg.Identity = config.Identity{Designator: "1X", TTYAddress: "XCHDD1X", Name: "wholesky switch"}
	cfg.HTTP.Addr = console
	if console == "" {
		cfg.HTTP.Addr = "127.0.0.1:0"
	}
	cfg.Store = config.Store{Backend: "mem"}
	cfg.Spool.Enabled = false
	cfg.Demo.Carriers = false
	cfg.Routing.Relay = true
	cfg.Ingress = nil
	cfg.Peers = nil

	add := func(name, designator, tty, format string) {
		cfg.Ingress = append(cfg.Ingress, config.Ingress{
			Name: name, Type: "tcp", Addr: "127.0.0.1:0",
			Identify: config.Identify{Peer: designator},
		})
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: designator, Carrier: designator, TTYAddress: tty,
			Format: format,
			Egress: config.Egress{Type: "tcp_accept"},
		})
	}
	for _, c := range m.Carriers {
		add("link-"+strings.ToLower(c.Designator), c.Designator, c.TTYAddress, c.Format)
	}
	add("link-gds", GDSDesignator, GDSAddress, "typeb")

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("switch config: %w", err)
	}
	return node.Build(ctx, cfg, log.With("node", "switch"), node.Options{
		LocatorSecret: []byte("wholesky-switch"),
		SkipConsole:   console == "",
	})
}

// buildGDS assembles the distribution side: one gateway, one link to the
// switch, and a peer entry per carrier in that carrier's own dialect --
// AIRIMP over Type B or PADIS over EDIFACT, routed either way by the switch.
func (s *Sim) buildGDS(ctx context.Context, m *world.Manifest, switchAddr string, log *slog.Logger) (*gateway.Gateway, store.Store, error) {
	st := store.NewMem()
	bus := gateway.NewBus(256)
	gw := gateway.New(gateway.Identity{
		Designator: GDSDesignator, TTYAddress: GDSAddress, Name: "wholesky gds",
	}, st, bus, log.With("node", "gds"), []byte("wholesky-gds"))
	gw.Avail = avail.NewCache()

	client := &transport.Client{
		Addr: switchAddr, Framer: transport.DefaultFramer(),
		Log: log.With("node", "gds"), SkipHello: true,
	}
	gw.Sender = client
	client.OnMessage = func(ctx context.Context, peer string, raw []byte) error {
		_, err := gw.Ingest(ctx, "net", raw)
		return err
	}
	client.OnUp = func() { s.gdsUp.Store(true) }
	gw.AddPeer(&gateway.Peer{Name: "net", Format: store.FormatTypeB, TTYAddress: "XCHDD1X"})
	for _, c := range m.Carriers {
		format := store.FormatTypeB
		if c.Format == "edifact" {
			format = store.FormatEDIFACT
		}
		gw.AddPeer(&gateway.Peer{
			Name: c.Designator, Carrier: c.Designator,
			Format: format, TTYAddress: c.TTYAddress,
		})
	}

	sub, unsub := bus.Subscribe()
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-sub:
				if ev.Type == gateway.EvMovement {
					s.Movements.Add(1)
				}
			}
		}
	}()
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("gds link ended", "err", err)
		}
	}()
	return gw, st, nil
}

func (s *Sim) waitForLinks(ctx context.Context, want int, within time.Duration) error {
	deadline := time.Now().Add(within)
	for {
		if len(s.Switch.LivePeers()) >= want && s.gdsUp.Load() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("only %d of %d links came up: %v",
				len(s.Switch.LivePeers()), want, s.Switch.LivePeers())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func regHash(f world.Flight) int {
	h := 0
	for _, r := range f.Carrier + f.Number {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}
