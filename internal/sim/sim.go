// Package sim boots and runs a world.
//
// There is exactly one wiring: skyd runs it as a daemon, the tests run it in
// a process and assert on it. A test standing on its own copy of the boot
// would be testing the copy -- the lesson Jetway's scenario suite already
// paid for.
package sim

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamf/jetway/pkg/acars"
	"github.com/adamf/jetway/pkg/api"
	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/config"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/node"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/queue"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"

	"github.com/adamf/jetway/pkg/irops"
	"github.com/adamf/jetway/pkg/mvt"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/eye"
	"github.com/adamf/wholesky/internal/fill"
	"github.com/adamf/wholesky/internal/fleet"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/interline"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/settle"
	"github.com/adamf/wholesky/internal/stats"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
	"gopkg.in/yaml.v3"
)

// The GDS identity for phase one. There is exactly one so far; the design
// calls for five.
const (
	GDSDesignator = "1G"
	GDSAddress    = "LONDD1G"
)

// gdsSlots are the distribution systems the world can run, in boot order.
// The designators are the two-character codes the real distribution world
// uses on records; the cities anchor each system's teletype address. The
// first slot doubles as the movement watcher: operational traffic is copied
// to its address, the way a single OCC display would subscribe once.
var gdsSlots = []struct{ Designator, City, Name string }{
	{"1G", "LON", "gds golf"},
	{"1S", "DFW", "gds sierra"},
	{"1A", "NCE", "gds alpha"},
	{"1V", "DEN", "gds victor"},
	{"1P", "ATL", "gds papa"},
}

func gdsAddress(slot struct{ Designator, City, Name string }) string {
	return slot.City + "DD" + slot.Designator
}

// GDSNode is one running distribution system.
type GDSNode struct {
	Designator string
	Address    string
	Name       string
	GW         *gateway.Gateway
	Store      store.Store
	Bus        *gateway.Bus

	up atomic.Bool
}

// Options configure a boot.
type Options struct {
	// External names carriers this world does not run itself: their
	// addresses stay in the switch's routing table, their flights in the
	// schedule, but no tenant is booted -- a player's own jetway node
	// dials the switch as the carrier, with the token the start pack
	// (/carrier/XX/pack) gives it. PublicSwitch is the address that node
	// dials, when it is not the listener's own; LinkSecret keys the tokens
	// (random per boot when empty).
	External     []string
	PublicSwitch string
	LinkSecret   string
	// DecisionWindow is how long a seat has to answer a decision before the
	// autopilot's default, in real time; zero is forty-five seconds.
	DecisionWindow time.Duration
	// Carriers caps how many carriers run, largest first. Zero runs all.
	Carriers int
	// SettleEvery is how often the settlement plan re-runs over the books
	// this machine holds; zero is every five minutes.
	SettleEvery time.Duration
	// Switches is how many message switches the fabric runs, one or two:
	// with two, every carrier is homed on one by hash and the switches are
	// joined by a trunk, so a message between carriers on different
	// switches crosses it the way the real network's inter-switch trunks
	// carry traffic between SITA and ARINC. Zero and one are one switch.
	Switches int
	// Console is the switch console address; empty skips the console.
	Console string
	// Capacity, when positive, overrides every cabin's seats at every
	// tenant. Zero means the aircraft's own cabins: a 737 has 174 seats
	// whatever the fare letter, and a full one waitlists, then refuses.
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
	// TenantMaxMessages and TenantMaxRecords bound each carrier's store,
	// separately from the hub caps, because tenants are the multiplier: five
	// hundred ledgers at the hub cap is gigabytes of retained wire bytes --
	// the deployed demo learned this from the OOM killer. Zero inherits the
	// hub caps.
	TenantMaxMessages int
	TenantMaxRecords  int
	// Warp is sim minutes per wall minute, used by the flight day and by the
	// Eye's aircraft animation. Zero means 1.
	Warp int
	// GDSList names the distribution systems that exist in this
	// deployment, for a region machine whose own boot runs none of them.
	// Empty means all five.
	GDSList []string
	// NoGDS runs no distribution system at all: a core machine, whose
	// GDSes are peers on other machines.
	NoGDS bool
	// GDSCount is how many distribution systems run, in gdsSlots order.
	// Zero runs all five; the design calls for five because the real world
	// has several, and inter-GDS behaviour -- the same flight sold through
	// two channels, a schedule change fanning out to every subscriber -- only
	// exists when there is more than one.
	GDSCount int
	// IROPSInterval is how often each distribution system works its
	// schedule-change queue. Zero uses the engine's default.
	IROPSInterval time.Duration
	// SellDays is how many dates demand books for, starting the day before
	// the flown one: 1 sells only the day the world flies; 4 is the older
	// window of -1..+2. Zero means 1.
	SellDays int
	// Fill, when positive, is the load factor the day's flights already
	// carry when the day starts: the bookings sold in the weeks before,
	// written into each carrier's book of record by internal/fill before
	// the flight day runs, and again after each end-of-day purge. Meant
	// for a world whose tenants are on Postgres; in memory it is a lot of
	// records. FillSeed makes the fill reproducible.
	Fill     float64
	FillSeed int64
	// Refill purges and refills the day at boot even when the books already
	// hold it: the way to change the load factor on a running world.
	Refill bool
	// TenantDSN, when set, backs every carrier tenant's records with one
	// shared Postgres: each tenant is a node view of it, and its message
	// log stays in bounded memory. Records are purged when the simulated
	// day wraps, so the database is one day deep. A value starting with
	// "$" names an environment variable holding the DSN.
	TenantDSN string
	// GDSDSN, when set, backs every distribution system's records with
	// Postgres, each as its own node view of one database (the message log
	// stays in bounded memory, like the tenants'). Purged with the tenants
	// when the day wraps. A value starting with "$" names an environment
	// variable holding the DSN.
	GDSDSN string
	// LinkBind is the host the switch's subscriber listeners bind on. Empty
	// binds loopback, which is the single-box world; a core machine binds
	// "::" so subscribers on other machines can dial in over the private
	// network.
	LinkBind string
	// StatsSnapshot, when set, persists the stats rings to this path every
	// thirty seconds and restores them on boot, so a redeploy or a thawed
	// machine does not open on blank charts.
	StatsSnapshot string
	// LinkWait bounds how long Boot waits for the fabric.
	LinkWait time.Duration
	Log      *slog.Logger
}

// Sim is a running world.
type Sim struct {
	// tenantDB is the shared Postgres behind every tenant's records, when
	// the deployment has one; dayStarted is the wall moment the current
	// simulated day began, which is what the end-of-day purge cuts at.
	tenantDB *store.Postgres
	gdsDB    *store.Postgres
	// tariff is the world's fare filing, derived from the schedule: what
	// the distribution systems price against and the filler prices with.
	tariff *tariff.Synthetic
	// Ledger is what the day's tickets were sold for, by leg: fed where the
	// price is known, read by the globe for the money in the air.
	Ledger *revenue.Ledger
	// marketedBy resolves a marketing carrier's number and boarding point
	// to the carrier that flies the leg.
	marketedBy map[string]string
	dayMu      sync.Mutex
	dayStarted time.Time
	sellDays   int
	fill       float64
	fillSeed   int64
	refill     bool
	Manifest   *world.Manifest
	// fate is the day's plan for every flight -- delays with their causes,
	// slots, late aircraft, crews -- decided once from the schedule so every
	// machine agrees; see internal/dayplan.
	fate *dayplan.Plan
	// Airline is the seats: who runs which carrier, their decisions and
	// their tape; announced remembers which cancellations went out.
	Airline      *airline.Registry
	airlineSrv   *airline.Server
	scoreMu      sync.Mutex
	scoreCache   map[string]airline.Scorecard
	scoreAt      time.Time
	external     map[string]bool
	linkSecret   string
	publicSwitch string
	announced    sync.Map
	Switch       *node.Node
	// Switches is every switch in the fabric, Switch being the first: the
	// one with the console, the instruments and the distribution systems.
	Switches []*node.Node
	// plan is the settlement plan and settlement its latest day: the HOT
	// each airline was handed, and how it reconciled.
	plan       settle.Plan
	settleMu   sync.Mutex
	settlement *settle.Summary
	billing    *interline.Summary
	// GDSes are the running distribution systems; GDS and GDSStore alias the
	// first, which is also the movement watcher.
	GDSes       []*GDSNode
	GDS         *gateway.Gateway
	GDSStore    store.Store
	Tenants     map[string]*host.Tenant
	capacity    int // the cabin override every carrier was built with
	Flights     map[string][]world.Flight
	BookingDate time.Time
	// Movements counts EvMovement events seen at the GDS: flights whose
	// departures and arrivals crossed the switch and were recognised.
	Movements atomic.Int64
	// Departures counts the departures the flight day issued from this
	// machine, and reportErrs the aircraft reports that failed to leave;
	// the run log carries both so a silent sky can be told from a quiet one.
	Departures atomic.Int64
	reportErrs atomic.Int64
	// Rebooked counts passengers the irops engines moved off cancelled
	// flights onto seats that were open.
	Rebooked atomic.Int64
	// Exchanged counts the involuntary reissues that followed a reprotection.
	Exchanged atomic.Int64
	irops     []*irops.Engine
	// DSP and ANSP are this machine's datalink provider and air navigation
	// service provider: the networks beside the airlines' own.
	DSP      *Datalink
	ANSP     *ANSP
	Border   *Border
	carriers map[string]world.Carrier
	// Each GDSNode's up flag flips when its client link is established. The
	// switch counting a session is not enough: there is a window where the
	// listener has accepted the connection but the client has not yet marked
	// its link usable, and a booking placed in that window sends into a link
	// that is "not up" -- recorded, undeliverable, and with no router behind
	// this sender, never retried. The first live run's two "free sale"
	// bookings were actually this.

	maxMessages, maxRecords int
	clock                   *simClock

	closedMu sync.Mutex
	closed   map[string]bool

	flightsByOrigin map[string][]world.Flight

	demMu sync.Mutex
	// DemCancelledLocs lists the locators demand cancelled, so a test can
	// hold the counter to account against the store.
	DemCancelledLocs []string

	// Demand's ledger, for the log line and the tests.
	DemBooked, DemFailed, DemInterline                       atomic.Int64
	DemTicketed, DemCancelled, DemRefunded, DemSplit, DemNDC atomic.Int64

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

	fedMu      sync.RWMutex
	fedHandler http.Handler
	invHandler http.HandlerFunc
	// ConsoleProxy, when set, forwards a /node/{code}/ request to the
	// machine that owns the node. It reports whether it handled it.
	ConsoleProxy func(w http.ResponseWriter, r *http.Request, code string) bool

	log    *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
}

