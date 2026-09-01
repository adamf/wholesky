package sim

// The world across machines.
//
// One box runs out of honesty before it runs out of sockets: real
// reservations volume is thousands of bookings a minute, and a single
// machine answering all of it is a benchmark, not a network. So the world
// splits along its real seams. A core machine runs the switch and the
// instruments -- the network and the room you watch it from. Each
// distribution system runs on its own machine with its share of demand,
// because bookings happen at the GDS. And the carrier tenants shard across
// region machines, each flying its slice of the day and telling its own
// ground story. Every link between them is the same jetway TCP that
// loopback used, dialled across the private network; the switch cannot tell
// the difference, which is the point.
//
// Federation is deliberately dumb: peers register with the core over HTTP
// and heartbeat every few seconds; the response carries the switch's link
// addresses, the current warp and the closed-airport set, so time control
// and chaos propagate on the same beat that liveness does. The core's fleet
// page merges every peer's rows and proxies drill-downs to whichever
// machine owns the node. Nothing here elects anything: the core is the
// core, and a peer that stops calling home simply goes dark on the board.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/api"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/queue"
	jstore "github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/eye"
	"github.com/adamf/wholesky/internal/fleet"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// ShardOf assigns a carrier to one of n region shards, stably: every
// machine computes the same split from the manifest alone.
func ShardOf(designator string, n int) int {
	if n <= 1 {
		return 0
	}
	h := 0
	for _, r := range designator {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h % n
}

// registration is what a peer tells the core, and heartbeats thereafter.
type registration struct {
	Role string `json:"role"` // "gds" or "region"
	Name string `json:"name"` // "1G", "region0"
	// URL is where the core reaches this peer's HTTP: fleet rows, summaries,
	// drill-downs.
	URL string `json:"url"`
}

// welcome is the core's reply: everything a peer needs to join and to stay
// in step.
type welcome struct {
	// SwitchAddr is the shared subscriber listener, ready to dial.
	SwitchAddr string `json:"switch_addr"`
	// MATIPAddrs maps each MATIP carrier to its own listener.
	MATIPAddrs map[string]string `json:"matip_addrs"`
	Warp       int               `json:"warp"`
	Closed     []string          `json:"closed"`
}

// shardSummary is the compact state a peer serves at /shard/summary.json for
// the core's instruments.
type shardSummary struct {
	Bookings int64            `json:"bookings"`
	Queues   map[string]int   `json:"queues"`
	Halos    map[string]int64 `json:"halos"`
}

// Core is a running core machine: the switch, the instruments, and the
// federation registry.
type Core struct {
	Sim *Sim // switch, Eye, fleet, stats; no tenants, no GDSes

	advertise    string
	mu           sync.Mutex
	peers        map[string]registration
	seen         map[string]time.Time
	nodeOwner    map[string]string // node code -> peer URL, from the last merge
	latestQueues map[string]int
}

// BootCore stands up the switch and the room you watch it from.
func BootCore(ctx context.Context, m *world.Manifest, opts Options, advertise string) (*Core, error) {
	if opts.LinkBind == "" {
		opts.LinkBind = "::"
	}
	opts.GDSCount = 0 // GDSes live on their own machines
	s, err := bootBase(ctx, m, opts, true)
	if err != nil {
		return nil, err
	}
	c := &Core{Sim: s, advertise: advertise,
		peers: map[string]registration{}, seen: map[string]time.Time{},
		nodeOwner: map[string]string{}}

	// The core flies its globe from its own switch: every movement crosses
	// it, whoever it was addressed to.
	go s.tapBus(ctx, s.Switch.Bus, func(ev gateway.Event) {
		if ev.Type != gateway.EvMovement {
			return
		}
		if p, ok := ev.Data.(map[string]any); ok {
			s.Movements.Add(1)
			s.Stats.OnMovement()
			s.Eye.OnMovement(p)
		}
	})

	// Hubs for the logical web: every GDS slot that could register.
	for _, slot := range gdsSlots {
		s.Eye.Hubs = append(s.Eye.Hubs, slot.Designator)
	}
	s.Eye.FlightPNRs = c.federatedFlightRecords

	fed := http.NewServeMux()
	c.Routes(fed)
	s.SetFederationHandler(fed)

	s.Fleet.Remotes = c.remoteFleets
	s.Fleet.Owner = c.ownerOf
	s.Fleet.OnOwners = c.SetOwners
	s.Stats.QueueDepths = c.federatedQueues
	go c.pollSummaries(ctx)
	return c, nil
}

// Routes mounts the federation surface onto the core's console mux.
func (c *Core) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /federation/register", c.register)
}

