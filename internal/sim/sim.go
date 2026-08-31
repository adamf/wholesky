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
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamf/jetway/pkg/api"
	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/config"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/node"
	"github.com/adamf/jetway/pkg/queue"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"

	"github.com/adamf/jetway/pkg/mvt"

	"github.com/adamf/wholesky/internal/eye"
	"github.com/adamf/wholesky/internal/fleet"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/stats"
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
	// AVSInterval is how often tenants rebroadcast availability; zero uses
	// the host default.
	AVSInterval time.Duration
	// MaxMessages and MaxRecords bound every node's in-memory store. Zero is
	// unbounded, which is right for a test that lives seconds and wrong for a
	// deployment that lives weeks: an unbounded store under continuous demand
	// and a looping flight day is a slow out-of-memory with extra steps.
	MaxMessages int
	MaxRecords  int
	// Warp is sim minutes per wall minute, used by the flight day and by the
	// Eye's aircraft animation. Zero means 1.
	Warp int
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

	maxMessages, maxRecords int
	warp                    int

	closedMu sync.Mutex
	closed   map[string]bool

	flightsByOrigin map[string][]world.Flight

	demMu sync.Mutex
	// DemCancelledLocs lists the locators demand cancelled, so a test can
	// hold the counter to account against the store.
	DemCancelledLocs []string

	// Demand's ledger, for the log line and the tests.
	DemBooked, DemFailed, DemInterline          atomic.Int64
	DemTicketed, DemCancelled, DemSplit, DemNDC atomic.Int64

	// Eye is the world's observer and map, mounted on the switch console.
	Eye *eye.Eye
	// Fleet is the cluster dashboard: every node's live ledger.
	Fleet *fleet.Collector
	// Stats is the instrument panel: the cluster as time series.
	Stats *stats.Collector

	// consoles holds one full Jetway console per embedded node, mounted at
	// /node/{code}/. The switch's own console stays at the root; these give
	// every tenant and the GDS the same face -- because each of them IS a
	// Jetway, and a system you cannot look inside is a diagram, not a system.
	consoles   map[string]http.Handler
	consolesMu sync.Mutex

	airports map[string]world.Airport

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
	byOrigin := map[string][]world.Flight{}
	for _, f := range m.Flights {
		if kept[f.Carrier] {
			flights[f.Carrier] = append(flights[f.Carrier], f)
			byOrigin[f.From] = append(byOrigin[f.From], f)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	warp := opts.Warp
	if warp < 1 {
		warp = 1
	}
	s := &Sim{
		Manifest: m, Tenants: map[string]*host.Tenant{}, Flights: flights,
		flightsByOrigin: byOrigin,
		BookingDate:     time.Now().UTC().AddDate(0, 0, 30),
		maxMessages:     opts.MaxMessages, maxRecords: opts.MaxRecords,
		warp: warp, Eye: eye.New(m, warp),
		closed:   map[string]bool{},
		airports: map[string]world.Airport{},
		consoles: map[string]http.Handler{},
		log:      log, cancel: cancel,
	}
	for _, a := range m.Airports {
		s.airports[a.IATA] = a
	}
	s.Eye.Chaos = s.chaos
	s.Fleet = fleet.New()
	s.Stats = stats.New()

	sw, err := buildSwitch(ctx, m, opts, func(mux *http.ServeMux) {
		s.Eye.Routes(mux)
		s.Fleet.Routes(mux)
		s.Stats.Routes(mux)
		mux.HandleFunc("/node/", s.serveNodeConsole)
	}, log)
	if err != nil {
		cancel()
		return nil, err
	}
	s.Switch = sw
	if err := sw.Start(ctx); err != nil {
		s.Stop()
		return nil, err
	}
	// The Eye watches the switch's message stream: every message in the world
	// crosses it, so its bus is the fabric firehose.
	go s.tapBus(ctx, sw.Bus, func(ev gateway.Event) {
		if ev.Type == gateway.EvMessage {
			if p, ok := ev.Data.(map[string]any); ok {
				s.Eye.OnMessage(p)
				s.Stats.OnMessage(p)
			}
		}
	})
	s.Fleet.Add(ctx, "1X", "the switch", fleet.KindSwitch, "typeb", "", "", 0, sw.Store, sw.Bus)
	s.Fleet.LivePeers = sw.LivePeers
	s.Fleet.LinkControl = s.LinkControl
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
	partners := partnerAddresses(m.Carriers, flights)
	for _, c := range m.Carriers {
		tenantBus := gateway.NewBus(64)
		t, err := host.Start(ctx, c, flights[c.Designator], host.Options{
			SwitchAddr:       sw.Addr("link-" + strings.ToLower(c.Designator)),
			WatchAddress:     GDSAddress,
			PartnerAddresses: partners[c.Designator],
			Capacity:         capacity,
			BookingDate:      s.BookingDate,
			MaxMessages:      opts.MaxMessages,
			MaxRecords:       opts.MaxRecords,
			AVSInterval:      opts.AVSInterval,
			Bus:              tenantBus,
			Log:              log,
		})
		if err != nil {
			s.Stop()
			return nil, fmt.Errorf("start carrier %s: %w", c.Designator, err)
		}
		s.Tenants[c.Designator] = t
		s.Fleet.Add(ctx, c.Designator, c.Name, fleet.KindCarrier,
			c.Format, c.Transport, c.Hub, len(flights[c.Designator]), t.Store, tenantBus)
		s.mountConsole(c.Designator, &api.Server{
			Gateway: t.Gateway, Store: t.Store, Bus: tenantBus,
			Log: log.With("console", c.Designator), Console: true,
		})
	}

	gds, gdsStore, gdsBus, err := s.buildGDS(ctx, m, sw.Addr("link-gds"), log)
	if err != nil {
		s.Stop()
		return nil, err
	}
	s.GDS, s.GDSStore = gds, gdsStore
	s.Fleet.Add(ctx, GDSDesignator, "the gds", fleet.KindGDS, "", "", "", 0, gdsStore, nil)
	// The sweeper is what notices silence -- a request nobody answered, a
	// ticketing deadline that passed. The GDS ran without one until the
	// demand model started ticketing, which is when its absence would have
	// become a silent hole rather than a dormant one.
	sweeper := &queue.Sweeper{
		Records: gdsStore, Queues: gds.Queues, Log: log.With("node", "gds"),
		Cancel: gds,
	}
	go sweeper.Run(ctx, 30*time.Second)
	s.mountConsole(GDSDesignator, &api.Server{
		Gateway: gds, Store: gdsStore, Bus: gdsBus,
		Log: log.With("console", GDSDesignator), Console: true,
	})

	wait := opts.LinkWait
	if wait <= 0 {
		wait = 30 * time.Second
	}
	if err := s.waitForLinks(ctx, len(m.Carriers)+1, wait); err != nil {
		s.Stop()
		return nil, err
	}

	s.Stats.Airborne = s.Eye.Airborne
	s.Stats.LinksUp = func() int { return len(s.Switch.LivePeers()) }
	s.Stats.QueueDepths = func() map[string]int {
		out := map[string]int{}
		items, err := s.GDSStore.ListQueue(context.Background(), store.QueueFilter{})
		if err != nil {
			return out
		}
		for _, it := range items {
			out[it.Queue]++
		}
		return out
	}
	go s.Stats.Run(ctx.Done())
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

// FlyDay walks the schedule and emits departures and arrivals as they come
// due, at warp sim-minutes per wall-minute.
//
// The day's position is anchored to wall-clock time, not to process start.
// The deployed demo suspends when nobody is watching, and a suspended
// process's monotonic clock freezes with it -- anchored to process start, the
// world spent its life stuck before dawn, thawing for a few seconds at a time
// and never reaching the first departure bank. Anchored to the wall clock,
// a thaw jumps the day to where it should be; the catch-up is clamped so the
// first tick replays the recent past rather than a whole frozen week.
func (s *Sim) FlyDay(ctx context.Context) {
	warp := s.warp
	const dayMin = 24 * 60
	// The largest backlog one tick will replay after a pause.
	const maxCatchUp = 120

	pos := func(at time.Time) int {
		return int(float64(at.Unix())/60*float64(warp)) % dayMin
	}
	day := time.Now().UTC().Truncate(24 * time.Hour)
	prev := pos(time.Now()) // no replay of history on boot
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		cur := pos(time.Now())
		if cur < prev {
			prev = 0 // the day wrapped; begin again
		}
		if cur-prev > maxCatchUp {
			prev = cur - maxCatchUp
		}
		for code, t := range s.Tenants {
			for _, f := range s.Flights[code] {
				if s.isClosed(f.From, f.To) {
					continue
				}
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

func buildSwitch(ctx context.Context, m *world.Manifest, opts Options, extend func(*http.ServeMux), log *slog.Logger) (*node.Node, error) {
	console := opts.Console
	cfg := config.Default()
	cfg.Identity = config.Identity{Designator: "1X", TTYAddress: "XCHDD1X", Name: "wholesky switch"}
	cfg.HTTP.Addr = console
	if console == "" {
		cfg.HTTP.Addr = "127.0.0.1:0"
	}
	// The switch sees every message in the world, so it hits any cap first.
	cfg.Store = config.Store{Backend: "mem",
		MaxMessages: opts.MaxMessages, MaxRecords: opts.MaxRecords}
	cfg.Spool.Enabled = false
	cfg.Demo.Carriers = false
	cfg.Routing.Relay = true
	cfg.Ingress = nil
	cfg.Peers = nil

	add := func(name, designator, tty, format, transport string) {
		ingressType := "tcp"
		if transport == "matip" {
			ingressType = "matip"
		}
		cfg.Ingress = append(cfg.Ingress, config.Ingress{
			Name: name, Type: ingressType, Addr: "127.0.0.1:0",
			Identify: config.Identify{Peer: designator},
		})
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: designator, Carrier: designator, TTYAddress: tty,
			Format: format,
			Egress: config.Egress{Type: "tcp_accept"},
		})
	}
	for _, c := range m.Carriers {
		add("link-"+strings.ToLower(c.Designator), c.Designator, c.TTYAddress, c.Format, c.Transport)
	}
	add("link-gds", GDSDesignator, GDSAddress, "typeb", "tcp")

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("switch config: %w", err)
	}
	return node.Build(ctx, cfg, log.With("node", "switch"), node.Options{
		LocatorSecret: []byte("wholesky-switch"),
		SkipConsole:   console == "",
		ExtendAPI:     extend,
	})
}

// buildGDS assembles the distribution side: one gateway, one link to the
// switch, and a peer entry per carrier in that carrier's own dialect --
// AIRIMP over Type B or PADIS over EDIFACT, routed either way by the switch.
func (s *Sim) buildGDS(ctx context.Context, m *world.Manifest, switchAddr string, log *slog.Logger) (*gateway.Gateway, store.Store, *gateway.Bus, error) {
	st := store.NewMem()
	st.MaxMessages, st.MaxRecords = s.maxMessages, s.maxRecords
	bus := gateway.NewBus(256)
	gw := gateway.New(gateway.Identity{
		Designator: GDSDesignator, TTYAddress: GDSAddress, Name: "wholesky gds",
	}, st, bus, log.With("node", "gds"), []byte("wholesky-gds"))
	gw.Avail = avail.NewCache()
	// Without a queue manager a schedule change has nowhere to put the
	// bookings it touches -- applySchedule quietly does nothing. The halos on
	// the map are these placements.
	gw.Queues = &queue.Manager{Store: st, Log: log.With("node", "gds"),
		Notify: func(item *store.QueueItem) { bus.Publish(gateway.EvQueue, item) }}

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

	go s.tapBus(ctx, bus, func(ev gateway.Event) {
		switch ev.Type {
		case gateway.EvMessage:
			// The GDS row was registered without a bus, because the bus is
			// born here; feed it by hand.
			if p, ok := ev.Data.(map[string]any); ok {
				s.Fleet.Count(GDSDesignator, p)
			}
		case gateway.EvMovement:
			s.Movements.Add(1)
			s.Stats.OnMovement()
			if p, ok := ev.Data.(map[string]any); ok {
				s.Eye.OnMovement(p)
			}
		case gateway.EvPNR:
			s.Eye.OnBooking()
			s.Stats.OnBooking()
		case gateway.EvQueue:
			if qi, ok := ev.Data.(*store.QueueItem); ok {
				s.Eye.OnQueue(string(qi.Queue), qi.Reason)
			}
		}
	})
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("gds link ended", "err", err)
		}
	}()
	return gw, st, bus, nil
}

