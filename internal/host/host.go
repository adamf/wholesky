// Package host runs carrier reservation systems as tenants.
//
// One process, many carriers: each tenant is a complete Jetway gateway --
// identity, inventory, PNR store -- holding one real TCP link to the message
// switch and reaching everything else by teletype address through it. That is
// not an efficiency hack. It is how the industry is actually shaped: most of
// the world's carriers are tenants of a handful of hosted reservation systems,
// and the ones that are not still hold one circuit to the network, not one per
// partner.
package host

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/avs"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/matip"
	"github.com/adamf/jetway/pkg/mvt"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"
	"github.com/adamf/jetway/pkg/typeb"

	"github.com/adamf/wholesky/internal/world"
)

// Tenant is one carrier's running system.
// link is the shape both circuit clients share: the plain framed transport
// and MATIP. A tenant runs whichever its carrier's circuit uses.
type link interface {
	Run(ctx context.Context) error
	Send(ctx context.Context, peer string, raw []byte) error
}

type Tenant struct {
	Carrier   world.Carrier
	Gateway   *gateway.Gateway
	Store     store.Store
	Inventory *gateway.Inventory

	flights      []world.Flight
	client       link
	watch        string // teletype address operational messages are copied to
	distribution []string
	partners     []string
	log          *slog.Logger

	// The link runs under its own sub-context so chaos can cut it without
	// touching the tenant. bootCtx is what a restored link derives from.
	linkMu     sync.Mutex
	bootCtx    context.Context
	linkCancel context.CancelFunc
	severed    bool
}

// Options configure a tenant.
type Options struct {
	// SwitchAddr is the tenant's listener on the message switch.
	SwitchAddr string
	// DistributionAddresses are every distribution system's teletype
	// addresses. Availability and schedule traffic goes to all of them --
	// a seat count or a cancellation only one channel hears about is a
	// divergence by construction. Empty falls back to WatchAddress alone.
	DistributionAddresses []string
	// WatchAddress receives this tenant's operational traffic -- availability
	// and movements -- the way a network's ops centre address does.
	WatchAddress string
	// PartnerAddresses are the teletype addresses of this carrier's interline
	// partners. Availability is broadcast to them as well as to the watch:
	// free sale between partners is the whole point of AVS, and a partner who
	// never hears your availability is not a partner.
	PartnerAddresses []string
	// Capacity is seats per class per flight. The load-suite lesson applies:
	// a demonstration wants single digits, a simulation wants headroom.
	Capacity int
	// BookingDate is the date the world sells, for availability broadcasts.
	BookingDate time.Time
	// MaxMessages and MaxRecords bound the tenant's store; zero is unbounded.
	MaxMessages int
	MaxRecords  int
	// AVSInterval is how often availability is rebroadcast. Zero uses the
	// default; a planet-sized deployment breathes slower than a demo.
	AVSInterval time.Duration
	Log         *slog.Logger
	Bus         *gateway.Bus
}

