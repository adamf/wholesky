package sim

// The two networks beside the airlines' own. A datalink service provider
// sits between the aircraft and the airline: the aircraft reports OUT, OFF,
// ON and IN over the air and the provider forwards each report to the
// airline's operations desk over Type B, where the movement message is
// derived from it. An air navigation service provider runs the aeronautical
// fixed network: airlines file flight plans with it, and its towers send
// departure and arrival messages when they see the aircraft move. Both are
// jetway nodes with one link each to the switch, like everyone else.

import (
	"context"
	"fmt"
	"github.com/adamf/jetway/pkg/padis"
	"github.com/adamf/jetway/pkg/paxlst"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adamf/jetway/pkg/acars"
	"github.com/adamf/jetway/pkg/aftn"
	"github.com/adamf/jetway/pkg/ats"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/transport"
	"github.com/adamf/jetway/pkg/typeb"

	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/world"
)

// Network peers at the switch, per region shard: two providers and two
// service providers, because a region dials its own instance and two
// sessions under one name would fight over the link.
const networkShards = 2

// dspPeer and atcPeer name the switch's links for a shard.
func dspPeer(shard int) string { return fmt.Sprintf("DSP%d", shard) }
func atcPeer(shard int) string { return fmt.Sprintf("ATC%d", shard) }

// dspAddress and atcAddress are the providers' Type B addresses: a datalink
// provider's communications centre and the ANSP's message switch.
func dspAddress(shard int) string { return fmt.Sprintf("LONDL%dS", shard) }
func atcAddress(shard int) string { return fmt.Sprintf("LONAT%dZ", shard) }

// govPeer and govAddress name the border control agency's link and address:
// where every international departure's passenger list goes.
func govPeer(shard int) string    { return fmt.Sprintf("GOV%d", shard) }
func govAddress(shard int) string { return fmt.Sprintf("LONGV%dX", shard) }

// Border is the border control agency: it receives the advance passenger
// information every international flight sends at the door, and counts what
// it was told, by the country the flight arrives in.
type Border struct {
	*netNode
	lists   atomic.Int64
	persons atomic.Int64
	pushes  atomic.Int64
	records atomic.Int64
	checked atomic.Int64
	mu      sync.Mutex
	byDest  map[string]int64
}

// Pushes is how many PNR pushes arrived; Records how many records they
// carried; CheckedIn how many of those records came with check-in data.
func (b *Border) Pushes() int64    { return b.pushes.Load() }
func (b *Border) Records() int64   { return b.records.Load() }
func (b *Border) CheckedIn() int64 { return b.checked.Load() }

func (b *Border) receivePush(ctx context.Context, peer *gateway.Peer, push *padis.GovPush) {
	b.pushes.Add(1)
	b.records.Add(int64(len(push.Records)))
	for _, r := range push.Records {
		if len(r.CheckIn) > 0 {
			b.checked.Add(1)
		}
	}
}

// Lists is how many passenger lists arrived; Persons how many travellers
// they named.
func (b *Border) Lists() int64   { return b.lists.Load() }
func (b *Border) Persons() int64 { return b.persons.Load() }

// ByArrival is persons reported, by the airport of first arrival.
func (b *Border) ByArrival() map[string]int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[string]int64, len(b.byDest))
	for k, v := range b.byDest {
		out[k] = v
	}
	return out
}

func (b *Border) receive(ctx context.Context, peer *gateway.Peer, list *paxlst.Message) {
	b.lists.Add(1)
	b.persons.Add(int64(len(list.People)))
	if len(list.Legs) > 0 {
		b.mu.Lock()
		b.byDest[list.Legs[0].To] += int64(len(list.People))
		b.mu.Unlock()
	}
}

// netNode is one provider node: a gateway with a link to the switch.
type netNode struct {
	Name  string
	GW    *gateway.Gateway
	Store store.Store
	Bus   *gateway.Bus
	up    atomic.Bool
	seq   atomic.Int64
	log   *slog.Logger
	icao  func(iata string) string
}