// chaos is the Eye's control surface: close an airport and the world reacts
// through its own machinery, or reopen it and the day resumes.
//
// Closing does two things. The flight day stops departing and arriving
// anything that touches the airport. And every operating carrier sends a real
// ASM cancellation for each affected flight -- one message per flight, paced,
// exactly as a schedule bureau would -- which the GDS ingests through the
// same applySchedule path production traffic uses, placing a queue item for
// every booking it holds on those flights. The halos on the map are those
// queue items arriving; nothing is drawn that did not happen.
func (s *Sim) chaos(action, iata string) error {
	if _, ok := s.Eye.Airport(iata); !ok {
		return fmt.Errorf("no airport %s in this world", iata)
	}
	switch action {
	case "close":
		s.closedMu.Lock()
		already := s.closed[iata]
		s.closed[iata] = true
		s.closedMu.Unlock()
		if already {
			return nil
		}
		s.log.Info("chaos: airport closed", "airport", iata)
		go s.divertAirborneTo(iata)
		go s.cancelFlightsTouching(iata)
		return nil
	case "reopen":
		s.closedMu.Lock()
		delete(s.closed, iata)
		s.closedMu.Unlock()
		s.log.Info("chaos: airport reopened", "airport", iata)
		s.Eye.ClearHalo(iata)
		return nil
	}
	return fmt.Errorf("unknown chaos action %q", action)
}