// Boot stands the topology up and waits for every link.
// bootBase stands up what every world shape shares: the Sim scaffolding,
// the clock, the collectors, and the switch with its console. withSwitch is
// false only for machines that dial someone else's switch.
func bootBase(ctx context.Context, m *world.Manifest, opts Options, withSwitch bool) (*Sim, error) {
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
	// GDSCount zero means the single-box default of all five; NoGDS means
	// none at all, which is a core machine whose distribution systems live
	// on machines of their own.
	gdsCount := opts.GDSCount
	if gdsCount <= 0 || gdsCount > len(gdsSlots) {
		gdsCount = len(gdsSlots)
	}
	if opts.NoGDS {
		gdsCount = 0
	}
	var gdses []*GDSNode
	for _, slot := range gdsSlots[:gdsCount] {
		gdses = append(gdses, &GDSNode{
			Designator: slot.Designator, Address: gdsAddress(slot), Name: slot.Name,
		})
	}

	s := &Sim{
		Manifest: m, GDSes: gdses, Tenants: map[string]*host.Tenant{}, Flights: flights,
		dayStarted:      time.Now(),
		tariff:          tariff.FromManifest(m),
		Ledger:          revenue.New(),
		marketedBy:      marketedIndex(m),
		fill:            opts.Fill,
		fillSeed:        opts.FillSeed,
		refill:          opts.Refill,
		flightsByOrigin: byOrigin,
		BookingDate:     sellingDate(m),
		maxMessages:     opts.MaxMessages, maxRecords: opts.MaxRecords,
		clock: newSimClock(warp), Eye: eye.New(m, warp),
		closed:   map[string]bool{},
		airports: map[string]world.Airport{},
		consoles: map[string]http.Handler{},
		log:      log, ctx: ctx, cancel: cancel,
	}
	for _, a := range m.Airports {
		s.airports[a.IATA] = a
	}
	s.carriers = map[string]world.Carrier{}
	for _, c := range m.Carriers {
		s.carriers[c.Designator] = c
	}
	s.Eye.Chaos = s.chaos
	s.Eye.FlightPNRs = s.flightRecords
	s.Eye.FlightDCS = s.flightDCS
	s.Ledger.Resolve = func(carrier, number, board string) (string, bool) {
		op, ok := s.marketedBy[revenue.Key(carrier, number, board)]
		return op, ok
	}
	s.Eye.Aloft = s.Ledger.Sum
	// Bound late: the stats collector is built after the Eye's hooks are.
	s.Eye.Sold = func() int64 {
		if s.Stats == nil {
			return 0
		}
		return s.Stats.RevenueTotal()
	}
	s.Eye.Settled = func() int64 {
		if sum := s.Settlement(); sum != nil {
			return sum.Gross
		}
		return 0
	}
	s.Eye.WarpNow = s.clock.Warp
	s.fate = dayplan.Build(m, sellingDate(m), delayFor)
	s.Airline = airline.New(opts.DecisionWindow)
	s.airlineSrv = &airline.Server{Reg: s.Airline, World: seatWorld{s}, Local: s.runsCarrier, Proxy: s.proxyCarrier}
	s.external = map[string]bool{}
	for _, d := range opts.External {
		if d = strings.ToUpper(strings.TrimSpace(d)); d != "" {
			s.external[d] = true
		}
	}
	s.linkSecret, s.publicSwitch = opts.LinkSecret, opts.PublicSwitch
	if s.linkSecret == "" {
		b := make([]byte, 16)
		rand.Read(b) //nolint:errcheck
		s.linkSecret = hex.EncodeToString(b)
	}
	s.Eye.Weather = func() ([]dayplan.Cell, []dayplan.Regulation, dayplan.Summary) {
		return s.fate.Weather, s.fate.Regulations, s.fate.Summary
	}
	s.Eye.SimPos = func() float64 { return s.clock.Pos(time.Now()) }
	s.Eye.SetWarp = s.SetWarp
	for _, g := range gdses {
		s.Eye.Hubs = append(s.Eye.Hubs, g.Designator)
	}
	s.Fleet = fleet.New()
	s.Stats = stats.New()
	s.Stats.SetSnapshotPath(opts.StatsSnapshot)

	if !withSwitch {
		return s, nil
	}
	nSwitches := switchCount(opts)
	sw, err := buildSwitch(ctx, m, opts, 0, nSwitches, "", func(mux *http.ServeMux) {
		s.Eye.Routes(mux)
		s.Fleet.Routes(mux)
		s.Stats.Routes(mux)
		mux.HandleFunc("/node/", s.serveNodeConsole)
		// The federation surface is late-bound: a core installs its registry
		// here after boot; every other shape 404s the path.
		// The invariant check is federated the same way when a core is
		// installed, and answers for this process's carriers otherwise.
		mux.HandleFunc("GET /settlement.json", s.serveSettlement)
		mux.HandleFunc("GET /dayplan.json", s.serveDayPlan)
		mux.HandleFunc("GET /carrier/{carrier}/pack", s.servePack)
		s.airlineSrv.Routes(mux)
		mux.HandleFunc("GET /settlement/", s.serveHOT)
		mux.HandleFunc("GET /billing.json", s.serveBilling)
		mux.HandleFunc("GET /billing/", s.serveInvoice)
		mux.HandleFunc("GET /ret/", s.serveRET)
		mux.HandleFunc("GET /invariants.json", func(w http.ResponseWriter, r *http.Request) {
			s.fedMu.RLock()
			h := s.invHandler
			s.fedMu.RUnlock()
			if h != nil {
				h(w, r)
				return
			}
			s.serveInvariants(w, r)
		})
		mux.HandleFunc("/federation/", func(w http.ResponseWriter, r *http.Request) {
			s.fedMu.RLock()
			h := s.fedHandler
			s.fedMu.RUnlock()
			if h == nil {
				http.NotFound(w, r)
				return
			}
			h.ServeHTTP(w, r)
		})
	}, log)
	if err != nil {
		cancel()
		return nil, err
	}
	s.Switch = sw
	s.Switches = []*node.Node{sw}
	if err := sw.Start(ctx); err != nil {
		s.Stop()
		return nil, err
	}
	for k := 1; k < nSwitches; k++ {
		other, err := buildSwitch(ctx, m, opts, k, nSwitches, sw.Addr("link-net"), nil, log)
		if err != nil {
			s.Stop()
			return nil, err
		}
		if err := other.Start(ctx); err != nil {
			s.Stop()
			return nil, err
		}
		s.Switches = append(s.Switches, other)
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
	return s, nil
}

func Boot(ctx context.Context, m *world.Manifest, opts Options) (*Sim, error) {
	s, err := bootBase(ctx, m, opts, true)
	if err != nil {
		return nil, err
	}
	log := s.log
	ctx = s.ctx
	sw := s.Switch
	gdses := s.GDSes
	flights := s.Flights

	capacity := opts.Capacity
	s.capacity = capacity
	partners := partnerAddresses(m.Carriers, flights)
	flightsByFrom := indexByFrom(flights)
	marketed, operators := codeshares(m.Carriers, flights)
	var distribution []string
	for _, g := range gdses {
		distribution = append(distribution, g.Address)
	}
	tenantMsgs := opts.TenantMaxMessages
	if tenantMsgs == 0 {
		tenantMsgs = opts.MaxMessages
	}
	tenantStore, err := s.tenantStores(ctx, opts, tenantMsgs)
	if err != nil {
		s.Stop()
		return nil, err
	}
	if err := s.startNetworks(ctx, 0, sw.Addr("link-net"), log); err != nil {
		s.Stop()
		return nil, err
	}
	for _, c := range m.Carriers {
		if s.external[c.Designator] {
			continue // run by someone's own jetway node; see Options.External
		}
		tenantBus := gateway.NewBus(64)
		s.tapTenantRevenue(ctx, tenantBus)
		switchAddr := s.switchAddrFor(c)
		tenantMaxMsgs, tenantMaxRecs := opts.TenantMaxMessages, opts.TenantMaxRecords
		if tenantMaxMsgs == 0 {
			tenantMaxMsgs = opts.MaxMessages
		}
		if tenantMaxRecs == 0 {
			tenantMaxRecs = opts.MaxRecords
		}
		t, err := host.Start(ctx, c, flights[c.Designator], host.Options{
			SwitchAddr:            switchAddr,
			DayPos:                func() float64 { return s.clock.Pos(time.Now()) },
			Tariff:                s.tariff,
			WatchAddress:          GDSAddress,
			DistributionAddresses: distribution,
			PartnerAddresses:      partners[c.Designator],
			Interline:             interlineFor(c.Designator, partners, m.Carriers, flightsByFrom),
			Marketed:              marketed[c.Designator],
			OperatorAddresses:     operators[c.Designator],
			Border:                govPeer(0),
			CountryOf:             s.countryOf,
			Capacity:              capacity,
			BookingDate:           s.BookingDate,
			MaxMessages:           tenantMaxMsgs,
			MaxRecords:            tenantMaxRecs,
			AVSInterval:           opts.AVSInterval,
			InboundDelay:          s.inboundDelay,
			RushDecision:          s.askRush,
			ICAO:                  s.icaoOf,
			Store:                 tenantStore(c.Designator),
			Bus:                   tenantBus,
			Log:                   log,
		})
		if err != nil {
			s.Stop()
			return nil, fmt.Errorf("start carrier %s: %w", c.Designator, err)
		}
		t.SetDay(s.BookingDate)
		s.Tenants[c.Designator] = t
		s.Fleet.Add(ctx, c.Designator, c.Name, fleet.KindCarrier,
			c.Format, c.Transport, c.Hub, len(flights[c.Designator]), t.Store, tenantBus)
		s.mountConsole(c.Designator, tenantConsole(t, tenantBus, log.With("console", c.Designator)))
	}

	gdsStore, err := s.gdsStores(ctx, opts.GDSDSN, opts.MaxMessages)
	if err != nil {
		s.Stop()
		return nil, err
	}
	for i, g := range s.GDSes {
		if err := s.buildGDSNode(ctx, m, g,
			sw.Addr("link-net"), gdsStore, i == 0, log); err != nil {
			s.Stop()
			return nil, err
		}
		s.Fleet.Add(ctx, g.Designator, g.Name, fleet.KindGDS, "", "", "", 0, g.Store, nil)
		// The sweeper is what notices silence -- a request nobody answered, a
		// ticketing deadline that passed. Each distribution system runs its
		// own, because each holds its own records.
		sweeper := &queue.Sweeper{
			Records: g.Store, Queues: g.GW.Queues,
			Log: log.With("node", strings.ToLower(g.Designator)), Cancel: g.GW,
		}
		go sweeper.Run(ctx, 30*time.Second)
		s.irops = append(s.irops, s.startIROPS(ctx, g, opts.IROPSInterval, log.With("node", strings.ToLower(g.Designator))))
		s.mountConsole(g.Designator, &api.Server{
			Gateway: g.GW, Store: g.Store, Bus: g.Bus,
			Log: log.With("console", g.Designator), Console: true,
		})
	}
	s.GDS, s.GDSStore = s.GDSes[0].GW, s.GDSes[0].Store
	s.Fleet.Add(ctx, s.DSP.Name, "datalink provider", fleet.KindNetwork, "typeb", "", "", 0, s.DSP.Store, s.DSP.Bus)
	s.Fleet.Add(ctx, s.ANSP.Name, "air navigation services", fleet.KindNetwork, "aftn", "", "", 0, s.ANSP.Store, s.ANSP.Bus)

	wait := opts.LinkWait
	if wait <= 0 {
		wait = 30 * time.Second
	}
	if err := s.waitForLinks(ctx, len(m.Carriers)+len(s.GDSes)+2, wait); err != nil {
		s.Stop()
		return nil, err
	}

	s.Stats.Airborne = s.Eye.Airborne
	s.Stats.LinksUp = s.linksUp
	s.Stats.QueueDepths = func() map[string]int {
		out := map[string]int{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, g := range s.GDSes {
			counts, err := g.Store.QueueCounts(ctx)
			if err != nil {
				continue
			}
			for q, n := range counts {
				out[q] += n
			}
		}
		return out
	}
	go s.Stats.Run(ctx.Done())
	s.StartSettling(opts.SettleEvery)
	return s, nil
}

// Stop tears the world down.
func (s *Sim) Stop() {
	s.cancel()
	for _, sw := range s.Switches {
		sw.Close()
	}
	if s.tenantDB != nil {
		s.tenantDB.Close()
	}
	if s.gdsDB != nil {
		s.gdsDB.Close()
	}
}

// gdsStores opens the distribution systems' shared database, when there
// is one, and hands out a node view per system with a bounded in-memory
// message log, the same shape the tenants take.
func (s *Sim) gdsStores(ctx context.Context, dsn string, maxMsgs int) (func(code string) store.Store, error) {
	if strings.HasPrefix(dsn, "$") {
		dsn = os.Getenv(strings.TrimPrefix(dsn, "$"))
	}
	if dsn == "" {
		return func(string) store.Store { return nil }, nil
	}
	pg, err := store.OpenPostgres(ctx, poolDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("gds database: %w", err)
	}
	pg.RetireGrace = time.Second
	if err := store.MigrateSchema(ctx, pg); err != nil {
		pg.Close()
		return nil, fmt.Errorf("gds database: migrate: %w", err)
	}
	s.gdsDB = pg
	return func(code string) store.Store {
		mem := store.NewMem()
		mem.MaxMessages = maxMsgs
		return store.Split{Messages: mem, Records: pg.Node(code)}
	}, nil
}

// poolDSN adds the pool settings a transaction-pooling proxy needs, unless
// the DSN already sets them.
func poolDSN(dsn string) string {
	for _, kv := range []string{"pool_max_conns=24", "default_query_exec_mode=cache_describe"} {
		key := strings.SplitN(kv, "=", 2)[0]
		if strings.Contains(dsn, key) {
			continue
		}
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + kv
	}
	return dsn
}

// tenantStores opens the shared tenant database named by the options and
// returns a factory for one tenant's store: a node view of Postgres for the
// records, bounded memory for the message log. Without a DSN the factory
// returns nil and the tenant keeps everything in memory.
func (s *Sim) tenantStores(ctx context.Context, opts Options, maxMsgs int) (func(code string) store.Store, error) {
	dsn := opts.TenantDSN
	if strings.HasPrefix(dsn, "$") {
		dsn = os.Getenv(strings.TrimPrefix(dsn, "$"))
	}
	if dsn == "" {
		return func(string) store.Store { return nil }, nil
	}
	// One pool serves hundreds of tenants; the default of a handful of
	// connections would serialise them behind each other. And a managed
	// database is usually reached through a transaction-pooling proxy,
	// which cannot carry a cached prepared statement from one transaction
	// to the next, so statement descriptions are cached client-side and no statement is prepared on the server.
	pg, err := store.OpenPostgres(ctx, poolDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("tenant database: %w", err)
	}
	// A simulated day retires the moment it has flown: a record's
	// retirement day is its flight day, not three days on.
	pg.RetireGrace = time.Second
	if err := store.MigrateSchema(ctx, pg); err != nil {
		pg.Close()
		return nil, fmt.Errorf("tenant database: migrate: %w", err)
	}
	s.tenantDB = pg
	return func(code string) store.Store {
		mem := store.NewMem()
		mem.MaxMessages = maxMsgs
		return store.Split{Messages: mem, Records: pg.Node(code)}
	}, nil
}

// purgeTenants discards every tenant's records and messages older than a
// moment: the end of a simulated day, after which what was booked for it
// has flown or not, and the book of record starts the next day clean.
// Sequential and paced, because hundreds of deletes at once on the shared
// database is a stampede on the day's first departures.
func (s *Sim) purgeTenants(ctx context.Context, before time.Time) {
	var msgs, recs, parts int
	codes := make([]string, 0, len(s.Tenants))
	for code := range s.Tenants {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	stores := map[string]store.Store{}
	for _, code := range codes {
		stores[code] = s.Tenants[code].Store
	}
	// The distribution systems' books go with the day too, when they are
	// on the database; in memory they are bounded and forget on their own.
	if s.gdsDB != nil {
		for _, g := range s.GDSes {
			codes = append(codes, g.Designator)
			stores[g.Designator] = g.Store
		}
	}
	// On a database the day's records leave as partitions: one call per
	// database, whatever the cutoff, for the day the world flew. The
	// in-memory message logs are purged node by node as before.
	cutoff := s.BookingDate.UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	for _, db := range []*store.Postgres{s.tenantDB, s.gdsDB} {
		if db == nil {
			continue
		}
		got, err := db.RetireBefore(ctx, cutoff)
		if err != nil {
			s.log.Warn("end-of-day retirement failed", "err", err)
			continue
		}
		parts += got.Partitions
		recs += got.Records
	}
	for _, code := range codes {
		if ctx.Err() != nil {
			return
		}
		st := stores[code]
		if sp, ok := st.(store.Split); ok && (s.tenantDB != nil || s.gdsDB != nil) {
			st = sp.Messages // the records were retired above
		}
		got, err := st.Purge(ctx, before)
		if err != nil {
			s.log.Warn("end-of-day purge failed", "node", code, "err", err)
			continue
		}
		msgs += got.Messages
		recs += got.Records
		time.Sleep(20 * time.Millisecond)
	}
	s.log.Info("end-of-day purge", "before", before.UTC().Format(time.RFC3339), "retired_through", cutoff.Format("2006-01-02"),
		"partitions", parts, "records", recs, "messages", msgs)
}

// Warp is the sim clock's current rate; Pos its position in the day, in
// minutes.
func (s *Sim) Warp() int { return s.clock.Warp() }
func (s *Sim) Pos() int  { return int(s.clock.Pos(time.Now())) }

// RunStats is the run log's line: what this machine's day has done.
func (s *Sim) RunStats() []any {
	out := []any{"movements", s.Movements.Load(),
		"departures", s.Departures.Load(), "report_failures", s.reportErrs.Load(),
		"pos", int(s.clock.Pos(time.Now())), "warp", s.clock.Warp()}
	// Only the machine that runs the switch has links to count; a region or
	// a distribution machine holds none of its own.
	if s.Switch != nil {
		out = append([]any{"links", len(s.Switch.LivePeers())}, out...)
	}
	return out
}

// marketedIndex maps marketing code + number + board to the operating
// carrier, for the legs sold under another carrier's code.
func marketedIndex(m *world.Manifest) map[string]string {
	out := map[string]string{}
	for _, f := range m.Flights {
		if f.Marketing != "" && f.Marketing != f.Carrier && f.MarketingNumber != "" {
			out[revenue.Key(f.Marketing, f.MarketingNumber, f.From)] = f.Carrier
		}
	}
	return out
}

// rebuildLedger counts every book on this machine into the revenue ledger:
// what each leg's priced records paid, from the carriers' books and the
// distribution systems' alike. The live sells add to it from here on.
func (s *Sim) rebuildLedger(ctx context.Context) {
	started := time.Now()
	s.Ledger.Reset()
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	stores := map[string]store.Store{}
	for code, t := range s.Tenants {
		stores[code] = t.Store
	}
	for _, g := range s.GDSes {
		stores[g.Designator] = g.Store
	}
	legs, failed := 0, 0
	for code, st := range stores {
		rows, err := st.RevenueByLeg(ctx, wire)
		if err != nil {
			failed++
			s.log.Warn("ledger rebuild failed", "node", code, "err", err)
			continue
		}
		s.Ledger.Fill(rows)
		legs += len(rows)
	}
	s.log.Info("ledger rebuilt", "books", len(stores), "legs", legs, "total_cents", s.Ledger.Total(), "failed", failed, "took", time.Since(started).Round(time.Millisecond).String())
}

// rebuildInventories counts every tenant's book of record into its seat
// inventory, after a fill or a purge and at boot: what is sold is what the
// store says is sold.
func (s *Sim) rebuildInventories(ctx context.Context) {
	started := time.Now()
	seats, failed := 0, 0
	for code, t := range s.Tenants {
		if ctx.Err() != nil {
			return
		}
		n, err := t.RebuildInventory(ctx)
		if err != nil {
			failed++
			s.log.Warn("inventory rebuild failed", "carrier", code, "err", err)
			continue
		}
		seats += n
	}
	s.log.Info("inventories rebuilt", "carriers", len(s.Tenants), "seats_held", seats, "failed", failed, "took", time.Since(started).Round(time.Millisecond).String())
}

// dayFilled reports whether the books already hold the flown day: the
// filler's records for it exist at the carrier with the most flights. A
// day interrupted mid-fill counts as filled and is finished by hand with a
// purge; that is rarer than a redeploy.
func (s *Sim) dayFilled(ctx context.Context) bool {
	var biggest *host.Tenant
	most := 0
	for code, t := range s.Tenants {
		if n := len(s.Flights[code]); n > most {
			biggest, most = t, n
		}
	}
	if biggest == nil {
		return false
	}
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	rows, err := biggest.Store.SoldSeats(ctx, biggest.Carrier.Designator, wire)
	if err != nil {
		return false
	}
	seats := 0
	for _, r := range rows {
		seats += r.Seats
	}
	// More seats than the day's own selling could have put there by now.
	return seats > most*10
}

// fillDay writes the day's pre-sold bookings into this machine's tenants'
// books of record: the manifest's flights for the carriers run here, at
// the configured load factor, deterministically from the seed. See
// internal/fill for what a filled record is.
func (s *Sim) fillDay(ctx context.Context) {
	if s.fill <= 0 || len(s.Tenants) == 0 {
		return
	}
	sub := &world.Manifest{}
	for code, fs := range s.Flights {
		if _, ok := s.Tenants[code]; ok {
			sub.Flights = append(sub.Flights, fs...)
		}
	}
	started := time.Now()
	sink := func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
		t, ok := s.Tenants[carrier]
		if !ok {
			return nil
		}
		if err := t.Store.LoadPNRs(ctx, recs, "fill"); err != nil {
			return err
		}
		// The pre-sold day's money, by leg. A codeshare is two records of
		// one sale; the operator's copy carries the marketing carrier's
		// locator and is not counted again.
		for _, r := range recs {
			if r.Origin.Channel == "codeshare" {
				continue
			}
			s.Ledger.Record(r)
		}
		return nil
	}
	plan, err := fill.Day(ctx, sub, fill.Options{LoadFactor: s.fill, Seed: s.fillSeed, Day: s.BookingDate, Tariff: s.tariff,
		Cabins: func(f world.Flight) map[string]int { return host.Cabins(f, s.capacity) }}, sink)
	if err != nil {
		s.log.Error("filling the day failed", "err", err, "records_written", plan.Records)
		return
	}
	s.log.Info("day filled", "load_factor", s.fill, "carriers", plan.Carriers, "flights", plan.Flights,
		"records", plan.Records, "passengers", plan.Passengers, "seats", plan.Seats,
		"connecting", plan.Connecting, "took", time.Since(started).Round(time.Millisecond).String())
}

// flightRecords is the globe's drill-through: every booking any channel
// holds on a flight, straight from the stores. A plane on the map becomes
// the people on it in one click, which is the whole point of drawing it.
func (s *Sim) flightRecords(flight, board string) []eye.FlightRecord {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var out []eye.FlightRecord
	seen := map[string]bool{}
	add := func(fr eye.FlightRecord) bool {
		if fr.Locator == "" || seen[fr.Locator] {
			return len(out) < 200
		}
		seen[fr.Locator] = true
		out = append(out, fr)
		return len(out) < 200
	}
	// A number flies several legs a day; a booking rides this one only if
	// one of its segments boards here.
	onLeg := func(r *pnr.PNR) bool {
		if board == "" {
			return true
		}
		for _, sg := range r.Segments {
			if sg.Board == board && sg.Carrier+strings.TrimLeft(sg.FlightNum, "0") == flight[:2]+strings.TrimLeft(flight[2:], "0") {
				return true
			}
		}
		return false
	}
	// The carrier's own system is the book of record for its flight, and it
	// keeps every booking for the day. A distribution system's ledger is
	// bounded and turns over in minutes at real volume, so asking only the
	// GDSes said "no records" about flights that were full.
	if len(flight) >= 3 {
		if t, ok := s.Tenants[flight[:2]]; ok {
			// Ever on the flight, not only still on it: a cancelled flight's
			// panel is the list of who was booked and what became of them.
			recs, err := t.Store.FindPNRsEverOnFlight(ctx, flight, "", 400)
			if err == nil {
				for _, r := range recs {
					if !onLeg(r) {
						continue
					}
					fr := eye.FlightRecord{Locator: r.RecordLocator, Party: len(r.Passengers),
						Status: string(r.Status), GDS: t.Carrier.Designator, Fare: fareOf(r)}
					// Shown under the locator the passenger was given, which
					// is the selling channel's, when the record carries it.
					for _, l := range r.Locators {
						for _, g := range gdsSlots {
							if l.Owner == g.Designator && l.Value != "" {
								fr.Locator, fr.GDS = l.Value, l.Owner
							}
						}
					}
					if len(r.Passengers) > 0 {
						fr.Surname = r.Passengers[0].Surname
					}
					if !add(fr) {
						return out
					}
				}
			}
		}
	}
	for _, g := range s.GDSes {
		recs, err := g.Store.FindPNRsEverOnFlight(ctx, flight, "", 200)
		if err != nil {
			continue
		}
		for _, r := range recs {
			if !onLeg(r) {
				continue
			}
			fr := eye.FlightRecord{
				Locator: r.RecordLocator, Party: len(r.Passengers),
				Status: string(r.Status), GDS: g.Designator, Fare: fareOf(r),
			}
			if len(r.Passengers) > 0 {
				fr.Surname = r.Passengers[0].Surname
			}
			if !add(fr) {
				return out
			}
		}
	}
	s.fates(ctx, out)
	return out
}

// fareOf renders a record's price for the panel: total and the fare basis.
func fareOf(r *pnr.PNR) string {
	if r.Pricing == nil {
		return ""
	}
	basis := ""
	if len(r.Pricing.Passengers) > 0 && len(r.Pricing.Passengers[0].Bases) > 0 {
		basis = " · " + strings.Join(r.Pricing.Passengers[0].Bases, "/")
	}
	return fmt.Sprintf("%s %d.%02d%s", r.Pricing.Currency, r.Pricing.Total/100, r.Pricing.Total%100, basis)
}

// fates fills in what the selling channel did about each booking after the
// flight failed it: the latest note on its queue, or that it still waits.
// A distribution system on this machine answers for the locators it
// issued; the carrier's copies carry those locators too.
func (s *Sim) fates(ctx context.Context, recs []eye.FlightRecord) {
	byCode := map[string]*GDSNode{}
	for _, g := range s.GDSes {
		byCode[g.Designator] = g
	}
	for i := range recs {
		g, ok := byCode[recs[i].GDS]
		if !ok {
			continue
		}
		rec, err := g.Store.GetPNR(ctx, recs[i].Locator)
		if err != nil {
			continue
		}
		items, err := g.Store.ListQueue(ctx, store.QueueFilter{PNRID: rec.ID, IncludeWorked: true, Limit: 10})
		if err != nil || len(items) == 0 {
			continue
		}
		// The most recent thing that happened wins: a worked item's note
		// over an older pending one, a pending one over nothing.
		var latest *store.QueueItem
		for _, it := range items {
			if latest == nil || it.PlacedAt.After(latest.PlacedAt) {
				latest = it
			}
		}
		for _, it := range items {
			if it.WorkedAt != nil && (latest.WorkedAt == nil || it.WorkedAt.After(*latest.WorkedAt)) {
				latest = it
			}
		}
		if latest.WorkedAt != nil {
			recs[i].Queue = latest.Note
			if recs[i].Queue == "" {
				recs[i].Queue = "worked"
			}
		} else {
			recs[i].Queue = latest.Reason
			recs[i].Waiting = true
		}
	}
}

// simClock is the world's one adjustable timepiece: a position in the sim
// day that advances at warp sim-minutes per wall-minute, and can be re-paced
// or paused while the world runs. Changing the warp re-anchors the clock at
// its current position, so time never jumps -- it just starts passing at the
// new rate. Anchors are stored with their monotonic reading stripped: a
// suspended machine's monotonic clock freezes with it, and a thaw must jump
// the day forward the way the wall did.
type simClock struct {
	mu         sync.Mutex
	anchorWall time.Time
	anchorPos  float64 // sim minutes into the day at anchorWall
	warp       int
}

func newSimClock(warp int) *simClock {
	now := time.Now()
	// The boot anchor derives from absolute wall time, so every process --
	// and every restart -- agrees where today is.
	pos := math.Mod(float64(now.Unix())/60*float64(warp), 24*60)
	return &simClock{anchorWall: now.Round(0), anchorPos: pos, warp: warp}
}

// Pos returns sim minutes into the day at a wall instant.
func (c *simClock) Pos(now time.Time) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.posLocked(now)
}

func (c *simClock) posLocked(now time.Time) float64 {
	elapsed := now.Round(0).Sub(c.anchorWall).Minutes()
	pos := math.Mod(c.anchorPos+elapsed*float64(c.warp), 24*60)
	if pos < 0 {
		pos += 24 * 60
	}
	return pos
}

// Warp returns the current rate.
func (c *simClock) Warp() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.warp
}