// Start brings one carrier up and dials its link.
func Start(ctx context.Context, c world.Carrier, flights []world.Flight, opts Options) (*Tenant, error) {
	if opts.SwitchAddr == "" {
		return nil, fmt.Errorf("host: carrier %s has no switch address", c.Designator)
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	bus := opts.Bus
	if bus == nil {
		bus = gateway.NewBus(64)
	}

	st := store.NewMem()
	st.MaxMessages, st.MaxRecords = opts.MaxMessages, opts.MaxRecords
	gw := gateway.New(gateway.Identity{
		Designator: c.Designator,
		TTYAddress: c.TTYAddress,
		Name:       c.Name,
	}, st, bus, log.With("carrier", c.Designator), []byte("wholesky-"+c.Designator))

	inv := gateway.NewInventory()
	inv.Carrier = c.Designator
	if opts.Capacity > 0 {
		inv.Capacity = opts.Capacity
	}
	gw.Responder = inv

	// The switch identifies this tenant by the listener it dialled, the way
	// a real circuit identifies its subscriber. Which client dials depends on
	// the carrier's circuit: most use the plain framed link, and a share of
	// the teletype world uses MATIP, the airline transport.
	var client link
	onMessage := func(ctx context.Context, raw []byte) error {
		_, err := gw.Ingest(ctx, "net", raw)
		return err
	}
	if c.Transport == "matip" {
		client = &matip.Client{
			Addr: opts.SwitchAddr,
			Log:  log.With("carrier", c.Designator),
			OnMessage: func(ctx context.Context, raw []byte) error {
				return onMessage(ctx, raw)
			},
		}
	} else {
		client = &transport.Client{
			Addr:      opts.SwitchAddr,
			Framer:    transport.DefaultFramer(),
			Log:       log.With("carrier", c.Designator),
			SkipHello: true,
			OnMessage: func(ctx context.Context, peer string, raw []byte) error {
				return onMessage(ctx, raw)
			},
		}
	}
	gw.Sender = client

	// From the tenant's side the peer is the network. Whoever a message is
	// really from is read off its origin line; the network is just the wire.
	// The peer's format is this carrier's own dialect, because it is what
	// replies built for traffic arriving over the network are encoded in.
	format := store.FormatTypeB
	if c.Format == "edifact" {
		format = store.FormatEDIFACT
	}
	gw.AddPeer(&gateway.Peer{
		Name: "net", Format: format, TTYAddress: opts.WatchAddress,
	})
	t := &Tenant{
		Carrier: c, Gateway: gw, Store: st, Inventory: inv,
		flights: flights, client: client, watch: opts.WatchAddress,
		distribution: opts.DistributionAddresses,
		partners:     opts.PartnerAddresses, log: log, bootCtx: ctx,
	}
	linkCtx, linkCancel := context.WithCancel(ctx)
	t.linkCancel = linkCancel
	go t.runLink(linkCtx)
	interval := opts.AVSInterval
	if interval <= 0 {
		interval = availabilityInterval
	}
	go t.broadcastAvailability(ctx, opts.BookingDate, interval, log)
	return t, nil
}

// runLink keeps the switch link up until its sub-context is cancelled.
func (t *Tenant) runLink(ctx context.Context) {
	if err := t.client.Run(ctx); err != nil && ctx.Err() == nil {
		t.log.Error("carrier link ended", "carrier", t.Carrier.Designator, "err", err)
	}
}

// Sever cuts the tenant's link to the switch and keeps it down until Restore.
// The TCP connection really closes: the tenant's sends start failing, the
// switch loses the session and marks its own sends undeliverable, and every
// dashboard sees exactly what a cut circuit looks like -- because that is
// what this is.
func (t *Tenant) Sever() {
	t.linkMu.Lock()
	defer t.linkMu.Unlock()
	if t.severed {
		return
	}
	t.severed = true
	t.linkCancel()
}

// Restore dials the switch again after a Sever. The reconnect goes through
// the same client and the same backoff as a real circuit repair.
func (t *Tenant) Restore() {
	t.linkMu.Lock()
	defer t.linkMu.Unlock()
	if !t.severed {
		return
	}
	t.severed = false
	linkCtx, cancel := context.WithCancel(t.bootCtx)
	t.linkCancel = cancel
	go t.runLink(linkCtx)
}

// Severed reports whether the link is deliberately down.
func (t *Tenant) Severed() bool {
	t.linkMu.Lock()
	defer t.linkMu.Unlock()
	return t.severed
}

// availabilityInterval is how often a tenant re-broadcasts.
//
// Change-triggered broadcasting is the realistic model and the roadmap; the
// timer keeps phase one honest about staleness instead -- the cache's rule
// that an old assertion never moves state backwards gets exercised on every
// cycle.
const availabilityInterval = 30 * time.Second

func (t *Tenant) broadcastAvailability(ctx context.Context, date time.Time, interval time.Duration, log *slog.Logger) {
	// A Type B message holds sixty lines and four kilobytes, and a carrier
	// with a thousand flights holds neither. Availability goes out in chunks
	// of flight-dates, each chunk one legal message -- which is also how the
	// real network carries it: nobody broadcasts an airline in one telex.
	const flightDatesPerMessage = 24

	send := func() {
		var groups [][]avail.Key
		for d := -1; d <= 1; d++ {
			day := date.AddDate(0, 0, d)
			for _, f := range t.flights {
				var g []avail.Key
				for _, cls := range []string{"F", "J", "Y", "M"} {
					g = append(g, avail.NewKey(f.Carrier, f.Number, day, f.From, f.To, cls))
				}
				groups = append(groups, g)
			}
		}
		if len(groups) == 0 {
			return
		}
		sent, failed := 0, 0
		for i := 0; i < len(groups); i += flightDatesPerMessage {
			end := i + flightDatesPerMessage
			if end > len(groups) {
				end = len(groups)
			}
			var keys []avail.Key
			for _, g := range groups[i:end] {
				keys = append(keys, g...)
			}
			entries := t.Inventory.Availability(keys, time.Now().UTC())
			if len(entries) == 0 {
				continue
			}
			if err := t.sendTypeBTo(ctx, append(t.distributionList(), t.partners...), avs.Build(entries), "AVS"); err != nil {
				failed++
			} else {
				sent++
			}
			if ctx.Err() != nil {
				return
			}
		}
		if failed > 0 {
			// Loud, not debug: an undelivered broadcast is a carrier the
			// network has stopped hearing from, and it hid at debug level
			// once already.
			log.Warn("availability broadcast partly undelivered",
				"carrier", t.Carrier.Designator, "sent", sent, "failed", failed)
		}
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Second):
		send()
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			send()
		}
	}
}