// LinkControl severs or restores a carrier's circuit to the switch: the
// fleet page's per-node chaos. A severed carrier keeps flying and keeps
// trying to talk -- its sends fail, the switch's copies to it go
// undeliverable, and the gap is visible on every instrument. Restore dials
// back in through the same reconnect path a real circuit repair uses.
func (s *Sim) LinkControl(code, action string) error {
	t := s.Tenants[strings.ToUpper(strings.TrimSpace(code))]
	if t == nil {
		return fmt.Errorf("no carrier %s in this world", code)
	}
	switch action {
	case "sever":
		s.log.Info("chaos: link severed", "carrier", t.Carrier.Designator)
		t.Sever()
		return nil
	case "restore":
		s.log.Info("chaos: link restored", "carrier", t.Carrier.Designator)
		t.Restore()
		return nil
	}
	return fmt.Errorf("unknown link action %q", action)
}

func (s *Sim) isClosed(a, b string) bool {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	return s.closed[a] || s.closed[b]
}

// divertAirborneTo turns the aircraft already in the air toward a closed
// airport away from it, each with a real DIV message from its operating
// carrier: the originally intended airport on the identification line, the
// alternate and new estimate in the EA element, and the AHM reason code for
// a closed field. The Eye reroutes each aircraft when the message reaches
// the watcher -- through the wire, like everything else it knows.
func (s *Sim) divertAirborneTo(iata string) {
	planes := s.Eye.PlanesTo(iata)
	if len(planes) == 0 {
		return
	}
	now := time.Now().UTC()
	for _, p := range planes {
		code := p.Flight[:2]
		t := s.Tenants[code]
		if t == nil {
			continue
		}
		alt := s.nearestOpenAirport(iata)
		if alt == "" {
			continue
		}
		num := p.Flight[2:]
		m := &mvt.Message{
			Kind:         mvt.KindDIV,
			Flight:       p.Flight,
			Day:          fmt.Sprintf("%02d", now.Day()),
			Registration: p.Reg,
			Station:      iata,
			EA:           &mvt.ETA{Time: now.Add(30 * time.Minute).Format("1504"), Airport: alt},
			// 71 is the diversion reason the published examples use for a
			// field below limits; close enough for a closed one.
			DR: "71",
		}
		if err := t.SendOps(context.Background(), m); err != nil {
			s.log.Debug("diversion not sent", "flight", p.Flight, "err", err)
		}
		_ = num
		time.Sleep(20 * time.Millisecond)
	}
	s.log.Info("chaos: airborne diverted", "airport", iata, "aircraft", len(planes))
}