// SetWarp re-paces the day from this instant. Zero pauses it.
func (c *simClock) SetWarp(now time.Time, w int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.anchorPos = c.posLocked(now)
	c.anchorWall = now.Round(0)
	c.warp = w
}

// Sync sets the clock to a position and rate somebody else holds -- the
// core's, over federation -- from this instant. A world across machines has
// one clock, and it is the core's: every peer booted at its own default
// warp, anchored its own day from the wall, and drifted hours from the
// switch's before this existed.
func (c *simClock) Sync(now time.Time, pos float64, w int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.anchorPos = math.Mod(pos, 24*60)
	c.anchorWall = now.Round(0)
	c.warp = w
}

// SetWarp is the time control: how many sim minutes pass per wall minute.
// Zero pauses the day -- aircraft hold, departures wait -- while the
// reservations world keeps moving, because bookings do not stop when
// aircraft do. The ceiling keeps one tick's catch-up sane.
func (s *Sim) SetWarp(w int) error {
	if w < 0 || w > 600 {
		return fmt.Errorf("warp %d is outside 0..600", w)
	}
	s.clock.SetWarp(time.Now(), w)
	s.log.Info("chaos: warp set", "warp", w)
	return nil
}

// SetFederationHandler installs the handler behind /federation/ on the
// switch console: the core's registry.
func (s *Sim) SetFederationHandler(h http.Handler) {
	s.fedMu.Lock()
	s.fedHandler = h
	s.fedMu.Unlock()
}

