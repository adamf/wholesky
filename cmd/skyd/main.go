// Command skyd boots a world.
//
// It reads a compiled manifest and stands the topology up the way the real
// one is shaped: a message switch in the middle (a Jetway node in relay mode),
// carrier reservation systems as tenants of a host process, and a GDS node
// that reaches every carrier through its single switch link. All of it over
// real TCP on loopback -- framing, reconnection and partial reads included,
// because those are the parts most likely to break.
//
//	worldc -countries "United Kingdom,France" -carriers 8 -o europe.json
//	skyd -world europe.json -carriers 8 -book 5 -warp 60
//
// The flight day runs against the wall clock multiplied by -warp: at 60, a
// day of departures and arrivals plays out in 24 minutes, each one a real MVT
// crossing the switch.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
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
	gdsDesignator = "1G"
	gdsAddress    = "LONDD1G"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "skyd:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		worldPath = flag.String("world", "world.json", "compiled manifest to boot")
		carriers  = flag.Int("carriers", 8, "how many carriers to run, largest first; 0 for all")
		warp      = flag.Int("warp", 60, "sim minutes per real minute for the flight day")
		book      = flag.Int("book", 3, "bookings to push through once the links are up")
		console   = flag.String("console", "127.0.0.1:8090", "switch console address")
		verbose   = flag.Bool("v", false, "debug logging")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	m, err := loadManifest(*worldPath)
	if err != nil {
		return err
	}
	if *carriers > 0 && *carriers < len(m.Carriers) {
		m.Carriers = m.Carriers[:*carriers]
	}
	// Phase one runs the fabric in Type B; the switch does not yet relay
	// EDIFACT by its UNB recipient, and pretending it does would sell seats
	// nobody can confirm.
	kept := map[string]bool{}
	for i := range m.Carriers {
		m.Carriers[i].Format = "typeb"
		kept[m.Carriers[i].Designator] = true
	}
	flightsByCarrier := map[string][]world.Flight{}
	total := 0
	for _, f := range m.Flights {
		if kept[f.Carrier] {
			flightsByCarrier[f.Carrier] = append(flightsByCarrier[f.Carrier], f)
			total++
		}
	}
	log.Info("world loaded", "manifest", *worldPath, "carriers", len(m.Carriers),
		"flights_per_day", total)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- the switch: one Jetway node in relay mode --------------------------
	sw, err := buildSwitch(ctx, m, *console, log)
	if err != nil {
		return err
	}
	defer sw.Close()
	if err := sw.Start(ctx); err != nil {
		return err
	}
	go func() {
		if err := sw.Serve(ctx, 10*time.Second); err != nil && ctx.Err() == nil {
			log.Error("switch console stopped", "err", err)
		}
	}()

	// --- the carriers: tenants dialling their circuits ----------------------
	bookingDate := time.Now().UTC().AddDate(0, 0, 30)
	tenants := map[string]*host.Tenant{}
	for _, c := range m.Carriers {
		t, err := host.Start(ctx, c, flightsByCarrier[c.Designator], host.Options{
			SwitchAddr:   sw.Addr("link-" + strings.ToLower(c.Designator)),
			WatchAddress: gdsAddress,
			Capacity:     100000,
			BookingDate:  bookingDate,
			Log:          log,
		})
		if err != nil {
			return fmt.Errorf("start carrier %s: %w", c.Designator, err)
		}
		tenants[c.Designator] = t
	}

	// --- the GDS: one gateway, one link, everyone reachable through it ------
	gds, movements, err := buildGDS(ctx, m, sw.Addr("link-gds"), log)
	if err != nil {
		return err
	}

	if err := waitForLinks(ctx, sw, len(m.Carriers)+1, 30*time.Second); err != nil {
		return err
	}
	log.Info("fabric up", "links", len(sw.LivePeers()), "console", "http://"+*console)

	// --- prove the fabric: bookings through the switch ----------------------
	booked := 0
	var locators []string
	for i := 0; booked < *book && i < len(m.Carriers)*3; i++ {
		c := m.Carriers[i%len(m.Carriers)]
		fs := flightsByCarrier[c.Designator]
		if len(fs) == 0 {
			continue
		}
		f := fs[i%len(fs)]
		res, err := gds.Book(ctx, &gateway.BookingRequest{
			Passengers: []gateway.BookingPassenger{{
				Surname: fmt.Sprintf("SKY%04d", i), Given: "TEST", Title: "MR",
			}},
			Segments: []gateway.BookingSegment{{
				Carrier: f.Carrier, FlightNum: f.Number, Class: "Y",
				Date:  strings.ToUpper(bookingDate.Format("02Jan")),
				Board: f.From, Off: f.To, Seats: 1,
			}},
			Agent: "skyd", Channel: "sim",
		})
		if err != nil {
			log.Warn("booking failed", "carrier", c.Designator, "flight", f.Number, "err", err)
			continue
		}
		booked++
		log.Info("booked", "locator", res.PNR.RecordLocator,
			"flight", f.Carrier+f.Number, "route", f.From+"-"+f.To,
			"sent", res.Sent)
		locators = append(locators, res.PNR.RecordLocator)
	}

	// A booking is not proven until the carrier answered through the switch.
	// Free-sale bookings are already HK; the ones that sent a request must
	// see the reply cross two links and apply.
	confirmed := 0
	deadline := time.Now().Add(15 * time.Second)
	for _, loc := range locators {
		for time.Now().Before(deadline) {
			rec, err := gds.Store.GetPNR(ctx, loc)
			if err != nil {
				break
			}
			settled := true
			for _, sg := range rec.Segments {
				if sg.Status == "HN" || sg.Status == "NN" {
					settled = false
				}
			}
			if settled {
				confirmed++
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	log.Info("bookings settled", "booked", booked, "confirmed", confirmed)

	// --- the flight day -----------------------------------------------------
	go flyDay(ctx, tenants, flightsByCarrier, *warp, log)

	// --- narrate ------------------------------------------------------------
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped")
			return nil
		case <-tick.C:
			msgs, _ := sw.Store.ListMessages(ctx, store.MessageFilter{Limit: 1})
			depth := 0
			if len(msgs) > 0 {
				depth = 1 // presence only; the console carries the real numbers
			}
			_ = depth
			log.Info("sky", "links", len(sw.LivePeers()),
				"movements_seen", movements.Load(), "booked", booked)
		}
	}
}