// nearestOpenAirport picks the alternate: closest open field to the closed
// one, by great circle.
func (s *Sim) nearestOpenAirport(iata string) string {
	from, ok := s.airports[iata]
	if !ok {
		return ""
	}
	best, bestKM := "", 1e18
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	for code, a := range s.airports {
		if code == iata || s.closed[code] {
			continue
		}
		km := haversine(from.Lat, from.Lon, a.Lat, a.Lon)
		if km < bestKM {
			best, bestKM = code, km
		}
	}
	return best
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

// cancelFlightsTouching sends one ASM CNL per affected flight, paced so the
// cascade is a stream rather than a detonation.
func (s *Sim) cancelFlightsTouching(iata string) {
	date := strings.ToUpper(s.BookingDate.Format("02Jan"))
	n := 0
	for code, fs := range s.Flights {
		t := s.Tenants[code]
		if t == nil {
			continue
		}
		for _, f := range fs {
			if f.From != iata && f.To != iata {
				continue
			}
			text := fmt.Sprintf("ASM\nUTC\nCNL\n%s%s/%s\n%s %s",
				f.Carrier, f.Number, date, f.From, f.To)
			if err := t.SendSchedule(context.Background(), text); err != nil {
				s.log.Debug("cancellation not sent", "flight", f.Carrier+f.Number, "err", err)
			}
			n++
			time.Sleep(8 * time.Millisecond)
			s.closedMu.Lock()
			still := s.closed[iata]
			s.closedMu.Unlock()
			if !still {
				return // reopened mid-cascade; stop cancelling
			}
		}
	}
	s.log.Info("chaos: cascade complete", "airport", iata, "flights_cancelled", n)
}

// mountConsole registers one node's console handler.
func (s *Sim) mountConsole(code string, srv *api.Server) {
	h := srv.Handler()
	s.consolesMu.Lock()
	s.consoles[strings.ToUpper(code)] = h
	s.consolesMu.Unlock()
}

// serveNodeConsole dispatches /node/{code}/... to that node's own console,
// stripping the prefix so the console's relative paths resolve beneath it.
func (s *Sim) serveNodeConsole(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/node/")
	code, sub, ok := strings.Cut(rest, "/")
	if !ok {
		// /node/FR -> /node/FR/ so the page's relative fetches resolve.
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}
	s.consolesMu.Lock()
	h := s.consoles[strings.ToUpper(code)]
	s.consolesMu.Unlock()
	if h == nil {
		http.Error(w, "no such node", http.StatusNotFound)
		return
	}
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/" + sub
	h.ServeHTTP(w, r2)
}

// partnerAddresses derives each carrier's interline partners: the carriers
// it shares the most airports with, capped small the way a real partner list
// is. Deterministic -- same manifest, same partners -- because the web the
// map draws should be a property of the world, not of a map iteration.
func partnerAddresses(carriers []world.Carrier, flights map[string][]world.Flight) map[string][]string {
	const maxPartners = 4
	touch := map[string]map[string]bool{}
	for code, fs := range flights {
		t := map[string]bool{}
		for _, f := range fs {
			t[f.From], t[f.To] = true, true
		}
		touch[code] = t
	}
	tty := map[string]string{}
	for _, c := range carriers {
		tty[c.Designator] = c.TTYAddress
	}
	out := map[string][]string{}
	for _, c := range carriers {
		type cand struct {
			code  string
			score int
		}
		var cs []cand
		for _, o := range carriers {
			if o.Designator == c.Designator {
				continue
			}
			n := 0
			for apt := range touch[c.Designator] {
				if touch[o.Designator][apt] {
					n++
				}
			}
			if n >= 2 {
				cs = append(cs, cand{o.Designator, n})
			}
		}
		sort.Slice(cs, func(i, j int) bool {
			if cs[i].score != cs[j].score {
				return cs[i].score > cs[j].score
			}
			return cs[i].code < cs[j].code
		})
		for i := 0; i < len(cs) && i < maxPartners; i++ {
			out[c.Designator] = append(out[c.Designator], tty[cs[i].code])
		}
	}
	return out
}

// tapBus runs fn for every event on a bus until the context ends.
func (s *Sim) tapBus(ctx context.Context, bus *gateway.Bus, fn func(gateway.Event)) {
	sub, unsub := bus.Subscribe()
	defer unsub()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-sub:
			fn(ev)
		}
	}
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