// startNetNode dials the switch as a named provider.
func (s *Sim) startNetNode(ctx context.Context, name, designator, tty, aftnAddr, switchAddr string, ground gateway.Ground, log *slog.Logger) (*netNode, error) {
	st := store.NewMem()
	st.MaxMessages, st.MaxRecords = 2000, 100
	bus := gateway.NewBus(64)
	gw := gateway.New(gateway.Identity{Designator: designator, TTYAddress: tty, AFTNAddress: aftnAddr, Name: name}, st, bus, log, []byte("wholesky-"+name))
	gw.Ground = ground
	client := &transport.Client{
		Addr: switchAddr, Framer: transport.DefaultFramer(),
		Hello: transport.Hello{Peer: name, Role: "network", Format: "typeb"},
		Log:   log,
	}
	gw.Sender = client
	client.OnMessage = func(ctx context.Context, peer string, raw []byte) error {
		_, err := gw.Ingest(ctx, "net", raw)
		return err
	}
	n := &netNode{Name: name, GW: gw, Store: st, Bus: bus, log: log, icao: s.icaoOf}
	client.OnUp = func() { n.up.Store(true) }
	gw.AddPeer(&gateway.Peer{Name: "net", Format: store.FormatTypeB, TTYAddress: "XCHDD1X"})
	go func() {
		if err := client.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("network link ended", "node", name, "err", err)
		}
	}()
	return n, nil
}

// icaoOf is the location indicator of an airport in this world.
func (s *Sim) icaoOf(iata string) string {
	if a, ok := s.airports[iata]; ok {
		return a.ICAO
	}
	return ""
}

// sendTypeB transmits one Type B message from the node's own address.
func (n *netNode) sendTypeB(ctx context.Context, from string, to []string, text, kind string) error {
	dests := make([]typeb.Address, 0, len(to))
	for _, a := range to {
		d, err := typeb.ParseAddress(a)
		if err != nil {
			return err
		}
		dests = append(dests, d)
	}
	origin, err := typeb.ParseAddress(from)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	out := &typeb.Message{
		Priority: "QU", Destinations: dests, Origin: origin,
		OriginTime: typeb.OriginTime{Present: true, Day: now.Day(), Hour: now.Hour(), Minute: now.Minute()},
		Text:       text,
	}
	raw, err := out.Encode(typeb.EncodeOptions{Charset: typeb.CharsetITA2, CRLF: true})
	if err != nil {
		return err
	}
	_, err = n.GW.Send(ctx, n.GW.Peer("net"), raw, kind, "", "")
	return err
}

// Datalink is the provider: the aircraft's reports on their way to the
// airline. OOOI reports carry ICAO indicators and the DT line says which
// ground station heard the aircraft.
type Datalink struct {
	*netNode
	provider string // the DT line's provider code
}

// Report forwards an aircraft's OUT/OFF or ON/IN report to the operating
// carrier's operations desk at the station.
func (d *Datalink) Report(ctx context.Context, c world.Carrier, f world.Flight, reg string, kind acars.Kind, first, second time.Time) error {
	m := &acars.Message{
		Kind: kind, Flight: c.Designator + strings.TrimLeft(f.Number, "0"), Registration: reg,
		Departure: d.icao(f.From),
	}
	station := f.From
	switch kind {
	case acars.KindDEP:
		m.Destination = d.icao(f.To)
		m.Out, m.Off = first.Format("1504"), second.Format("1504")
	case acars.KindARR:
		m.Arrival = d.icao(f.To)
		m.On, m.In = first.Format("1504"), second.Format("1504")
		station = f.To
	}
	m.Service = &acars.Service{Provider: d.provider, Station: station,
		Time: second.Format("021504"), Number: fmt.Sprintf("M%02dA", d.seq.Add(1)%100)}
	text, err := acars.Build(m)
	if err != nil {
		return err
	}
	// The report goes to the airline's operations desk at the station the
	// aircraft is at, from the provider's communications centre.
	return d.sendTypeB(ctx, d.GW.Identity.TTYAddress, []string{station + host.DeptOps + c.Designator}, text, "ACARS/"+string(kind)+"/"+m.Flight)
}

// ANSP is the air navigation service provider: it takes the airlines'
// flight plans and its towers report departures and arrivals over the AFTN.
type ANSP struct {
	*netNode
	plans atomic.Int64
}