// settledIn reports whether a record in st has no segment still awaiting an
// answer.
func settledIn(ctx context.Context, st store.Store, locator string) (bool, error) {
	rec, err := st.GetPNR(ctx, locator)
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
	return settledIn(ctx, s.GDSStore, locator)
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
	// The largest backlog one tick will replay after a pause.
	const maxCatchUp = 120

	pos := func(at time.Time) int { return int(s.clock.Pos(at)) }
	// The day the world flies is the day it sold: bookings are for the
	// selling date, so that is the date the name lists ask the stores about
	// and the date on every message. Flying the calendar day instead sent
	// the airport empty lists.
	day := s.BookingDate
	// A filled world starts its day with the books loaded: whatever an
	// earlier run left is purged and the day's bookings written before the
	// first tick, so the first name lists out are the full ones.
	if s.fill > 0 {
		// A restart in the middle of a filled day keeps the day: the books
		// already hold it, and refilling would cost the first departures
		// their name lists while a million records were rewritten.
		if !s.refill && s.dayFilled(ctx) {
			s.log.Info("day already filled; keeping the books", "date", day.Format("2006-01-02"))
		} else {
			s.purgeTenants(ctx, time.Now())
			s.fillDay(ctx)
		}
	}
	s.rebuildInventories(ctx)
	s.rebuildLedger(ctx)
	// The plan settles what was sold for the day: the advance sales the
	// agents ticketed, handed to each airline as its HOT before the day
	// flies, the way yesterday's sales reach an airline's accountants; and
	// the carriers bill each other for the codeshare coupons.
	s.Settle(ctx)
	s.Bill(ctx)
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
			// The day wrapped. What was booked for it has flown or not;
			// the tenants' books of record start the new day clean.
			prev = 0
			s.dayMu.Lock()
			before := s.dayStarted
			s.dayStarted = time.Now()
			s.dayMu.Unlock()
			go func() {
				s.purgeTenants(ctx, before)
				s.fillDay(ctx)
				s.rebuildInventories(ctx)
				s.rebuildLedger(ctx)
				s.Settle(ctx)
				s.Bill(ctx)
			}()
		}
		if cur-prev > maxCatchUp {
			prev = cur - maxCatchUp
		}
		for code, t := range s.Tenants {
			for _, f := range s.Flights[code] {
				if s.isClosed(f.From, f.To) {
					continue
				}
				reg := f.Tail
				if reg == "" {
					reg = fmt.Sprintf("SKY%03d", regHash(f)%1000)
				}
				depDelay, arrDelay := s.flightDelays(f, day)
				fate := s.fate.Of(f)
				cancelled := fate.Cancelled
				// The ground story of a departure, at the hours it really
				// happens: reservations sends the airport the name list,
				// the counter fills in waves, the amendments follow, the
				// counter closes, the gate boards, the sortation system
				// reports the hold, and the door closes. The events due
				// this tick run in order in one goroutine: at high warp
				// several fall in one tick, and a door that closed before
				// the counter opened would be a different story.
				if cancelled {
					// The record says this one never went. The airport
					// hears it two hours out, after the counter opened;
					// nothing departs or arrives, and the manifest is let
					// go at the hour it would have landed.
					if w := wrapMin(f.DepMin - cancelledBefore); w > prev && w <= cur {
						go s.announceCancellation(ctx, t, f, day, fate.Reason)
					}
					if a := f.ArrMin; a > prev && a <= cur {
						t.Forget(ctx, f, day)
					}
				}
				// Flow management: two hours before off-block the Network
				// Manager gives a regulated flight its slot.
				if fate.CTOT > 0 && !cancelled && s.ANSP != nil {
					if w := wrapMin(f.DepMin - slotBefore); w > prev && w <= cur {
						go func(f world.Flight, fate dayplan.Flight) {
							if err := s.ANSP.Slot(ctx, s.carriers[f.Carrier], f, day, fate); err != nil {
								s.log.Debug("slot not sent", "flight", f.Carrier+f.Number, "err", err)
								return
							}
							s.askSlot(ctx, t, f, fate)
						}(f, fate)
					}
				}
				var due []groundEvent
				for _, g := range groundEvents {
					if cancelled && g.before < cancelledBefore {
						continue
					}
					w := f.DepMin - g.before
					if w < 0 {
						// A departure in the first hours after midnight
						// opens its counter the evening before. Left
						// negative, these windows never came and the
						// flights before 03:00 flew with no ground story.
						w += 24 * 60
					}
					if w > prev && w <= cur {
						due = append(due, g)
					}
				}
				if len(due) > 0 {
					go func(f world.Flight, due []groundEvent) {
						for _, g := range due {
							err := g.do(ctx, s, t, f, day)
							if err == nil || errors.Is(err, dcs.ErrFlightNotFound) {
								// Not under control: the flight's list went out
								// before this process started. Nothing to say.
								continue
							}
							s.log.Debug(g.what+" failed", "flight", f.Carrier+f.Number, "err", err)
						}
					}(f, due)
				}
				if cancelled {
					continue
				}
				if d := f.DepMin + depDelay; d > prev && d <= cur {
					s.departure(ctx, t, f, day, reg, depDelay)
				}
				if f.Actual != nil && f.Actual.Diverted && f.Actual.DivertedTo != "" {
					// The record says the aircraft landed somewhere else
					// first. Seven tenths of the way there is where a
					// diversion is usually decided; the record does not say.
					if w := f.DepMin + depDelay + f.BlockMin*7/10; w > prev && w <= cur {
						go s.recordedDiversion(ctx, t, f, day, reg)
					}
				}
				if a := f.ArrMin + arrDelay; a > prev && a <= cur {
					s.arrival(ctx, t, f, day, reg, arrDelay)
				}
			}
		}
		prev = cur
	}
}