func loadManifest(path string) (*world.Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m world.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("manifest %s: %w", path, err)
	}
	sort.Slice(m.Carriers, func(i, j int) bool { return m.Carriers[i].Routes > m.Carriers[j].Routes })
	return &m, nil
}

// buildSwitch assembles the network: a listener per subscriber, relay on.
func buildSwitch(ctx context.Context, m *world.Manifest, console string, log *slog.Logger) (*node.Node, error) {
	cfg := config.Default()
	cfg.Identity = config.Identity{Designator: "1X", TTYAddress: "XCHDD1X", Name: "wholesky switch"}
	cfg.HTTP.Addr = console
	cfg.Store = config.Store{Backend: "mem"}
	cfg.Spool.Enabled = false
	cfg.Demo.Carriers = false
	cfg.Routing.Relay = true
	cfg.Ingress = nil
	cfg.Peers = nil

	add := func(name, designator, tty string) {
		cfg.Ingress = append(cfg.Ingress, config.Ingress{
			Name: name, Type: "tcp", Addr: "127.0.0.1:0",
			Identify: config.Identify{Peer: designator},
		})
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: designator, Carrier: designator, TTYAddress: tty,
			Format: "typeb",
			Egress: config.Egress{Type: "tcp_accept"},
		})
	}
	for _, c := range m.Carriers {
		add("link-"+strings.ToLower(c.Designator), c.Designator, c.TTYAddress)
	}
	add("link-gds", gdsDesignator, gdsAddress)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("switch config: %w", err)
	}
	return node.Build(ctx, cfg, log.With("node", "switch"), node.Options{
		LocatorSecret: []byte("wholesky-switch"),
	})
}

// buildGDS assembles the distribution side: one gateway, one link to the
// switch, and a peer entry per carrier so bookings route by carrier code.
// Every carrier peer's traffic rides the same client link -- the in-process
// equivalent of the "via" egress -- because the address line already names
// the true destination and the switch routes on it.
func buildGDS(ctx context.Context, m *world.Manifest, switchAddr string, log *slog.Logger) (*gateway.Gateway, *counter, error) {
	st := store.NewMem()
	bus := gateway.NewBus(256)
	gw := gateway.New(gateway.Identity{
		Designator: gdsDesignator, TTYAddress: gdsAddress, Name: "wholesky gds",
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
	gw.AddPeer(&gateway.Peer{Name: "net", Format: store.FormatTypeB, TTYAddress: "XCHDD1X"})
	for _, c := range m.Carriers {
		gw.AddPeer(&gateway.Peer{
			Name: c.Designator, Carrier: c.Designator,
			Format: store.FormatTypeB, TTYAddress: c.TTYAddress,
		})
	}

	mv := &counter{}
	sub, cancel := bus.Subscribe()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-sub:
				if ev.Type == gateway.EvMovement {
					mv.Add(1)
				}
			}
		}
	}()

	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("gds link ended", "err", err)
		}
	}()
	return gw, mv, nil
}

// flyDay walks the schedule against warped wall time, emitting departures and
// arrivals as they come due. Position in the day starts at process start.
func flyDay(ctx context.Context, tenants map[string]*host.Tenant,
	flights map[string][]world.Flight, warp int, log *slog.Logger) {

	if warp < 1 {
		warp = 1
	}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	start := time.Now()
	prev := 0
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	regs := map[string]int{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		cur := int(time.Since(start).Minutes()*float64(warp)) % (24 * 60)
		if cur < prev {
			prev = 0 // the day wrapped; start again
		}
		for code, t := range tenants {
			for _, f := range flights[code] {
				reg := fmt.Sprintf("SKY%03d", regHash(f)%1000)
				if f.DepMin > prev && f.DepMin <= cur {
					regs[code]++
					if err := t.Depart(ctx, f, day, reg, 0); err != nil {
						log.Debug("departure not sent", "flight", f.Carrier+f.Number, "err", err)
					}
				}
				if f.ArrMin > prev && f.ArrMin <= cur {
					if err := t.Arrive(ctx, f, day, reg, 0); err != nil {
						log.Debug("arrival not sent", "flight", f.Carrier+f.Number, "err", err)
					}
				}
			}
		}
		prev = cur
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

func waitForLinks(ctx context.Context, sw *node.Node, want int, within time.Duration) error {
	deadline := time.Now().Add(within)
	for {
		if len(sw.LivePeers()) >= want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("only %d of %d links came up: %v", len(sw.LivePeers()), want, sw.LivePeers())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

type counter struct{ n int64 }

func (c *counter) Add(d int64) { c.n += d }
func (c *counter) Load() int64 { return c.n }