// send puts an ATS message in an envelope from a tower and sends it.
func (a *ANSP) send(ctx context.Context, fromICAO string, addressees []string, m *ats.Message) error {
	text, err := ats.Build(m)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	env := &aftn.Message{
		TransmissionID: fmt.Sprintf("ATC%03d", a.seq.Add(1)%1000),
		Priority:       aftn.PrioritySafety, Addressees: addressees,
		FilingTime: now.Format("021504"), Originator: fromICAO + "ZPZX", Text: text,
	}
	raw, err := env.Encode(aftn.EncodeOptions{CRLF: true})
	if err != nil {
		return err
	}
	_, err = a.GW.Send(ctx, a.GW.Peer("net"), raw, "ATS/"+string(m.Type)+"/"+m.AircraftID, "", "")
	return err
}

// Departure is the tower at the departure aerodrome telling the airline
// and the destination that the aircraft is off.
func (a *ANSP) Departure(ctx context.Context, c world.Carrier, f world.Flight, day time.Time, off time.Time) error {
	dep, dest := a.icao(f.From), a.icao(f.To)
	if dep == "" || dest == "" {
		return nil
	}
	m := &ats.Message{Type: ats.TypeDEP, AircraftID: host.Callsign(c, f),
		Departure: dep, EOBT: off.Format("1504"), Destination: dest,
		Other: []ats.Item{{Key: "DOF", Value: day.Format("060102")}}}
	return a.send(ctx, dep, []string{dest + c.ICAO + "X", dest + "ZPZX"}, m)
}

// Arrival is the tower at the arrival aerodrome telling the airline the
// aircraft has landed.
func (a *ANSP) Arrival(ctx context.Context, c world.Carrier, f world.Flight, at string, landed time.Time) error {
	dep, dest := a.icao(f.From), a.icao(f.To)
	arr := a.icao(at)
	if dep == "" || arr == "" {
		return nil
	}
	m := &ats.Message{Type: ats.TypeARR, AircraftID: host.Callsign(c, f),
		Departure: dep, Destination: dest, Arrival: arr, ArrivalTime: landed.Format("1504")}
	return a.send(ctx, arr, []string{dep + c.ICAO + "X"}, m)
}

// Cancellation is the flight plan being cancelled: the airline tells the
// towers, the way a filed plan is withdrawn.
func (a *ANSP) Cancellation(ctx context.Context, c world.Carrier, f world.Flight, day time.Time) error {
	dep, dest := a.icao(f.From), a.icao(f.To)
	if dep == "" || dest == "" {
		return nil
	}
	m := &ats.Message{Type: ats.TypeCNL, AircraftID: host.Callsign(c, f),
		Departure: dep, EOBT: hhmmOf(f.DepMin), Destination: dest,
		Other: []ats.Item{{Key: "DOF", Value: day.Format("060102")}}}
	return a.send(ctx, dep, []string{dest + "ZPZX", dep + c.ICAO + "X"}, m)
}

func hhmmOf(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d%02d", min/60, min%60)
}

// startNetworks stands up this machine's datalink provider and ANSP.
func (s *Sim) startNetworks(ctx context.Context, shard int, switchAddr string, log *slog.Logger) error {
	dsp, err := s.startNetNode(ctx, dspPeer(shard), "XS", dspAddress(shard), "", switchAddr, nil, log.With("node", "dsp"))
	if err != nil {
		return err
	}
	s.DSP = &Datalink{netNode: dsp, provider: "SKY"}
	gov, err := s.startNetNode(ctx, govPeer(shard), "GV", govAddress(shard), "", switchAddr, nil, log.With("node", "border"))
	if err != nil {
		return err
	}
	s.Border = &Border{netNode: gov, byDest: map[string]int64{}}
	gov.GW.APIS = s.Border.receive
	gov.GW.PNRGOV = s.Border.receivePush
	ansp := &ANSP{}
	node, err := s.startNetNode(ctx, atcPeer(shard), "AT", atcAddress(shard), "XXXXZQZX", switchAddr,
		gateway.GroundFuncs{OnATS: func(ctx context.Context, m *ats.Message, env *aftn.Message) error {
			if m.Type == ats.TypeFPL {
				ansp.plans.Add(1)
			}
			return nil
		}}, log.With("node", "atc"))
	if err != nil {
		return err
	}
	ansp.netNode = node
	s.ANSP = ansp
	return nil
}

// FlightPlansFiled is how many flight plans this machine's ANSP received.
func (a *ANSP) FlightPlansFiled() int64 { return a.plans.Load() }