// sellingDate is the day the world sells and flies: the recorded day on a
// replay, thirty days out on a synthetic one.
func sellingDate(m *world.Manifest) time.Time { return world.SellingDate(m) }

// slotBefore is when the Network Manager issues a regulated flight its
// slot: two hours before off-block, the manual's SIT1.
const slotBefore = 120

// flightDelays is what the day will do to a flight: the plan's fate --
// the record's delays on a replay, the model's with its slots, late
// aircraft and crews on a synthetic day -- or, before the plan exists,
// the record or the model alone.
func (s *Sim) flightDelays(f world.Flight, day time.Time) (dep, arr int) {
	if s.fate != nil {
		if _, ok := s.fate.Flights[dayplan.Key(f)]; ok {
			pf := s.fate.Of(f)
			return pf.DepDelay, pf.ArrDelay
		}
	}
	if d, a, ok := f.RecordedDelay(); ok {
		return d, a
	}
	return delayFor(f, day)
}

// runsCarrier says this machine hosts the carrier's tenant.
func (s *Sim) runsCarrier(code string) bool {
	_, ok := s.Tenants[strings.ToUpper(code)]
	return ok
}

// proxyCarrier forwards a seat's request to the machine that runs the
// carrier, when this one federates; false when nobody does.
func (s *Sim) proxyCarrier(w http.ResponseWriter, r *http.Request, code string) bool {
	if s.ConsoleProxy == nil {
		return false
	}
	return s.ConsoleProxy(w, r, code)
}

// isExternal says a carrier is run by someone's own node, not a tenant.
func isExternal(opts Options, designator string) bool {
	for _, d := range opts.External {
		if strings.EqualFold(strings.TrimSpace(d), designator) {
			return true
		}
	}
	return false
}

// processSecret is the per-process link secret when none was configured,
// so the switch and the start pack agree on the tokens.
var processSecret = func() string {
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}()

func linkSecretOf(opts Options) string {
	if opts.LinkSecret != "" {
		return opts.LinkSecret
	}
	return processSecret
}

// linkToken is a carrier's shared secret for its link: an HMAC of the
// designator under the world's secret, so every machine derives the same.
func linkToken(secret, designator string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.ToUpper(designator)))
	return hex.EncodeToString(mac.Sum(nil))[:32]
}

// StartPack is what a player's own jetway node needs to fly one of the
// world's carriers: the node's configuration, the carrier's schedule as an
// SSIM file, and the notes that go with them.
type StartPack struct {
	Carrier    string   `json:"carrier"`
	Name       string   `json:"name"`
	Hub        string   `json:"hub"`
	Switch     string   `json:"switch"`
	Token      string   `json:"token"`
	Flights    int      `json:"flights"`
	ConfigYAML string   `json:"config_yaml"`
	SSIM       string   `json:"ssim"`
	Notes      []string `json:"notes"`
}

// Pack builds the start pack for an external carrier.
func (s *Sim) Pack(designator string) (*StartPack, error) {
	code := strings.ToUpper(designator)
	c, ok := s.carriers[code]
	if !ok {
		return nil, fmt.Errorf("no carrier %s in this world", code)
	}
	if !s.external[code] {
		return nil, fmt.Errorf("%s is run by this world; boot with -external %s to hand it to your own node", code, code)
	}
	switchAddr := s.publicSwitch
	if switchAddr == "" && s.Switch != nil {
		switchAddr = s.Switch.Addr("link-net")
	}
	firstDesignator, firstTTY := switchIdentity(0)
	cfg := config.Default()
	cfg.Identity = config.Identity{Designator: code, TTYAddress: c.TTYAddress, Name: c.Name}
	cfg.Store = config.Store{Backend: "mem"}
	cfg.Spool.Enabled = false
	cfg.Demo.Carriers = false
	cfg.HTTP.Addr = ":8080"
	cfg.HTTP.Console = true
	cfg.Ingress = nil
	cfg.Peers = []config.Peer{{Name: firstDesignator, Carrier: firstDesignator, TTYAddress: firstTTY, Format: "typeb",
		Token: linkToken(s.linkSecret, code), Egress: config.Egress{Type: "link_dial", Addr: switchAddr, Role: "carrier"}}}
	for _, g := range s.GDSes {
		cfg.Peers = append(cfg.Peers, config.Peer{Name: g.Designator, Carrier: g.Designator, TTYAddress: g.Address, Format: "typeb", Egress: config.Egress{Type: "via", Via: firstDesignator}})
	}
	cfg.Peers = append(cfg.Peers,
		config.Peer{Name: dspPeer(0), TTYAddress: dspAddress(0), Format: "typeb", Egress: config.Egress{Type: "via", Via: firstDesignator}},
		config.Peer{Name: atcPeer(0), TTYAddress: atcAddress(0), Format: "aftn", Egress: config.Egress{Type: "via", Via: firstDesignator}},
		config.Peer{Name: govPeer(0), Carrier: govPeer(0), TTYAddress: govAddress(0), Format: "edifact", Egress: config.Egress{Type: "via", Via: firstDesignator}})
	y, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	sub := &world.Manifest{Airports: s.Manifest.Airports, Carriers: []world.Carrier{c}, Flights: s.Flights[code]}
	var ssim bytes.Buffer
	if err := world.WriteSSIM(&ssim, sub, s.BookingDate); err != nil {
		return nil, err
	}
	return &StartPack{Carrier: code, Name: c.Name, Hub: c.Hub, Switch: switchAddr, Token: linkToken(s.linkSecret, code), Flights: len(s.Flights[code]),
		ConfigYAML: string(y), SSIM: ssim.String(), Notes: []string{
			"Run: jetwayd -config carrier.yaml. The node dials the world's switch with link_dial and the token; the world's distribution systems sell into your inventory over the same link.",
			"Your node is a bare jetway: reservations, inventory, DCS and console. The world's ground story for your flights -- name lists, check-in, closure -- is yours to drive; a PNL the airport never gets is a flight that never opens.",
			"The schedule is the SSIM file; load it into whatever you run, or read it with jetway's pkg/ssim.",
			"The token is this world's for this carrier and changes when the world restarts unless it was booted with -link-secret.",
		}}, nil
}

// servePack hands out a carrier's start pack.
func (s *Sim) servePack(w http.ResponseWriter, r *http.Request) {
	pack, err := s.Pack(r.PathValue("carrier"))
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pack) //nolint:errcheck
}

// tapTenantRevenue feeds the revenue ledger from a carrier's own book on a
// machine that runs no distribution system: a region hears each sale as
// the carrier's copy of the record, priced, and that is the only copy it
// has. Where a distribution system runs on the same machine its copy is
// counted instead, once, so the two never double.
func (s *Sim) tapTenantRevenue(ctx context.Context, bus *gateway.Bus) {
	if len(s.GDSes) > 0 {
		return
	}
	go s.tapBus(ctx, bus, func(ev gateway.Event) {
		if ev.Type != gateway.EvPNR {
			return
		}
		if p, ok := ev.Data.(map[string]any); ok {
			if rec, ok := p["record"].(*pnr.PNR); ok && rec != nil && rec.Pricing != nil && rec.Version <= 1 && rec.Origin.Channel != "codeshare" {
				s.Ledger.Record(rec)
			}
		}
	})
}