func (c *Core) register(w http.ResponseWriter, r *http.Request) {
	var reg registration
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&reg); err != nil {
		http.Error(w, "malformed registration", http.StatusBadRequest)
		return
	}
	if reg.Name == "" || reg.URL == "" {
		http.Error(w, "a peer needs a name and a url", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	c.peers[reg.Name] = reg
	c.seen[reg.Name] = time.Now()
	c.mu.Unlock()

	c.Sim.closedMu.Lock()
	closed := make([]string, 0, len(c.Sim.closed))
	for a := range c.Sim.closed {
		closed = append(closed, a)
	}
	c.Sim.closedMu.Unlock()
	sort.Strings(closed)

	wl := welcome{
		SwitchAddr: rehost(c.Sim.Switch.Addr("link-net"), c.advertise),
		MATIPAddrs: map[string]string{},
		Warp:       c.Sim.clock.Warp(),
		Closed:     closed,
	}
	for _, cr := range c.Sim.Manifest.Carriers {
		if cr.Transport == "matip" {
			wl.MATIPAddrs[cr.Designator] = rehost(
				c.Sim.Switch.Addr("link-"+strings.ToLower(cr.Designator)), c.advertise)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wl) //nolint:errcheck
}

// rehost swaps a listener's bound host ("::" or loopback) for the address
// peers can actually dial.
func rehost(addr, advertise string) string {
	if advertise == "" {
		return addr
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return net.JoinHostPort(advertise, port)
}

// livePeers returns the peers heard from recently.
func (c *Core) livePeers() []registration {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []registration
	for name, reg := range c.peers {
		if time.Since(c.seen[name]) < 30*time.Second {
			out = append(out, reg)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (c *Core) remoteFleets() []fleet.Remote {
	var out []fleet.Remote
	for _, p := range c.livePeers() {
		out = append(out, fleet.Remote{Name: p.Name, URL: p.URL})
	}
	return out
}

// ownerOf answers which peer's machine holds a node, for drill-down proxying.
func (c *Core) ownerOf(code string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nodeOwner[code]
}

// SetOwners records the code->peer map the fleet merge discovered.
func (c *Core) SetOwners(m map[string]string) {
	c.mu.Lock()
	c.nodeOwner = m
	c.mu.Unlock()
}

// pollSummaries keeps the core's instruments fed with the state that lives
// on other machines: booking totals, queue depths, halo counts.
func (c *Core) pollSummaries(ctx context.Context) {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		var bookings int64
		queues := map[string]int{}
		halos := map[string]int64{}
		for _, p := range c.livePeers() {
			resp, err := client.Get(p.URL + "/shard/summary.json")
			if err != nil {
				continue
			}
			var sum shardSummary
			err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&sum)
			resp.Body.Close()
			if err != nil {
				continue
			}
			bookings += sum.Bookings
			for k, v := range sum.Queues {
				queues[k] += v
			}
			for k, v := range sum.Halos {
				halos[k] += v
			}
		}
		c.Sim.Eye.SetBookings(bookings)
		c.Sim.Eye.SetHaloCounts(halos)
		c.mu.Lock()
		c.latestQueues = queues
		c.mu.Unlock()
	}
}

func (c *Core) federatedQueues() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]int{}
	for k, v := range c.latestQueues {
		out[k] = v
	}
	return out
}

// federatedFlightRecords fans the globe's drill-through out to every GDS
// machine and merges what they hold.
func (c *Core) federatedFlightRecords(flight string) []eye.FlightRecord {
	client := &http.Client{Timeout: 2 * time.Second}
	var out []eye.FlightRecord
	for _, p := range c.livePeers() {
		if p.Role != "gds" {
			continue
		}
		resp, err := client.Get(p.URL + "/shard/flight/" + flight)
		if err != nil {
			continue
		}
		var recs []eye.FlightRecord
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&recs)
		resp.Body.Close()
		if err != nil {
			continue
		}
		out = append(out, recs...)
		if len(out) >= 200 {
			break
		}
	}
	return out
}

// federate registers with the core and heartbeats forever, applying the
// warp and closed-airport state each beat carries. One mechanism is the
// whole control plane: liveness, time control and chaos propagate on the
// same pulse.
func federate(ctx context.Context, s *Sim, coreURL string, reg registration,
	onWelcome func(welcome), log *slog.Logger) (welcome, error) {

	client := &http.Client{Timeout: 4 * time.Second}
	call := func() (welcome, error) {
		body, _ := json.Marshal(reg)
		resp, err := client.Post(coreURL+"/federation/register", "application/json",
			bytes.NewReader(body))
		if err != nil {
			return welcome{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return welcome{}, fmt.Errorf("core refused registration: %s: %s", resp.Status, b)
		}
		var wl welcome
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&wl); err != nil {
			return welcome{}, err
		}
		return wl, nil
	}

	// The first registration must succeed before anything dials a link;
	// the core may still be booting, so patience.
	var first welcome
	deadline := time.Now().Add(2 * time.Minute)
	for {
		wl, err := call()
		if err == nil {
			first = wl
			break
		}
		if time.Now().After(deadline) {
			return welcome{}, fmt.Errorf("the core never answered: %w", err)
		}
		log.Info("waiting for the core", "core", coreURL, "err", err)
		select {
		case <-ctx.Done():
			return welcome{}, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	apply := func(wl welcome) {
		if s.clock.Warp() != wl.Warp {
			s.clock.SetWarp(time.Now(), wl.Warp)
		}
		nowClosed := map[string]bool{}
		for _, a := range wl.Closed {
			nowClosed[a] = true
		}
		s.closedMu.Lock()
		var opened, shut []string
		for a := range s.closed {
			if !nowClosed[a] {
				opened = append(opened, a)
			}
		}
		for a := range nowClosed {
			if !s.closed[a] {
				shut = append(shut, a)
			}
		}
		s.closed = nowClosed
		s.closedMu.Unlock()
		for _, a := range shut {
			log.Info("following core chaos: airport closed", "airport", a)
			go s.cancelFlightsTouching(a)
		}
		for _, a := range opened {
			log.Info("following core chaos: airport reopened", "airport", a)
		}
		if onWelcome != nil {
			onWelcome(wl)
		}
	}
	apply(first)

	go func() {
		tick := time.NewTicker(5 * time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if wl, err := call(); err == nil {
					apply(wl)
				}
			}
		}
	}()
	return first, nil
}

// shardRoutes serves the compact state the core's instruments poll, plus the
// full local fleet endpoints the core proxies drill-downs to.
func shardRoutes(mux *http.ServeMux, s *Sim, bookings func() int64) {
	s.Fleet.Routes(mux)
	mux.HandleFunc("/node/", s.serveNodeConsole)
	mux.HandleFunc("GET /shard/summary.json", func(w http.ResponseWriter, r *http.Request) {
		sum := shardSummary{Queues: map[string]int{}, Halos: map[string]int64{}}
		if bookings != nil {
			sum.Bookings = bookings()
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, g := range s.GDSes {
			items, err := g.Store.ListQueue(ctx, jstore.QueueFilter{})
			if err != nil {
				continue
			}
			for _, it := range items {
				sum.Queues[it.Queue]++
				if it.Queue == string(jstore.QueueScheduleChange) {
					if apt := airportOfReason(it.Reason); apt != "" {
						sum.Halos[apt]++
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sum) //nolint:errcheck
	})
	mux.HandleFunc("GET /shard/flight/{flight}", func(w http.ResponseWriter, r *http.Request) {
		recs := s.flightRecords(strings.ToUpper(r.PathValue("flight")))
		if recs == nil {
			recs = []eye.FlightRecord{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(recs) //nolint:errcheck
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	})
}

// airportOfReason digs the board point out of a schedule-change reason line,
// e.g. "BA0117 Y 16DEC LHR-JFK ...": the fourth field's first half.
func airportOfReason(reason string) string {
	f := strings.Fields(reason)
	if len(f) < 4 {
		return ""
	}
	if i := strings.IndexByte(f[3], '-'); i == 3 {
		return f[3][:3]
	}
	return ""
}

// distributionAddresses maps GDS designators to their teletype addresses.
// An empty list means every slot -- the single-box default -- but a
// deployment running three channels must not have its tenants broadcasting
// availability at two that do not exist: every such message is an
// undeliverable the switch retries forever.
func distributionAddresses(list []string) []string {
	if len(list) == 0 {
		for _, slot := range gdsSlots {
			list = append(list, slot.Designator)
		}
	}
	var out []string
	for _, d := range list {
		for _, slot := range gdsSlots {
			if slot.Designator == d {
				out = append(out, gdsAddress(slot))
			}
		}
	}
	return out
}

// GDSMachine is one distribution system on a machine of its own.
type GDSMachine struct {
	Sim  *Sim
	Node *GDSNode
	Mux  *http.ServeMux
}

// BootGDS stands up one distribution system that dials the core's switch,
// runs its share of demand, and serves its console and shard state.
func BootGDS(ctx context.Context, m *world.Manifest, opts Options,
	coreURL, selfURL, designator string) (*GDSMachine, error) {

	opts.NoGDS = true // bootBase builds none; this machine builds its own one
	s, err := bootBase(ctx, m, opts, false)
	if err != nil {
		return nil, err
	}
	var slot *struct{ Designator, City, Name string }
	for i := range gdsSlots {
		if gdsSlots[i].Designator == designator {
			slot = &gdsSlots[i]
		}
	}
	if slot == nil {
		return nil, fmt.Errorf("no GDS slot %q", designator)
	}

	wl, err := federate(ctx, s, coreURL,
		registration{Role: "gds", Name: designator, URL: selfURL}, nil, s.log)
	if err != nil {
		s.Stop()
		return nil, err
	}

	g := &GDSNode{Designator: slot.Designator, Address: gdsAddress(*slot), Name: slot.Name}
	s.GDSes = []*GDSNode{g}
	s.Eye.Hubs = []string{designator}
	if err := s.buildGDSNode(ctx, m, g, wl.SwitchAddr, opts.GDSDSN, false, s.log); err != nil {
		s.Stop()
		return nil, err
	}
	s.GDS, s.GDSStore = g.GW, g.Store
	s.Fleet.Add(ctx, g.Designator, g.Name, fleet.KindGDS, "", "", "", 0, g.Store, nil)
	sweeper := &queue.Sweeper{
		Records: g.Store, Queues: g.GW.Queues,
		Log: s.log.With("node", strings.ToLower(g.Designator)), Cancel: g.GW,
	}
	go sweeper.Run(ctx, 30*time.Second)
	s.mountConsole(g.Designator, &api.Server{
		Gateway: g.GW, Store: g.Store, Bus: g.Bus,
		Log: s.log.With("console", g.Designator), Console: true,
	})

	mux := http.NewServeMux()
	shardRoutes(mux, s, func() int64 { return s.DemBooked.Load() })
	return &GDSMachine{Sim: s, Node: g, Mux: mux}, nil
}

// RegionMachine is one shard of the carrier world.
type RegionMachine struct {
	Sim *Sim
	Mux *http.ServeMux
}

// BootRegion stands up this shard's tenants, dialling the core's switch, and
// flies their slice of the day.
func BootRegion(ctx context.Context, m *world.Manifest, opts Options,
	coreURL, selfURL string, shard, shards int) (*RegionMachine, error) {

	opts.NoGDS = true
	s, err := bootBase(ctx, m, opts, false)
	if err != nil {
		return nil, err
	}
	name := fmt.Sprintf("region%d", shard)
	wl, err := federate(ctx, s, coreURL,
		registration{Role: "region", Name: name, URL: selfURL}, nil, s.log)
	if err != nil {
		s.Stop()
		return nil, err
	}

	capacity := opts.Capacity
	if capacity <= 0 {
		capacity = 100000
	}
	partners := partnerAddresses(m.Carriers, s.Flights)
	distribution := distributionAddresses(opts.GDSList)
	mine := 0
	for _, c := range m.Carriers {
		if ShardOf(c.Designator, shards) != shard {
			continue
		}
		mine++
		tenantBus := gateway.NewBus(64)
		switchAddr := wl.SwitchAddr
		if c.Transport == "matip" {
			addr, ok := wl.MATIPAddrs[c.Designator]
			if !ok {
				s.Stop()
				return nil, fmt.Errorf("the core offers no MATIP listener for %s", c.Designator)
			}
			switchAddr = addr
		}
		tenantMaxMsgs, tenantMaxRecs := opts.TenantMaxMessages, opts.TenantMaxRecords
		if tenantMaxMsgs == 0 {
			tenantMaxMsgs = opts.MaxMessages
		}
		if tenantMaxRecs == 0 {
			tenantMaxRecs = opts.MaxRecords
		}
		t, err := host.Start(ctx, c, s.Flights[c.Designator], host.Options{
			SwitchAddr:            switchAddr,
			WatchAddress:          GDSAddress,
			DistributionAddresses: distribution,
			PartnerAddresses:      partners[c.Designator],
			Capacity:              capacity,
			BookingDate:           s.BookingDate,
			MaxMessages:           tenantMaxMsgs,
			MaxRecords:            tenantMaxRecs,
			AVSInterval:           opts.AVSInterval,
			Bus:                   tenantBus,
			Log:                   s.log,
		})
		if err != nil {
			s.Stop()
			return nil, fmt.Errorf("start carrier %s: %w", c.Designator, err)
		}
		s.Tenants[c.Designator] = t
		s.Fleet.Add(ctx, c.Designator, c.Name, fleet.KindCarrier,
			c.Format, c.Transport, c.Hub, len(s.Flights[c.Designator]), t.Store, tenantBus)
		s.mountConsole(c.Designator, &api.Server{
			Gateway: t.Gateway, Store: t.Store, Bus: tenantBus,
			Log: s.log.With("console", c.Designator), Console: true,
		})
	}
	// The board's link dots and sever buttons for this shard's rows come
	// from the tenants themselves: a region has no switch to count sessions,
	// but it knows which circuits it has deliberately cut.
	s.Fleet.LivePeers = func() []string {
		var up []string
		for code, t := range s.Tenants {
			if !t.Severed() {
				up = append(up, code)
			}
		}
		return up
	}
	s.Fleet.LinkControl = s.LinkControl
	s.log.Info("region up", "shard", shard, "of", shards, "carriers", mine)

	mux := http.NewServeMux()
	shardRoutes(mux, s, nil)
	return &RegionMachine{Sim: s, Mux: mux}, nil
}