// Depart emits the movement message for a flight leaving now.
func (t *Tenant) Depart(ctx context.Context, f world.Flight, day time.Time, reg string, delayMin int) error {
	dep := day.Add(time.Duration(f.DepMin+delayMin) * time.Minute)
	off := dep.Add(12 * time.Minute)
	eta := day.Add(time.Duration(f.ArrMin+delayMin) * time.Minute)
	m := &mvt.Message{
		Kind:         mvt.KindMVT,
		Flight:       f.Carrier + strings.TrimLeft(f.Number, "0"),
		Day:          fmt.Sprintf("%02d", day.Day()),
		Registration: reg,
		Station:      f.From,
		AD:           &mvt.TimePair{First: dep.Format("1504"), Second: off.Format("1504")},
		EA:           &mvt.ETA{Time: eta.Format("1504"), Airport: f.To},
		Pax:          []int{f.Seats * 8 / 10},
	}
	if delayMin > 0 {
		m.Delays = []mvt.Delay{{Code: "93", Duration: fmt.Sprintf("%02d%02d", delayMin/60, delayMin%60)}}
	}
	text, err := m.Build()
	if err != nil {
		return err
	}
	return t.sendTypeB(ctx, text, "MVT")
}

// Arrive emits the movement message for a flight landing now.
func (t *Tenant) Arrive(ctx context.Context, f world.Flight, day time.Time, reg string, delayMin int) error {
	on := day.Add(time.Duration(f.ArrMin+delayMin) * time.Minute)
	m := &mvt.Message{
		Kind:         mvt.KindMVT,
		Flight:       f.Carrier + strings.TrimLeft(f.Number, "0"),
		Day:          fmt.Sprintf("%02d", day.Day()),
		Registration: reg,
		Station:      f.To,
		AA:           &mvt.TimePair{First: on.Add(-8 * time.Minute).Format("1504"), Second: on.Format("1504")},
		FLD:          fmt.Sprintf("%02d", day.Day()),
	}
	text, err := m.Build()
	if err != nil {
		return err
	}
	return t.sendTypeB(ctx, text, "MVT")
}

// SendOps transmits an operational message this tenant authored -- a
// diversion, a delay revision -- built by the caller.
func (t *Tenant) SendOps(ctx context.Context, m *mvt.Message) error {
	text, err := m.Build()
	if err != nil {
		return err
	}
	return t.sendTypeB(ctx, text, string(m.Kind))
}

// SendSchedule transmits a schedule message -- an ASM cancellation, a
// retiming -- to the network, the way this carrier's schedule bureau would.
func (t *Tenant) SendSchedule(ctx context.Context, text string) error {
	// A schedule change only some channels hear about strands the others'
	// bookings; it goes to every distribution system.
	return t.sendTypeBTo(ctx, t.distributionList(), text, "ASM")
}

// distributionList is every distribution address, or the watcher alone when
// none were configured.
func (t *Tenant) distributionList() []string {
	if len(t.distribution) > 0 {
		return append([]string(nil), t.distribution...)
	}
	return []string{t.watch}
}

func (t *Tenant) sendTypeB(ctx context.Context, text, kind string) error {
	return t.sendTypeBTo(ctx, []string{t.watch}, text, kind)
}

// sendTypeBTo transmits one message to several addressees. One message: the
// address block carries them all, and the switch fans it out, which is what
// the address block is for.
func (t *Tenant) sendTypeBTo(ctx context.Context, addrs []string, text, kind string) error {
	dests := make([]typeb.Address, 0, len(addrs))
	for _, a := range addrs {
		d, err := typeb.ParseAddress(a)
		if err != nil {
			return fmt.Errorf("host: address %q: %w", a, err)
		}
		dests = append(dests, d)
	}
	origin, err := typeb.ParseAddress(t.Carrier.TTYAddress)
	if err != nil {
		return fmt.Errorf("host: own address: %w", err)
	}
	now := time.Now().UTC()
	out := &typeb.Message{
		Priority:     "QU",
		Destinations: dests,
		Origin:       origin,
		OriginTime:   typeb.OriginTime{Present: true, Day: now.Day(), Hour: now.Hour(), Minute: now.Minute()},
		Text:         text,
	}
	raw, err := out.Encode(typeb.EncodeOptions{Charset: typeb.CharsetITA2, CRLF: true})
	if err != nil {
		return err
	}
	peer := t.Gateway.Peer("net")
	_, err = t.Gateway.Send(ctx, peer, raw, kind, "", "")
	return err
}