// Fate is the day's plan.
func (s *Sim) Fate() *dayplan.Plan { return s.fate }

// serveDayPlan is the day's weather, regulations and fate in numbers.
func (s *Sim) serveDayPlan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.fate == nil {
		w.Write([]byte("{}")) //nolint:errcheck
		return
	}
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"day": s.fate.Day, "replay": s.fate.Replay, "summary": s.fate.Summary,
		"weather": s.fate.Weather, "regulations": s.fate.Regulations,
	})
}

// fateLines are the panel's lines for a flight's fate: the delay in its
// parts, and the crew.
func fateLines(f dayplan.Flight) (delay, crew string) {
	var parts []string
	if f.Own > 0 {
		parts = append(parts, fmt.Sprintf("carrier %dm", f.Own))
	}
	if f.ATFM > 0 {
		parts = append(parts, fmt.Sprintf("slot %dm (%s)", f.ATFM, f.Regulation))
	}
	if f.LateAircraft > 0 {
		parts = append(parts, fmt.Sprintf("late aircraft %dm", f.LateAircraft))
	}
	if f.Reserve {
		parts = append(parts, fmt.Sprintf("reserve crew callout %dm", dayplan.ReserveCall))
	}
	if len(parts) > 0 {
		delay = fmt.Sprintf("%dm: %s", f.DepDelay, strings.Join(parts, " · "))
	}
	if f.Duty > 0 {
		crew = fmt.Sprintf("duty %d of the tail's day", f.Duty)
		switch {
		case f.Cancelled && strings.HasPrefix(f.Reason, "crew"):
			crew += " · timed out, cancelled"
		case f.Reserve:
			crew += " · timed out, reserves called at the base"
		case f.Extended:
			crew += " · legal on the two-hour extension"
		default:
			crew += " · legal"
		}
	}
	return delay, crew
}

// cancelledBefore is when a recorded cancellation is announced: two hours
// before the scheduled departure, which is a typical notice and, unlike
// the record, a moment -- BTS says a flight was cancelled, not when.
const cancelledBefore = 120

// groundEvent is one step of a departure's ground story, at a number of
// minutes before scheduled departure.
type groundEvent struct {
	before int
	what   string
	do     func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error
}

// groundEvents is the timetable every departure follows. Check-in opens
// three hours out and closes at forty-five minutes; boarding runs from
// thirty; the door closes at ten.
var groundEvents = []groundEvent{
	{180, "pnl", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.SendPNL(ctx, f, day)
	}},
	{170, "pnr push", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		// A state's PNR regime hears the records before departure and
		// again at the door; the international flight's first push goes
		// once the name list has.
		return t.PushPNRGOV(ctx, f, day)
	}},
	{150, "crew", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		// A crew the plan times out: the seat may call reserves before
		// the cancellation is announced at two hours.
		if fate := s.fate.Of(f); fate.Cancelled && strings.HasPrefix(fate.Reason, "crew") {
			s.askCrew(ctx, f, fate)
		}
		return nil
	}},
	{150, "check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CheckIn(ctx, f, day, 150)
	}},
	{125, "retime", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		// The day's delay is known to the record (or the hash) from the
		// start; the carrier learns it about two hours out, and announces
		// the ones long enough to matter: forty-six minutes and up.
		dep, arr := s.flightDelays(f, day)
		if dep < 46 {
			return nil
		}
		if !s.askRetime(ctx, f, dep, arr) {
			return nil
		}
		return t.Retime(ctx, f, day, dep, arr)
	}},
	{110, "aircraft substitution", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		// One flight in a hundred and fifty goes technical after check-in
		// has opened: a smaller aircraft takes it, the cabin is re-seated,
		// distribution hears the EQT.
		if aogHash(f.Carrier+f.Number+day.Format("0102"))%150 != 0 {
			return nil
		}
		if !s.askSubstitute(ctx, f) {
			s.fate.Update(f, func(pf *dayplan.Flight) {
				pf.Cancelled, pf.Reason, pf.Code = true, "cancelled: aircraft unserviceable", "A"
			})
			s.announceCancellation(ctx, t, f, day, "cancelled: aircraft unserviceable")
			return nil
		}
		_, err := t.Substitute(ctx, f, day)
		return err
	}},
	{120, "check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CheckIn(ctx, f, day, 120)
	}},
	{90, "check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CheckIn(ctx, f, day, 90)
	}},
	{60, "flight plan", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.FileFlightPlan(ctx, f, day)
	}},
	{60, "adl", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.SendADL(ctx, f, day)
	}},
	{60, "check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CheckIn(ctx, f, day, 60)
	}},
	{46, "check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CheckIn(ctx, f, day, 46)
	}},
	{45, "close check-in", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.CloseCheckIn(ctx, f, day)
	}},
	{30, "boarding", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.Board(ctx, f, day, 30)
	}},
	{20, "boarding", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.Board(ctx, f, day, 20)
	}},
	{14, "boarding", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.Board(ctx, f, day, 14)
	}},
	{12, "bag report", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.ReportBags(ctx, f, day)
	}},
	{10, "close", func(ctx context.Context, s *Sim, t *host.Tenant, f world.Flight, day time.Time) error {
		return t.Close(ctx, f, day)
	}},
}

// Settle runs the settlement plan over the day the agents have sold: one
// HOT per airline from the distribution systems' ticketed records, each
// reconciled against the carrier's own book when this process runs the
// carrier. The result is kept for the instruments and served as files.
func (s *Sim) Settle(ctx context.Context) {
	if len(s.GDSes) == 0 && len(s.Tenants) == 0 {
		return // a core with neither books nor agents federates instead
	}
	// The agents are the distribution systems: those on this machine with
	// their books, the rest by name, so a region can attribute the sales
	// its carriers' books carry to the system that made them.
	local := map[string]bool{}
	agents := make([]settle.Agent, 0, len(gdsSlots))
	for _, g := range s.GDSes {
		agents = append(agents, settle.Agent{Designator: g.Designator, Name: g.Name, Store: g.Store})
		local[g.Designator] = true
	}
	for _, slot := range gdsSlots {
		if !local[slot.Designator] {
			agents = append(agents, settle.Agent{Designator: slot.Designator, Name: slot.Name})
		}
	}
	var airlines []settle.Airline
	for _, c := range s.Manifest.Carriers {
		a := settle.Airline{Designator: c.Designator, Accounting: s.accountingCode(c.Designator)}
		if t, ok := s.Tenants[c.Designator]; ok {
			a.Store = t.Store
		}
		airlines = append(airlines, a)
	}
	if s.plan.BSP == "" {
		s.plan = settle.Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	}
	sum, err := s.plan.Run(ctx, s.BookingDate, agents, airlines)
	if err != nil {
		s.log.Warn("settlement failed", "err", err)
		return
	}
	s.settleMu.Lock()
	s.settlement = sum
	s.settleMu.Unlock()
	s.log.Info("settled", "airlines", sum.Airlines, "transactions", sum.Transactions, "gross", sum.Gross,
		"remittance", sum.Remittance, "matched", sum.Matched, "unreported", sum.Unreported, "unknown", sum.Unknown, "unverified", sum.Unverified)
}

// StartSettling runs the plan on a timer for the life of the world: every
// few minutes over what the books hold now, so a machine that only holds
// agents' books -- a distribution system's -- or carriers' books alone
// builds its statement through the day rather than at the wrap alone.
// Each role calls it once its books and agents are in place; started any
// earlier, the loop read them while the boot was still writing them.
func (s *Sim) StartSettling(every time.Duration) {
	if every <= 0 {
		every = 5 * time.Minute
	}
	go s.settleLoop(s.ctx, every)
}

// settleLoop re-runs the plan on a timer; a run still going is not
// doubled.
func (s *Sim) settleLoop(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	var running atomic.Bool
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if !running.CompareAndSwap(false, true) {
			continue
		}
		go func() {
			defer running.Store(false)
			s.Settle(ctx)
			s.Bill(ctx)
		}()
	}
}

// Bill runs the interline billing plan over the books this machine holds:
// every ticketed coupon flown by one carrier on another's document,
// prorated by mileage, less the service charge, as an invoice per pair.
func (s *Sim) Bill(ctx context.Context) {
	if len(s.GDSes) == 0 && len(s.Tenants) == 0 {
		return
	}
	var books []interline.Book
	for _, g := range s.GDSes {
		books = append(books, interline.Book{Name: g.Designator, Store: g.Store})
	}
	for code, t := range s.Tenants {
		books = append(books, interline.Book{Name: code, Store: t.Store})
	}
	p := &interline.Plan{ServiceCharge: 900, Accounting: s.accountingCode}
	sum, err := p.Run(ctx, s.BookingDate, s.Manifest, books)
	if err != nil {
		s.log.Warn("interline billing failed", "err", err)
		return
	}
	s.settleMu.Lock()
	s.billing = sum
	s.settleMu.Unlock()
	s.log.Info("billed", "invoices", sum.Invoices, "coupons", sum.Coupons, "prorate", sum.Prorate, "service_charge", sum.ServiceCharge, "net", sum.Net)
}

// Billing is the latest interline billing summary, or nil before the first.
func (s *Sim) Billing() *interline.Summary {
	s.settleMu.Lock()
	defer s.settleMu.Unlock()
	return s.billing
}

// SetBilling installs a billing view assembled elsewhere: the core's merge.
func (s *Sim) SetBilling(sum *interline.Summary) {
	s.settleMu.Lock()
	s.billing = sum
	s.settleMu.Unlock()
}

