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
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/avs"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/mvt"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"
	"github.com/adamf/jetway/pkg/typeb"

	"github.com/adamf/wholesky/internal/world"
)

// Tenant is one carrier's running system.
type Tenant struct {
	Carrier   world.Carrier
	Gateway   *gateway.Gateway
	Store     store.Store
	Inventory *gateway.Inventory

	flights []world.Flight
	client  *transport.Client
	watch   string // teletype address operational messages are copied to
}

// Options configure a tenant.
type Options struct {
	// SwitchAddr is the tenant's listener on the message switch.
	SwitchAddr string
	// WatchAddress receives this tenant's operational traffic -- availability
	// and movements -- the way a network's ops centre address does.
	WatchAddress string
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

	client := &transport.Client{
		Addr:   opts.SwitchAddr,
		Framer: transport.DefaultFramer(),
		Log:    log.With("carrier", c.Designator),
		// The switch identifies this tenant by the listener it dialled, the
		// way a real circuit identifies its subscriber.
		SkipHello: true,
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
	client.OnMessage = func(ctx context.Context, peer string, raw []byte) error {
		_, err := gw.Ingest(ctx, "net", raw)
		return err
	}

	t := &Tenant{
		Carrier: c, Gateway: gw, Store: st, Inventory: inv,
		flights: flights, client: client, watch: opts.WatchAddress,
	}
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("carrier link ended", "carrier", c.Designator, "err", err)
		}
	}()
	interval := opts.AVSInterval
	if interval <= 0 {
		interval = availabilityInterval
	}
	go t.broadcastAvailability(ctx, opts.BookingDate, interval, log)
	return t, nil
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
			if err := t.sendTypeB(ctx, avs.Build(entries), "AVS"); err != nil {
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
	return t.sendTypeB(ctx, text, "ASM")
}

func (t *Tenant) sendTypeB(ctx context.Context, text, kind string) error {
	dest, err := typeb.ParseAddress(t.watch)
	if err != nil {
		return fmt.Errorf("host: watch address: %w", err)
	}
	origin, err := typeb.ParseAddress(t.Carrier.TTYAddress)
	if err != nil {
		return fmt.Errorf("host: own address: %w", err)
	}
	now := time.Now().UTC()
	out := &typeb.Message{
		Priority:     "QU",
		Destinations: []typeb.Address{dest},
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