func (s *Sim) serveBilling(w http.ResponseWriter, r *http.Request) {
	sum := s.Billing()
	if sum == nil {
		http.Error(w, "no billing yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sum.AsView()) //nolint:errcheck
}

// serveInvoice hands out one pair's invoice, lines and all, proxying to
// the machine that holds it when this one does not.
func (s *Sim) serveInvoice(w http.ResponseWriter, r *http.Request) {
	key := strings.ToUpper(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/billing/"), ".json"))
	sum := s.Billing()
	if sum == nil {
		http.NotFound(w, r)
		return
	}
	inv, ok := sum.ByPair[key]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if inv.Lines == nil && inv.Peer != "" {
		if proxyPass(w, r, inv.Peer) {
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(inv) //nolint:errcheck
}

// SetSettlement installs a settlement view assembled elsewhere: the core's
// merge of what its regions settled.
func (s *Sim) SetSettlement(sum *settle.Summary) {
	s.settleMu.Lock()
	s.settlement = sum
	s.settleMu.Unlock()
}

// Settlement is the latest settlement summary, or nil before the first.
func (s *Sim) Settlement() *settle.Summary {
	s.settleMu.Lock()
	defer s.settleMu.Unlock()
	return s.settlement
}

// serveRET hands out one distribution system's agent reporting file for
// the day: GET /ret/{designator}.txt.
func (s *Sim) serveRET(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/ret/"), ".txt"))
	var g *GDSNode
	for _, x := range s.GDSes {
		if x.Designator == code {
			g = x
		}
	}
	if g == nil {
		http.NotFound(w, r)
		return
	}
	var airlines []settle.Airline
	for _, c := range s.Manifest.Carriers {
		airlines = append(airlines, settle.Airline{Designator: c.Designator, Accounting: s.accountingCode(c.Designator)})
	}
	p := s.plan
	if p.BSP == "" {
		p = settle.Plan{BSP: "WSK", Country: "XX", Currency: "USD2", CommissionRate: 100}
	}
	ret, err := p.WriteRET(r.Context(), s.BookingDate, settle.Agent{Designator: g.Designator, Name: g.Name, Store: g.Store}, airlines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
	if err := ret.Write(w); err != nil {
		s.log.Warn("could not write RET", "gds", code, "err", err)
	}
}

func (s *Sim) serveSettlement(w http.ResponseWriter, r *http.Request) {
	sum := s.Settlement()
	if sum == nil {
		http.Error(w, "no settlement yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sum.AsView()) //nolint:errcheck
}

// serveHOT hands out one airline's file as the plan produced it.
func (s *Sim) serveHOT(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/settlement/"), ".hot"))
	sum := s.Settlement()
	if sum == nil {
		http.NotFound(w, r)
		return
	}
	st, ok := sum.Statements[code]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if st.File == nil {
		// Another machine holds the file: the region whose carriers'
		// books the plan ran over.
		if st.Peer != "" && proxyPass(w, r, st.Peer) {
			return
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=us-ascii")
	if err := st.File.Write(w); err != nil {
		s.log.Warn("could not write HOT", "airline", code, "err", err)
	}
}

// switchCount is how many switches the options ask for, at least one.
func switchCount(opts Options) int {
	if opts.Switches < 1 {
		return 1
	}
	return opts.Switches
}

// homeSwitch is the switch a carrier is homed on: deterministic by
// designator, so every process in a multi-machine world agrees.
func homeSwitch(designator string, n int) int {
	if n <= 1 {
		return 0
	}
	return aogHash(designator) % n
}

// switchIdentity names switch k: 1X is the first, 1Y the second.
func switchIdentity(k int) (designator, tty string) {
	d := "1" + string(rune('X'+k))
	return d, "XCHDD" + d
}

// buildSwitch assembles switch k of n. Subscribers homed here are links
// it accepts; those homed elsewhere are reached via the trunk to the
// first switch (from a later one) or via the trunk the later switch
// dialled in on (from the first). The distribution systems, the
// networks and the border agencies live on the first switch, as do the
// console and the instruments; trunkTo is the first switch's listener,
// which a later switch dials.
func buildSwitch(ctx context.Context, m *world.Manifest, opts Options, k, n int, trunkTo string, extend func(*http.ServeMux), log *slog.Logger) (*node.Node, error) {
	console := opts.Console
	if k > 0 {
		console = ""
	}
	firstDesignator, firstTTY := switchIdentity(0)
	cfg := config.Default()
	// jetway spools every inbound message to disk by default, which is
	// right for a carrier and wrong for a switch relaying sixteen thousand a
	// second on a machine that forgets its day at midnight anyway.
	cfg.Spool.Enabled = false
	designator, tty := switchIdentity(k)
	name := "wholesky switch"
	if n > 1 {
		name = fmt.Sprintf("wholesky switch %d of %d", k+1, n)
	}
	cfg.Identity = config.Identity{Designator: designator, TTYAddress: tty, Name: name}
	// Traffic for a subscriber on another switch goes down the trunk. The
	// first switch's trunk to switch k is the link switch k dialled in on;
	// a later switch's trunk is the link it holds to the first.
	viaTrunk := func(home int) config.Egress {
		if home == k {
			return config.Egress{Type: "tcp_accept"}
		}
		if k == 0 {
			d, _ := switchIdentity(home)
			return config.Egress{Type: "via", Via: d}
		}
		return config.Egress{Type: "via", Via: firstDesignator}
	}
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

	// One network access point serves every plain-TCP subscriber: each
	// connection names itself in its hello frame, the way v0.1.16's
	// by_hello identification allows. MATIP circuits keep a listener per
	// carrier -- a MATIP session is a host-to-host circuit, and its own
	// session-open is the identification.
	bind := opts.LinkBind
	if bind == "" {
		bind = "127.0.0.1"
	}
	cfg.Ingress = append(cfg.Ingress, config.Ingress{
		Name: "link-net", Type: "tcp", Addr: net.JoinHostPort(bind, "0"),
		Identify: config.Identify{ByHello: true},
	})
	addPeer := func(designator, tty, format string, home int) {
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: designator, Carrier: designator, TTYAddress: tty,
			Format: format,
			Egress: viaTrunk(home),
		})
	}
	for _, c := range m.Carriers {
		home := homeSwitch(c.Designator, n)
		if c.Transport == "matip" && home == k {
			cfg.Ingress = append(cfg.Ingress, config.Ingress{
				Name: "link-" + strings.ToLower(c.Designator), Type: "matip",
				Addr: net.JoinHostPort(bind, "0"), Identify: config.Identify{Peer: c.Designator},
			})
		}
		addPeer(c.Designator, c.TTYAddress, c.Format, home)
		// The carrier's ICAO designator routes its AFTN traffic here too.
		cfg.Peers[len(cfg.Peers)-1].ICAO = c.ICAO
		if isExternal(opts, c.Designator) {
			// Someone else's node will dial in as this carrier: it has to
			// know the secret.
			cfg.Peers[len(cfg.Peers)-1].Token = linkToken(linkSecretOf(opts), c.Designator)
		}
	}
	// The other networks: a datalink provider and an ANSP per region shard.
	for shard := 0; shard < networkShards; shard++ {
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: dspPeer(shard), TTYAddress: dspAddress(shard), Format: "typeb",
			Egress: viaTrunk(0),
		})
		// One link takes the aeronautical network's unclaimed indicators --
		// the towers' -- so the flight plans land on one ANSP; the other
		// shard's ANSP still sends its own towers' messages.
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: atcPeer(shard), TTYAddress: atcAddress(shard), Format: "aftn", AFTN: shard == 0,
			Egress: viaTrunk(0),
		})
		// The border control agency receives passenger lists as EDIFACT
		// interchanges addressed to it, so it is a carrier-like peer: the
		// switch routes on the UNB recipient.
		cfg.Peers = append(cfg.Peers, config.Peer{
			Name: govPeer(shard), Carrier: govPeer(shard), TTYAddress: govAddress(shard), Format: "edifact",
			Egress: viaTrunk(0),
		})
	}
	for _, slot := range gdsSlots {
		addPeer(slot.Designator, gdsAddress(slot), "typeb", 0)
	}
	// The trunks. The first switch accepts each later switch's link; a
	// later switch holds its link to the first open itself.
	if k == 0 {
		for j := 1; j < n; j++ {
			d, t := switchIdentity(j)
			cfg.Peers = append(cfg.Peers, config.Peer{Name: d, Carrier: d, TTYAddress: t, Format: "typeb", Trunk: true,
				Egress: config.Egress{Type: "tcp_accept"}})
		}
	} else {
		cfg.Peers = append(cfg.Peers, config.Peer{Name: firstDesignator, Carrier: firstDesignator, TTYAddress: firstTTY, Format: "typeb", Trunk: true,
			Egress: config.Egress{Type: "link_dial", Addr: trunkTo, Role: "switch"}})
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("switch config: %w", err)
	}
	return node.Build(ctx, cfg, log.With("node", strings.ToLower(designator)), node.Options{
		LocatorSecret: []byte("wholesky-switch"),
		SkipConsole:   console == "",
		ExtendAPI:     extend,
	})
}

// buildGDSNode assembles one distribution system: a gateway with an
// availability cache and queues, dialled into its own listener on the
// switch. watcher marks the node whose bus flies the aircraft -- movement
// traffic is copied to its address, and only its movement events reach the
// Eye, so each flight is flown once.
func (s *Sim) buildGDSNode(ctx context.Context, m *world.Manifest, g *GDSNode,
	switchAddr string, backing func(code string) store.Store, watcher bool, log *slog.Logger) error {

	log = log.With("node", strings.ToLower(g.Designator))
	st := backing(g.Designator)
	if st == nil {
		mem := store.NewMem()
		mem.MaxMessages, mem.MaxRecords = s.maxMessages, s.maxRecords
		st = mem
	}
	bus := gateway.NewBus(256)
	gw := gateway.New(gateway.Identity{
		Designator: g.Designator, TTYAddress: g.Address, Name: g.Name,
	}, st, bus, log, []byte("wholesky-"+g.Designator))
	gw.Avail = avail.NewCache()
	gw.Tariff = s.tariff
	// The world keeps every clock on UTC: a flight's times are UTC minutes
	// and the segments sold from them are written the same way, so a UTC
	// schedule message applies to a held segment with no offset at all.
	gw.LocalClock = func(string, time.Time) (time.Duration, bool) { return 0, true }
	// Without a queue manager a schedule change has nowhere to put the
	// bookings it touches -- applySchedule quietly does nothing. The halos on
	// the map are these placements.
	gw.Queues = &queue.Manager{Store: st, Log: log,
		Notify: func(item *store.QueueItem) { bus.Publish(gateway.EvQueue, item) }}

	client := &transport.Client{
		Addr: switchAddr, Framer: transport.DefaultFramer(),
		Hello: transport.Hello{Peer: g.Designator, Role: "gds", Format: "typeb"},
		Log:   log,
	}
	gw.Sender = client
	client.OnMessage = func(ctx context.Context, peer string, raw []byte) error {
		_, err := gw.Ingest(ctx, "net", raw)
		return err
	}
	client.OnUp = func() { g.up.Store(true) }
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
	g.GW, g.Store, g.Bus = gw, st, bus

	go s.tapBus(ctx, bus, func(ev gateway.Event) {
		switch ev.Type {
		case gateway.EvMessage:
			// Each GDS row was registered without a bus, because the bus is
			// born here; feed it by hand.
			if p, ok := ev.Data.(map[string]any); ok {
				s.Fleet.Count(g.Designator, p)
			}
		case gateway.EvMovement:
			if !watcher {
				return
			}
			s.Movements.Add(1)
			s.Stats.OnMovement()
			if p, ok := ev.Data.(map[string]any); ok {
				s.Eye.OnMovement(p)
			}
		case gateway.EvPNR:
			s.Eye.OnBooking()
			s.Stats.OnBooking()
			// The price rides the record; count it once, when the record
			// is first written.
			if p, ok := ev.Data.(map[string]any); ok {
				if rec, ok := p["record"].(*pnr.PNR); ok && rec != nil && rec.Pricing != nil && rec.Version <= 1 {
					s.Stats.OnRevenue(rec.Pricing.Total)
					s.Ledger.Record(rec)
				}
			}
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
	return nil
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
			// The airport hears it too: anyone already checked in comes off
			// with their bags, and the counter takes nobody else.
			if err := t.CancelFlight(context.Background(), f, s.BookingDate, "airport "+iata+" closed"); err != nil {
				s.log.Debug("dcs cancellation failed", "flight", f.Carrier+f.Number, "err", err)
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

// departure is the aircraft leaving. With a datalink provider on this
// machine the aircraft reports OUT and OFF, the provider forwards the report
// to the airline, and the airline's operations desk derives the MVT from it;
// the tower sends its own DEP over the AFTN. Without one, the tenant asserts
// the movement itself, as the world did before it had aircraft that talk.
func (s *Sim) departure(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time, reg string, delay int) {
	c := s.carriers[f.Carrier]
	dep := day.Add(time.Duration(f.DepMin+delay) * time.Minute)
	off := dep.Add(12 * time.Minute)
	s.Departures.Add(1)
	if s.DSP == nil {
		if err := t.Depart(ctx, f, day, reg, delay); err != nil {
			s.log.Debug("departure not sent", "flight", f.Carrier+f.Number, "err", err)
		}
		return
	}
	go func() {
		if err := s.DSP.Report(ctx, c, f, reg, acars.KindDEP, dep, off); err != nil {
			// The first few are worth a warning: a provider that cannot
			// report is a sky with no aircraft in it. After that the count
			// in the run log carries the story.
			if s.reportErrs.Add(1) <= 20 {
				s.log.Warn("aircraft report not sent", "flight", f.Carrier+f.Number, "carrier", c.Designator, "err", err)
			}
		}
		if s.ANSP != nil {
			if err := s.ANSP.Departure(ctx, c, f, day, off); err != nil {
				s.log.Debug("tower departure not sent", "flight", f.Carrier+f.Number, "err", err)
			}
		}
	}()
}

// arrival is the aircraft landing: ON and IN from the aircraft, ARR from
// the tower.
func (s *Sim) arrival(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time, reg string, delay int) {
	c := s.carriers[f.Carrier]
	in := day.Add(time.Duration(f.ArrMin+delay) * time.Minute)
	on := in.Add(-8 * time.Minute)
	if s.DSP == nil {
		if err := t.Arrive(ctx, f, day, reg, delay); err != nil {
			s.log.Debug("arrival not sent", "flight", f.Carrier+f.Number, "err", err)
		}
		return
	}
	go func() {
		if err := s.DSP.Report(ctx, c, f, reg, acars.KindARR, on, in); err != nil {
			s.log.Debug("aircraft report not sent", "flight", f.Carrier+f.Number, "err", err)
		}
		if s.ANSP != nil {
			if err := s.ANSP.Arrival(ctx, c, f, f.To, on); err != nil {
				s.log.Debug("tower arrival not sent", "flight", f.Carrier+f.Number, "err", err)
			}
		}
	}()
}

// wrapMin folds a minute of the day into 0..1440.
func wrapMin(m int) int {
	m %= 24 * 60
	if m < 0 {
		m += 24 * 60
	}
	return m
}

// recordedDiversion sends the DIV a replayed flight really made, to the
// airport the record names.
func (s *Sim) recordedDiversion(ctx context.Context, t *host.Tenant, f world.Flight, day time.Time, reg string) {
	now := time.Now().UTC()
	m := &mvt.Message{
		Kind: mvt.KindDIV, Flight: f.Carrier + strings.TrimLeft(f.Number, "0"),
		Day: fmt.Sprintf("%02d", day.Day()), Registration: reg, Station: f.To,
		EA: &mvt.ETA{Time: now.Add(25 * time.Minute).Format("1504"), Airport: f.Actual.DivertedTo},
		SI: "DIVERSION AS RECORDED BTS",
	}
	if err := t.SendOps(ctx, m); err != nil {
		s.log.Debug("recorded diversion not sent", "flight", f.Carrier+f.Number, "err", err)
	}
}

// tenantConsole is a carrier's console: the gateway's pages plus the
// departures board, with the agent's actions wired to the wire.
func tenantConsole(t *host.Tenant, bus *gateway.Bus, log *slog.Logger) *api.Server {
	return &api.Server{
		Gateway: t.Gateway, Store: t.Store, Bus: bus, Log: log, Console: true,
		Ground: t.DCS, OnAccept: t.OnAccept, OnOffload: t.OnOffload, OnClose: t.SendClosure,
	}
}

// inboundDelay is the arrival delay the flight day will give a flight, for
// the connecting passengers who depend on it.
func (s *Sim) inboundDelay(f world.Flight, day time.Time) int {
	_, arr := s.flightDelays(f, day)
	return arr
}

// flightDCS is the globe's other drill-through: what departure control
// says about the aircraft, if this machine runs the carrier.
func (s *Sim) flightDCS(flight, board string) any {
	if len(flight) < 3 {
		return nil
	}
	t, ok := s.Tenants[flight[:2]]
	if !ok {
		return nil
	}
	sum, ok := t.Summarise(flight, board)
	if !ok {
		return nil
	}
	for _, f := range s.Flights[flight[:2]] {
		if f.Carrier+f.Number == flight && f.From == board {
			sum.Delay, sum.CrewDuty = fateLines(s.fate.Of(f))
			break
		}
	}
	return sum
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
		// Not one of ours. On a core machine the consoles live on the peer
		// that runs the node; the same ownership map the fleet drill-downs
		// use says where, and the whole page proxies through.
		if s.ConsoleProxy != nil && s.ConsoleProxy(w, r, strings.ToUpper(code)) {
			return
		}
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
// codeshares lists, per marketing carrier, the legs it sells under its own
// code but does not fly, and the teletype addresses of the carriers that
// fly them.
func codeshares(carriers []world.Carrier, flights map[string][]world.Flight) (map[string][]world.Flight, map[string]map[string]string) {
	tty := map[string]string{}
	for _, c := range carriers {
		tty[c.Designator] = c.TTYAddress
	}
	marketed := map[string][]world.Flight{}
	operators := map[string]map[string]string{}
	for _, fs := range flights {
		for _, f := range fs {
			if f.Marketing == "" || f.Marketing == f.Carrier || f.MarketingNumber == "" {
				continue
			}
			marketed[f.Marketing] = append(marketed[f.Marketing], f)
			if operators[f.Marketing] == nil {
				operators[f.Marketing] = map[string]string{}
			}
			operators[f.Marketing][f.Carrier] = tty[f.Carrier]
		}
	}
	return marketed, operators
}

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

// switchAddrFor is the listener a carrier dials: its home switch's shared
// subscriber listener, or its own MATIP listener there.
func (s *Sim) switchAddrFor(c world.Carrier) string {
	sw := s.Switches[homeSwitch(c.Designator, len(s.Switches))]
	if c.Transport == "matip" {
		return sw.Addr("link-" + strings.ToLower(c.Designator))
	}
	return sw.Addr("link-net")
}

// linksUp is how many subscriber links the fabric holds: every switch's
// live peers, less the trunks between them, which are the fabric's own.
func (s *Sim) linksUp() int {
	n := 0
	for _, sw := range s.Switches {
		n += len(sw.LivePeers())
	}
	if t := len(s.Switches) - 1; t > 0 {
		n -= 2 * t
	}
	if n < 0 {
		n = 0
	}
	return n
}

func (s *Sim) waitForLinks(ctx context.Context, want int, within time.Duration) error {
	deadline := time.Now().Add(within)
	for {
		allGDSUp := true
		for _, g := range s.GDSes {
			if !g.up.Load() {
				allGDSUp = false
			}
		}
		if s.linksUp() >= want && allGDSUp {
			return nil
		}
		if time.Now().After(deadline) {
			var live []string
			for _, sw := range s.Switches {
				live = append(live, sw.LivePeers()...)
			}
			return fmt.Errorf("only %d of %d links came up: %v", s.linksUp(), want, live)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// delayFor is one day's operational noise for one flight: how late it goes
// out and how much of that it makes back in the air. Deterministic in the
// flight and the calendar day, so a replayed day delays the same flights the
// same way and two observers of one world agree. The mix is the familiar
// shape of a network's day: most flights on time, a fat middle of minor
// delays, and a thin tail of the ones the arrivals board apologises for.
func delayFor(f world.Flight, day time.Time) (depMin, arrMin int) {
	h := 0
	for _, r := range f.Carrier + f.Number + day.Format("20060102") {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	r := h % 100
	spread := h / 100
	switch {
	case r < 62:
		depMin = 0
	case r < 82:
		depMin = 1 + spread%15
	case r < 94:
		depMin = 16 + spread%30
	case r < 99:
		depMin = 46 + spread%75
	default:
		depMin = 121 + spread%120
	}
	// A late departure recovers a little en route; an on-time one lands on
	// time, because inventing early arrivals would move a flight before its
	// own schedule.
	arrMin = depMin - spread%9
	if arrMin < 0 {
		arrMin = 0
	}
	return depMin, arrMin
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

// indexByFrom lists every carrier's flights by boarding point, for
// connections that cross carriers.
func indexByFrom(flights map[string][]world.Flight) map[string][]world.Flight {
	out := map[string][]world.Flight{}
	for _, fs := range flights {
		for _, f := range fs {
			out[f.From] = append(out[f.From], f)
		}
	}
	return out
}

// interlineFor is the flights of a carrier's interline partners that leave
// from where its own flight lands: what a connecting passenger can be
// through-checked onto. Partners are the carriers whose teletype addresses
// the carrier holds, resolved back to their codes.
func interlineFor(code string, partners map[string][]string, carriers []world.Carrier, byFrom map[string][]world.Flight) func(world.Flight) []world.Flight {
	byAddr := map[string]string{}
	for _, c := range carriers {
		byAddr[c.TTYAddress] = c.Designator
	}
	partner := map[string]bool{}
	for _, a := range partners[code] {
		if p := byAddr[a]; p != "" && p != code {
			partner[p] = true
		}
	}
	if len(partner) == 0 {
		return nil
	}
	return func(f world.Flight) []world.Flight {
		var out []world.Flight
		for _, g := range byFrom[f.To] {
			if partner[g.Carrier] {
				out = append(out, g)
			}
		}
		return out
	}
}

// countryOf is the country an airport is in, for deciding whether a flight
// crosses a border and owes the agency a passenger list.
func (s *Sim) countryOf(iata string) string {
	if a, ok := s.airports[iata]; ok {
		return a.Country
	}
	return ""
}

// aogHash spreads the day's technical failures over the schedule
// deterministically: the same flight goes technical on the same day of
// every run, so a story can be found again.
func aogHash(key string) int {
	h := 0
	for _, r := range key {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}
