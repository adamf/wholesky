package host

// The ground side of a carrier: departure control, the sortation system and
// the arrival station, all served by the same tenant node the way a hosted
// carrier's are. Reservations sends the name list to the airport over the
// network; the airport is this same process, reached by a different address
// down the same circuit, and it answers with the messages the rest of the
// network runs on.
//
// What is real here is the system: pkg/dcs opens the flight from the PNL,
// seats and tags and boards, closes the door and builds the messages. What
// is synthetic is the passengers' behaviour -- who turns up when, who checks
// a bag, who misses a connection -- and that is drawn deterministically from
// each party's identity so a given world always tells the same story.

import (
	"context"
	"errors"
	"fmt"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/iatci"
	"github.com/adamf/jetway/pkg/paxlst"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/baggage"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/pnl"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/typeb"

	"github.com/adamf/wholesky/internal/airport"
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/extras"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// Office function designators on a station address: the two letters after
// the city that say which desk a message is for. RP 1704 standardises the
// vocabulary; these are the conventional ones.
const (
	deptCheckIn = "KP" // passenger handling: check-in, the DCS
	deptBaggage = "KB" // baggage: the sortation system
	deptLoad    = "KL" // load control
	deptRevenue = "RA" // revenue accounting
)

// stationAddress is this carrier's address for a desk at an airport.
func (t *Tenant) stationAddress(iata, dept string) string {
	return iata + dept + t.Carrier.Designator
}

// flightKey is how departure control names a departure.
func flightKey(f world.Flight, day time.Time) dcs.Key {
	return dcs.Key{Flight: f.Carrier + f.Number, Date: strings.ToUpper(day.Format("02Jan")), Board: f.From}
}

// registrationOf matches the registration the movement messages carry, so
// the LDM and the MVT agree about which airframe flew.
func registrationOf(f world.Flight) string {
	if f.Tail != "" {
		return f.Tail
	}
	return fmt.Sprintf("SKY%03d", hashOf(f.Carrier+f.Number)%1000)
}

// fleetOf is the fleet a carrier flies: the same types with the cabins a
// carrier of its kind fits. A network carrier's narrow-bodies have a
// front cabin, two seats either side of the aisle: three rows on an
// A320, four on a 737-800 and five on an A321, which is 12, 16 and 20
// seats and about what American, Delta and United fit. A low-cost
// carrier's have one cabin from the door to the tail. The wide-bodies
// have a business cabin whoever flies them.
func fleetOf(carrier string) *dcs.FleetData {
	if equipment.ModelOf(carrier) != equipment.Network {
		oneCabinOnce.Do(func() { oneCabin = fleetData() })
		return oneCabin
	}
	twoCabinOnce.Do(func() {
		fl := fleetData()
		front := func(code string, rows int) {
			t, ok := fl.Types[code]
			if !ok {
				return
			}
			cp := *t
			cp.Cabin.Sections = []dcs.Section{{Compartment: "C", FromRow: 1, ToRow: rows, Letters: "AC DF"}}
			for _, sec := range t.Cabin.Sections {
				if sec.ToRow <= rows {
					continue
				}
				sec.FromRow = max(sec.FromRow, rows+1)
				cp.Cabin.Sections = append(cp.Cabin.Sections, sec)
			}
			fl.Types[code] = &cp
		}
		front("320", 3)
		front("73H", 4)
		front("321", 5)
		twoCabin = fl
	})
	return twoCabin
}

var (
	oneCabinOnce, twoCabinOnce sync.Once
	oneCabin, twoCabin         *dcs.FleetData
)

// fleetData is jetway's representative fleet plus the two types a replayed
// US day is mostly made of: the 76-seat regional jet and the 737-800.
// Representative figures, like the rest -- not an operator's AHM 560.
func fleetData() *dcs.FleetData {
	fl := dcs.DefaultFleet()
	fl.Types["E75"] = &dcs.AircraftType{
		Code: "E75", Name: "Embraer 175",
		Cabin: dcs.CabinLayout{Sections: []dcs.Section{{Compartment: "Y", FromRow: 1, ToRow: 19, Letters: "AB CD"}}},
		DOW:   21500, DOWArm: 15.4, MZFW: 31700, MTOW: 38790, MLW: 34000,
		RefArm: 15.2, C: 500, K: 50, LEMAC: 14.6, MAC: 2.6, FwdMAC: 14, AftMAC: 36, FuelArm: 15.3,
		Compartments: []dcs.Compartment{{Name: "1", Max: 1200, Arm: 9.8}, {Name: "3", Max: 1600, Arm: 21.9}},
		Zones:        []dcs.Zone{{Name: "OA", FromRow: 1, ToRow: 7, Arm: 11.5}, {Name: "OB", FromRow: 8, ToRow: 13, Arm: 15.3}, {Name: "OC", FromRow: 14, ToRow: 19, Arm: 19.2}},
	}
	fl.Types["73H"] = &dcs.AircraftType{
		Code: "73H", Name: "Boeing 737-800",
		Cabin: dcs.CabinLayout{Sections: []dcs.Section{{Compartment: "Y", FromRow: 1, ToRow: 29, Letters: "ABC DEF"}}},
		DOW:   41400, DOWArm: 18.9, MZFW: 62700, MTOW: 79000, MLW: 66300,
		RefArm: 18.6, C: 1000, K: 50, LEMAC: 17.5, MAC: 3.96, FwdMAC: 15, AftMAC: 33, FuelArm: 18.7,
		Compartments: []dcs.Compartment{{Name: "1", Max: 3600, Arm: 12.0}, {Name: "3", Max: 2500, Arm: 25.4}, {Name: "4", Max: 1900, Arm: 28.6}},
		Zones:        []dcs.Zone{{Name: "OA", FromRow: 1, ToRow: 10, Arm: 13.2}, {Name: "OB", FromRow: 11, ToRow: 20, Arm: 18.7}, {Name: "OC", FromRow: 21, ToRow: 29, Arm: 24.3}},
	}
	return fl
}

// crewFor is the cockpit/cabin complement the LDM reports, by type.
func crewFor(equipment string) string {
	switch equipment {
	case "AT7", "E75":
		return "2/2"
	case "73H":
		return "2/4"
	case "320":
		return "2/4"
	case "321":
		return "2/5"
	case "789":
		return "3/10"
	case "77W":
		return "4/13"
	}
	return "2/4"
}

// nopStore is the departure control store for a simulated carrier: nothing
// is persisted, because five hundred carriers' manifests as JSON documents
// is memory the region machines do not have, and a simulated day has no
// restart to survive. A real deployment gives the station a durable store.
type nopStore struct{}

func (nopStore) SaveFlight(context.Context, *dcs.Flight) error { return nil }
func (nopStore) LoadFlight(context.Context, dcs.Key) (*dcs.Flight, error) {
	return nil, dcs.ErrNotFound
}
func (nopStore) ListFlights(context.Context) ([]dcs.Key, error) { return nil, nil }
func (nopStore) DeleteFlight(context.Context, dcs.Key) error    { return nil }

// startGround stands up the carrier's departure control and plugs it into
// the gateway's Ground seam.
func (t *Tenant) startGround(accountingCode string) {
	t.flightsByNum = map[string]world.Flight{}
	for _, f := range t.flights {
		t.flightsByNum[f.Carrier+f.Number] = f
	}
	if accountingCode == "" {
		accountingCode = fmt.Sprintf("%03d", hashOf(t.Carrier.Designator)%900+100)
	}
	st := dcs.NewStation(t.Carrier.Designator)
	st.AccountingCode = accountingCode
	st.Store = nopStore{}
	st.Fleet = fleetOf(t.Carrier.Designator)
	st.Log = t.log
	st.Equipment = func(k dcs.Key) (dcs.Equipment, bool) {
		f, ok := t.flightsByNum[k.Flight]
		if !ok {
			return dcs.Equipment{}, false
		}
		return dcs.Equipment{Type: f.Equipment, Registration: registrationOf(f), Dest: f.To, Crew: crewFor(f.Equipment)}, true
	}
	t.DCS = st
	t.sortation = map[string]map[string]bool{}
	t.boarded = map[string]int{}
	t.carried = map[string]int{}
	t.cancelled = map[string]string{}
	t.Gateway.Ground = t
}

// CancelFlight is the airport hearing that a flight will not fly: departure
// control closes it, offloading whoever was accepted and pulling their
// bags, and the rest of the day's ground story for it is skipped. A flight
// not yet under control is remembered so its counter never opens.
func (t *Tenant) CancelFlight(ctx context.Context, f world.Flight, day time.Time, reason string) error {
	key := flightKey(f, day)
	t.groundMu.Lock()
	t.cancelled[key.Flight+"/"+key.Date] = reason
	t.groundMu.Unlock()
	c, err := t.DCS.CancelFlight(ctx, key, reason)
	if errors.Is(err, dcs.ErrFlightNotFound) || errors.Is(err, dcs.ErrFlightClosed) {
		return nil
	}
	if err != nil {
		return err
	}
	var firstErr error
	for _, p := range c.Offloaded {
		if err := t.announceBags(ctx, c.Flight, p, "DEL"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// isCancelled reports whether the day has cancelled a flight.
func (t *Tenant) isCancelled(f world.Flight, day time.Time) bool {
	key := flightKey(f, day)
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	_, ok := t.cancelled[key.Flight+"/"+key.Date]
	return ok
}

// NameList implements gateway.Ground: reservations' PNL or ADL arriving at
// the airport, which is this tenant at a different address.
func (t *Tenant) NameList(ctx context.Context, m *pnl.Message, origin typeb.Address) error {
	_, err := t.DCS.ApplyNameList(ctx, m)
	return err
}

// Baggage implements gateway.Ground. A BSM is check-in telling the sortation
// system a bag exists; this tenant plays the sortation system and remembers
// the tag. A BPM is the sortation system reporting what it loaded; this
// tenant's departure control reconciles it.
func (t *Tenant) Baggage(ctx context.Context, m *baggage.Message, origin typeb.Address) error {
	if m.Outbound == nil {
		return fmt.Errorf("bag message names no flight")
	}
	switch m.Kind {
	case baggage.KindBSM:
		key := m.Outbound.Flight + "/" + m.Outbound.Date
		t.groundMu.Lock()
		defer t.groundMu.Unlock()
		tags := t.sortation[key]
		if tags == nil {
			tags = map[string]bool{}
			t.sortation[key] = tags
		}
		for _, tag := range m.Tags {
			for i := 0; i < max(1, tag.Count); i++ {
				n := tag.Number
				if i > 0 && len(n) == 10 {
					var seq int
					fmt.Sscanf(n[4:], "%d", &seq)
					n = fmt.Sprintf("%s%06d", n[:4], seq+i)
				}
				if m.Change == "DEL" {
					delete(tags, n)
				} else {
					tags[n] = true
				}
			}
		}
		return nil
	case baggage.KindBPM:
		_, _, err := t.DCS.ApplyBagReport(ctx, m)
		return err
	case baggage.KindBUM:
		// A bag is coming without its passenger: the station it is going
		// to expects it, counted by the flight's destination.
		t.groundMu.Lock()
		t.rushIn[m.Outbound.City] += len(m.Tags)
		t.groundMu.Unlock()
		return nil
	}
	return nil
}

// Departure implements gateway.Ground: departure control output arriving
// at the desk it was addressed to. Final sales come home to reservations
// and are written onto the records; the arrival station's messages are
// counted, which is all a simulated station does with them.
func (t *Tenant) Departure(ctx context.Context, m *dcs.Message, origin typeb.Address) error {
	switch m.Kind {
	case dcs.KindPFS:
		return t.applyFinalSales(ctx, m.PFS)
	default:
		t.groundMu.Lock()
		t.arrivals[m.Kind]++
		t.groundMu.Unlock()
		return nil
	}
}

// applyFinalSales writes what the airport reported onto the bookings:
// a no-show or an offload becomes a remark and an event on the record,
// which is what a reservations system does with a PFS before revenue
// management does the rest.
func (t *Tenant) applyFinalSales(ctx context.Context, m *dcs.PFS) error {
	if m == nil {
		return nil
	}
	var firstErr error
	for _, g := range m.Groups {
		for _, item := range g.Items {
			if item.Category != "NOSHO" && item.Category != "OFFLD" {
				continue
			}
			loc := ""
			for _, e := range item.Name.Elements {
				if strings.HasPrefix(e, ".L/") {
					loc = strings.TrimPrefix(e, ".L/")
				}
			}
			if loc == "" {
				continue
			}
			rec, err := t.Store.GetPNR(ctx, loc)
			if err != nil {
				continue // cancelled since the list went out; nothing to mark
			}
			expected := rec.Version
			rec.UpdatedAt = time.Now().UTC()
			rec.Remarks = append(rec.Remarks, pnr.Remark{Text: fmt.Sprintf("%s %s/%s %s", item.Category, m.Flight, m.Date, m.Board)})
			ev := store.Event{Type: "final_sales", Detail: item.Category + " " + m.Flight + "/" + m.Date, Actor: "dcs", At: time.Now().UTC()}
			if err := t.Store.UpdatePNR(ctx, rec, expected, []store.Event{ev}); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// ArrivalMessages reports how many of each departure control message this
// tenant received as an arrival station or revenue desk.
func (t *Tenant) ArrivalMessages() map[dcs.Kind]int {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	out := map[dcs.Kind]int{}
	for k, v := range t.arrivals {
		out[k] = v
	}
	return out
}

// party is one name item's members as departure control holds them.
type party struct {
	key  string
	lead *dcs.Passenger
	pax  []*dcs.Passenger
}

func partiesOf(f *dcs.Flight, status dcs.Status) []party {
	var order []string
	byKey := map[string]*party{}
	for _, p := range f.Passengers {
		if p.Status != status {
			continue
		}
		pt, ok := byKey[p.Party]
		if !ok {
			pt = &party{key: p.Party, lead: p}
			byKey[p.Party] = pt
			order = append(order, p.Party)
		}
		pt.pax = append(pt.pax, p)
	}
	out := make([]party, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// arrival is when a party turns up at the airport, in minutes before the
// scheduled departure, or false if they never do. It is drawn from the
// party's identity so it is the same on every tick, and it is where the
// synthetic behaviour lives: about 1 party in 11 does not come, more of
// those on fares that can be refunded (tariff.NoShow); a share connect
// off an inbound flight and miss the connection when that flight is late.
func (t *Tenant) arrival(pt party, f world.Flight, day time.Time) (minutesOut int, inbound *dcs.Connection, ok bool) {
	h := hashOf(pt.key + f.Carrier + f.Number + day.Format("0102"))
	class := ""
	if pt.lead != nil {
		class = pt.lead.Class
	}
	if staysAway(h, class) {
		return 0, nil, false // no-show
	}
	r := h % 100
	switch {
	case r < 14:
		// Connecting off one of this carrier's own arrivals into f.From.
		inb, found := t.inboundFor(f, h)
		if !found {
			break
		}
		delay, held := 0, 0
		if t.inboundDelay != nil {
			delay = t.inboundDelay(inb, day)
		}
		if t.departureDelay != nil {
			held = t.departureDelay(f, day)
		}
		gap := f.DepMin - inb.ArrMin%1440
		if missesConnection(h, gap-t.connectTime(inb, f), delay, held) {
			// Misconnected: they are not on this flight.
			t.missedMu.Lock()
			t.missed[pt.key+f.Carrier+f.Number] = Missed{Flight: f, Inbound: inb, Pax: len(pt.pax)}
			t.missedMu.Unlock()
			return 0, nil, false
		}
		arrives := gap - delay + held // minutes before the departure leaves
		conn := &dcs.Connection{Flight: inb.Carrier + inb.Number, Date: strings.ToUpper(day.Format("02Jan")), Station: f.From, Dest: f.To}
		return max(min(arrives-10, 170), 0), conn, true
	}
	return 46 + (h/100)%125, nil, true // 46..170 minutes before departure
}

// staysAway says whether a party booked in a class does not come for the
// flight. The party's own number decides, against the share of its fare's
// travellers who stay away.
func staysAway(h int, class string) bool {
	return (float64((h/13)%10000)+0.5)/10000 < tariff.NoShow(class)
}

// volunteerShare is the share of seated passengers who give up the seat
// for the carrier's offer when it asks. An estimate: with 150 passengers
// seated, 6 take it. Worked out once over the oversold flights of a
// filled US world, which are short of 3.7 seats on average, this share
// finds a volunteer for 91 in 100 of the passengers without a seat: 11
// volunteers for each passenger turned away. Airlines reported 9 to 1 in
// 2012 and 33 to 1 in 2018 (GAO-20-191). A share of 0.06 gives 50 to 1.
const volunteerShare = 0.04

// volunteers says whether a seated passenger takes the carrier's offer.
func volunteers(p *dcs.Passenger, f world.Flight) bool {
	h := hashOf(p.Party + fmt.Sprint(p.ID) + f.Number + "offer")
	return (float64((h/7)%1000)+0.5)/1000 < volunteerShare
}

// Denied is one passenger denied boarding today: the flight, the class
// booked, whether the passenger agreed, and the cause.
type Denied struct {
	Flight    world.Flight
	Class     string
	Voluntary bool
	Cause     dcs.Oversale
}

// Denied is the passengers denied boarding today.
func (t *Tenant) Denied() []Denied {
	t.missedMu.Lock()
	defer t.missedMu.Unlock()
	return append([]Denied(nil), t.denied...)
}

func (t *Tenant) noteDenied(f world.Flight, p *dcs.Passenger) {
	t.missedMu.Lock()
	t.denied = append(t.denied, Denied{Flight: f, Class: p.Class, Voluntary: p.Volunteer, Cause: p.Oversale})
	t.missedMu.Unlock()
}

// minimumConnection is the minutes a connection needs at a station whose
// own time is not known here.
const minimumConnection = 45

// connectTime is the least time a passenger needs between two flights:
// the station's minimum connecting time, where the world has told the
// carrier what it is.
func (t *Tenant) connectTime(in, out world.Flight) int {
	if t.connect != nil {
		if m := t.connect(in, out); m > 0 {
			return m
		}
	}
	return minimumConnection
}

// missesConnection says whether a passenger misses a connection that was
// scheduled with slack minutes over the minimum connecting time, when
// the inbound flight is late by inbound minutes and the outbound leaves
// late by outbound. The chance is the curve of internal/airport; the
// passenger's own number decides which side of it they fall, so the
// answer is the same every time it is asked.
func missesConnection(h, slack, inbound, outbound int) bool {
	return (float64((h/7)%1000)+0.5)/1000 < airport.Misconnect(slack, inbound, outbound)
}

// Missed is one party that missed a connection today: the departure it
// was booked on, the arrival that brought it in late, and how many
// passengers it is.
type Missed struct {
	Flight, Inbound world.Flight
	Pax             int
}

// Misconnected is the passengers who missed a connection onto one of
// this carrier's departures today because their inbound flight was late.
func (t *Tenant) Misconnected() int {
	n := 0
	for _, m := range t.Missed() {
		n += m.Pax
	}
	return n
}

// Missed is the parties that missed a connection today.
func (t *Tenant) Missed() []Missed {
	t.missedMu.Lock()
	defer t.missedMu.Unlock()
	out := make([]Missed, 0, len(t.missed))
	for _, m := range t.missed {
		out = append(out, m)
	}
	return out
}

// NewDay starts the counts that are a day's over: the bag office's
// files, the missed connections and the denied boardings. The day that ended has been closed
// into the carrier's book with them.
func (t *Tenant) NewDay() {
	t.groundMu.Lock()
	t.tracing = Tracing{}
	t.carried = map[string]int{}
	t.extras, t.extraUnits = map[string]int64{}, map[string]int{}
	t.groundMu.Unlock()
	t.missedMu.Lock()
	t.missed = map[string]Missed{}
	t.denied = nil
	t.missedMu.Unlock()
}

// inboundFor picks one of the carrier's arrivals into f.From that a
// passenger could plausibly connect from: landing between five hours and
// forty-five minutes before f departs.
func (t *Tenant) inboundFor(f world.Flight, h int) (world.Flight, bool) {
	var cands []world.Flight
	for _, g := range t.flights {
		if g.To != f.From || g.Carrier != f.Carrier {
			continue
		}
		gap := f.DepMin - g.ArrMin%1440
		if gap >= t.connectTime(g, f) && gap <= 300 {
			cands = append(cands, g)
		}
	}
	if len(cands) == 0 {
		return world.Flight{}, false
	}
	return cands[(h/1000)%len(cands)], true
}

// onwardFor picks a same-carrier departure from f.To that a passenger could
// connect to, or nothing.
func (t *Tenant) onwardFor(f world.Flight, day time.Time, h int) *dcs.Connection {
	if (h/3)%100 >= 18 {
		return nil
	}
	arr := f.ArrMin % 1440
	fits := func(g world.Flight) bool {
		if g.From != f.To {
			return false
		}
		gap := g.DepMin - arr
		if gap < 0 {
			gap += 1440
		}
		return gap >= minimumConnection && gap <= 300
	}
	var own, other []world.Flight
	for _, g := range t.flights {
		if g.Carrier == f.Carrier && fits(g) {
			own = append(own, g)
		}
	}
	if t.interline != nil {
		for _, g := range t.interline(f) {
			if g.Carrier != f.Carrier && fits(g) {
				other = append(other, g)
			}
		}
	}
	// One connection in four crosses carriers when a partner's flight fits:
	// those are the ones the IATCI dialogue exists for.
	cands := own
	if len(other) > 0 && ((h/11)%4 == 0 || len(own) == 0) {
		cands = other
	}
	if len(cands) == 0 {
		return nil
	}
	g := cands[(h/7)%len(cands)]
	c := &dcs.Connection{Flight: g.Carrier + g.Number, Date: strings.ToUpper(day.Format("02Jan")), Station: f.To, Dest: g.To, Class: "Y"}
	if g.Carrier != f.Carrier {
		c.Carrier = g.Carrier
	}
	return c
}

// throughPending is a through check-in request awaiting the other carrier's
// answer, keyed by the passenger reference the request carried.
type throughPending struct {
	key dcs.Key
	pid int
}

// ThroughCheck asks the other carrier's departure control to check the
// party's connecting passengers in on its flight: the delivering side of
// IATCI. The request goes to the network addressed to the receiving carrier;
// the answer arrives as a DCRCKA and lands in onThroughCheckIn.
func (t *Tenant) ThroughCheck(ctx context.Context, f world.Flight, day time.Time, key dcs.Key, fl *dcs.Flight, pax []*dcs.Passenger) error {
	byCarrier := map[string][]*dcs.Passenger{}
	for _, p := range pax {
		if p.Onward != nil && p.Onward.Carrier != "" && p.Onward.Carrier != t.Carrier.Designator {
			byCarrier[p.Onward.Carrier] = append(byCarrier[p.Onward.Carrier], p)
		}
	}
	var firstErr error
	for carrier, ps := range byCarrier {
		on := ps[0].Onward
		req := &iatci.CheckInRequest{
			Requestor: t.Carrier.Designator, RequestorStation: f.From,
			Inbound:  iatci.Flight{Carrier: f.Carrier, Number: f.Number, Date: day.Add(time.Duration(f.DepMin) * time.Minute), Arrives: day.Add(time.Duration(f.ArrMin) * time.Minute), Board: f.From, Off: f.To},
			Outbound: iatci.Flight{Carrier: carrier, Number: strings.TrimPrefix(on.Flight, carrier), Date: day, Board: on.Station, Off: on.Dest},
		}
		for _, p := range ps {
			ref := fmt.Sprintf("%d", p.ID)
			ip := iatci.Passenger{Ref: ref, Surname: p.Surname, Given: p.Given, Type: "A", Locator: p.Locator, Class: on.Class, Pieces: len(p.Bags)}
			if p.Type == dcs.PaxChild {
				ip.Type = "C"
			}
			for _, b := range p.Bags {
				ip.Weight += b.Weight
				ip.Tags = append(ip.Tags, iatci.Tag{Carrier: t.Carrier.Designator, Serial: b.Tag, Count: 1, Dest: on.Dest})
			}
			for _, ssr := range p.SSRs {
				ip.SSRs = append(ip.SSRs, ssr.Code)
			}
			req.Passengers = append(req.Passengers, ip)
			t.pendingMu.Lock()
			t.pendingThrough[carrier+"/"+ref] = throughPending{key: key, pid: p.ID}
			t.pendingMu.Unlock()
		}
		peer := &gateway.Peer{Name: "net", Carrier: carrier, Format: store.FormatEDIFACT}
		if err := t.Gateway.RequestThroughCheckIn(ctx, peer, req); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// onThroughCheckIn hears the receiving carrier's answer and writes the seat
// or the refusal onto the passenger's connection, so the manifest and the
// transfer message carry it.
func (t *Tenant) onThroughCheckIn(ctx context.Context, peer *gateway.Peer, res *iatci.CheckInResponse) {
	for _, r := range res.Passengers {
		t.pendingMu.Lock()
		pend, ok := t.pendingThrough[res.Flight.Carrier+"/"+r.Ref]
		if ok {
			delete(t.pendingThrough, res.Flight.Carrier+"/"+r.Ref)
		}
		t.pendingMu.Unlock()
		if !ok {
			continue
		}
		refused := ""
		if r.Status != "" && r.Status != "H" {
			refused = "refused"
			if len(r.Errors) > 0 {
				refused = strings.ToLower(r.Errors[0].Text)
				if refused == "" {
					refused = "code " + r.Errors[0].Code
				}
			}
		}
		if err := t.DCS.RecordOnward(ctx, pend.key, pend.pid, r.Seat, r.Sequence, refused); err != nil {
			t.log.Debug("through check-in answer not recorded", "flight", pend.key.Flight, "pid", pend.pid, "err", err)
		}
	}
}

// international reports whether a flight crosses a border, where the
// world has told the carrier each airport's country.
func (t *Tenant) international(f world.Flight) bool {
	if t.countryOf == nil {
		return false
	}
	a, b := t.countryOf(f.From), t.countryOf(f.To)
	return a != "" && b != "" && a != b
}

// sellExtras counts what the passengers who flew bought beside the seat:
// their bags at the carrier's price for them, a seat, and what they took
// on board (internal/extras). It is counted at the door, for the
// passengers who boarded. A flight that does not fly sells nothing, and a
// passenger who does not come or is turned away buys nothing: the law
// has the fee for a service not provided refunded (14 CFR 260.4).
func (t *Tenant) sellExtras(leg world.Flight, fl *dcs.Flight) {
	sold, units := extrasOf(leg, fl, t.international(leg))
	t.groundMu.Lock()
	if t.extras == nil {
		t.extras, t.extraUnits = map[string]int64{}, map[string]int{}
	}
	for code, cents := range sold {
		t.extras[code] += cents
		t.extraUnits[code] += units[code]
	}
	t.groundMu.Unlock()
}

// extrasOf is what the passengers who boarded a flight bought beside the
// seat, in cents and in units by service. A passenger who came off a
// connecting flight paid for the bags where the journey began.
func extrasOf(leg world.Flight, fl *dcs.Flight, intl bool) (sold map[string]int64, units map[string]int) {
	menu := extras.For(leg.Carrier)
	minutes := leg.BlockMin
	if minutes <= 0 {
		minutes = leg.ArrMin - leg.DepMin
	}
	sold, units = map[string]int64{}, map[string]int{}
	for _, p := range fl.Passengers {
		if p.Status != dcs.StatusBoarded {
			continue
		}
		cabin := p.Compartment
		if fl.Cabin != nil {
			cabin = fl.Cabin.CompartmentFor(p.Class)
		}
		bags := 0
		for _, b := range p.Bags {
			if !b.Offloaded {
				bags++
			}
		}
		who := extras.Passenger{Class: p.Class, Cabin: cabin, KM: leg.KM, Minutes: minutes, International: intl,
			Bags: bags, Continuing: p.Inbound != nil}
		for _, s := range menu.Buy(who, uint64(hashOf(p.Party+"/"+p.Surname+"/"+p.Given+"/"+fl.Flight+"/"+fmt.Sprint(p.ID)))) {
			sold[s.Code] += s.Cents
			units[s.Code] += s.Units
		}
	}
	return sold, units
}

// Extras is what the day's passengers bought beside the seat, in cents
// by service, and how many of each.
func (t *Tenant) Extras() (cents map[string]int64, units map[string]int) {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	cents, units = map[string]int64{}, map[string]int{}
	for k, v := range t.extras {
		cents[k] = v
	}
	for k, v := range t.extraUnits {
		units[k] = v
	}
	return cents, units
}

// CheckIn is one wave of the counter: every listed party that has arrived
// by now and has not yet been accepted is checked in, seated, tagged, and
// its bags announced to the sortation system.
func (t *Tenant) CheckIn(ctx context.Context, f world.Flight, day time.Time, minutesOut int) error {
	if t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	fl, err := t.DCS.Flight(key)
	if err != nil {
		return err
	}
	if fl.State != dcs.StateOpen {
		return nil
	}
	var firstErr error
	for _, pt := range partiesOf(fl, dcs.StatusListed) {
		at, inbound, arrives := t.arrival(pt, f, day)
		if !arrives || at < minutesOut {
			continue
		}
		h := hashOf(pt.key + f.Carrier + f.Number)
		req := dcs.AcceptRequest{
			Locator: pt.lead.Locator, Surname: pt.lead.Surname,
			Bags: extras.For(f.Carrier).Bags(uint64(h)), Onward: t.onwardFor(f, day, h), Inbound: inbound,
		}
		if pt.lead.Locator == "" {
			req.PassengerID = pt.lead.ID
		}
		acc, err := t.DCS.Accept(ctx, key, req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(acc.Passengers) > 0 && acc.Passengers[0].Status != dcs.StatusAccepted {
			// The cabin is full and the party waits for seats with a claim
			// to them. Its bags are held until it has them.
			continue
		}
		if len(acc.Tags) > 0 {
			if err := t.announceBags(ctx, acc.Flight, acc.Passengers[0], ""); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if req.Onward != nil && req.Onward.Carrier != "" {
			if err := t.ThroughCheck(ctx, f, day, key, acc.Flight, acc.Passengers); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	// One flight in a hundred has somebody at the counter the list never
	// carried: a ticket and no record. That is what NOREC is for.
	if minutesOut == 90 && hashOf(f.Carrier+f.Number+day.Format("0102"))%100 == 0 {
		g := dcs.GoShow{Surname: "GOSHOW", Given: fmt.Sprintf("PAX%02dMR", hashOf(f.Number)%100), Class: "Y", Bags: []int{15}}
		if acc, err := t.DCS.AcceptGoShow(ctx, key, g); err == nil && len(acc.Tags) > 0 && acc.Passengers[0].Status == dcs.StatusAccepted {
			_ = t.announceBags(ctx, acc.Flight, acc.Passengers[0], "")
		}
	}
	return firstErr
}

// announceBags sends the BSM for one passenger's bags to the sortation
// system at the departure airport.
func (t *Tenant) announceBags(ctx context.Context, fl *dcs.Flight, p *dcs.Passenger, change string) error {
	if len(p.Bags) == 0 {
		return nil
	}
	m := t.DCS.BSMFor(fl, p, change)
	text, err := baggage.Build(m)
	if err != nil {
		return err
	}
	return t.sendTypeBFrom(ctx, t.stationAddress(fl.Board, deptCheckIn), []string{t.stationAddress(fl.Board, deptBaggage)}, text, "BSM")
}

// OnAccept is the console hook: an agent's acceptance announces bags the
// same way the wave does.
func (t *Tenant) OnAccept(ctx context.Context, acc *dcs.Acceptance) {
	if acc == nil || len(acc.Tags) == 0 || len(acc.Passengers) == 0 {
		return
	}
	if err := t.announceBags(ctx, acc.Flight, acc.Passengers[0], ""); err != nil {
		t.log.Debug("bsm not sent", "err", err)
	}
}

// OnOffload is the console hook: an offloaded passenger's bags must come off.
func (t *Tenant) OnOffload(ctx context.Context, fl *dcs.Flight, p *dcs.Passenger) {
	if err := t.announceBags(ctx, fl, p, "DEL"); err != nil {
		t.log.Debug("bsm del not sent", "err", err)
	}
}

// CloseCheckIn closes the counter; standbys clear into whatever is left,
// the booked passengers who wait for a seat first. If some of those
// still wait, the agent asks for volunteers.
func (t *Tenant) CloseCheckIn(ctx context.Context, f world.Flight, day time.Time) error {
	if t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	fl, cleared, err := t.DCS.CloseCheckIn(ctx, key)
	if err != nil {
		return err
	}
	for _, p := range cleared {
		if aerr := t.announceBags(ctx, fl, p, ""); aerr != nil && err == nil {
			err = aerr
		}
	}
	if serr := t.solicit(ctx, f, key, fl); serr != nil && err == nil {
		err = serr
	}
	return err
}

// solicit asks the seated passengers, in the order they checked in, to
// give up their seats while booked passengers wait for one. Each who
// agrees is a denied boarding by agreement. Their seat goes to the
// passenger who has waited longest, and their bags come off.
func (t *Tenant) solicit(ctx context.Context, f world.Flight, key dcs.Key, fl *dcs.Flight) error {
	waiting := fl.Counts().Waiting
	var firstErr error
	for _, p := range fl.Passengers {
		if waiting <= 0 {
			break
		}
		if p.Status != dcs.StatusAccepted || !volunteers(p, f) {
			continue
		}
		v, err := t.DCS.Volunteer(ctx, key, p.ID)
		if errors.Is(err, dcs.ErrNotOversold) {
			break
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		t.noteDenied(f, v.Volunteer)
		if err := t.announceBags(ctx, v.Flight, v.Volunteer, "DEL"); err != nil && firstErr == nil {
			firstErr = err
		}
		if v.Seated != nil {
			waiting--
			if err := t.announceBags(ctx, v.Flight, v.Seated, ""); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Board is one wave of the gate: every accepted passenger whose turn has
// come walks on. One in a few hundred never does.
func (t *Tenant) Board(ctx context.Context, f world.Flight, day time.Time, minutesOut int) error {
	if t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	fl, err := t.DCS.Flight(key)
	if err != nil {
		return err
	}
	if fl.State == dcs.StateClosed {
		return nil
	}
	var firstErr error
	for _, p := range fl.Passengers {
		if p.Status != dcs.StatusAccepted {
			continue
		}
		h := hashOf(p.Party + fmt.Sprint(p.ID) + f.Number)
		if (h/17)%300 == 0 {
			continue // gate no-show: at the bar, on the phone, gone
		}
		at := 14 + (h/11)%22 // boards 14..35 minutes before departure
		if at < minutesOut {
			continue
		}
		if _, err := t.DCS.Board(ctx, key, p.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ReportBags is the sortation system's word: everything it was told about
// and not told to pull is on board. It goes to check-in as a BPM, and
// departure control reconciles it against the manifest.
func (t *Tenant) ReportBags(ctx context.Context, f world.Flight, day time.Time) error {
	if t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	t.groundMu.Lock()
	tags := t.sortation[key.Flight+"/"+key.Date]
	var list []string
	for tag := range tags {
		list = append(list, tag)
	}
	delete(t.sortation, key.Flight+"/"+key.Date)
	t.groundMu.Unlock()
	if len(list) == 0 {
		return nil
	}
	sortStrings(list)
	// A message carries sixty lines; forty tags to a BPM keeps it legal.
	for i := 0; i < len(list); i += 40 {
		end := min(i+40, len(list))
		m := &baggage.Message{
			Kind: baggage.KindBPM, Version: "1L" + f.From,
			Outbound: &baggage.FlightLeg{Flight: key.Flight, Date: key.Date, City: f.To},
		}
		for _, tag := range list[i:end] {
			m.Tags = append(m.Tags, baggage.Tag{Number: tag, Count: 1})
		}
		text, err := baggage.Build(m)
		if err != nil {
			return err
		}
		if err := t.sendTypeBFrom(ctx, t.stationAddress(f.From, deptBaggage), []string{t.stationAddress(f.From, deptCheckIn)}, text, "BPM"); err != nil {
			return err
		}
	}
	return nil
}

// fuelFor is the flight plan's fuel by type and block time: a burn rate
// times the trip, a reserve on top. Load control needs a number; this is
// the number a dispatcher would hand over.
func fuelFor(f world.Flight) dcs.FuelPlan {
	rate := map[string]int{"AT7": 650, "320": 2500, "321": 2900, "789": 5600, "77W": 7600}[f.Equipment]
	if rate == 0 {
		rate = 2500
	}
	trip := rate * f.BlockMin / 60
	return dcs.FuelPlan{Trip: trip, TakeOff: trip*13/10 + rate/4}
}

// cargoFor is the dead load a flight carries besides bags.
func cargoFor(f world.Flight, h int) (cargo, mail int) {
	switch f.Equipment {
	case "789", "77W":
		return 1500 + h%6000, h / 7 % 400
	case "AT7":
		return h % 120, 0
	default:
		return h % 900, h / 7 % 60
	}
}

// Close is the door closing: departure control finalises the flight and
// this tenant transmits what it produced, each message to the desk that
// reads it.
func (t *Tenant) Close(ctx context.Context, f world.Flight, day time.Time) error {
	if t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	h := hashOf(f.Carrier + f.Number + day.Format("0102"))
	cargo, mail := cargoFor(f, h)
	// The door does not close over a loaded bag whose passenger is not on
	// board: reconciliation names them, sortation is told to pull each
	// (BSM DEL), and the door closes on the second try with the hold
	// matching the cabin.
	opts := dcs.CloseOptions{Fuel: fuelFor(f), Cargo: cargo, Mail: mail, Force: true, RequireReconciled: true}
	cl, err := t.DCS.CloseFlight(ctx, key, opts)
	var ue *dcs.UnreconciledError
	if errors.As(err, &ue) {
		t.pullBags(ctx, key, ue.Bags)
		opts.RequireReconciled = false
		cl, err = t.DCS.CloseFlight(ctx, key, opts)
	}
	if err != nil {
		return err
	}
	err = t.SendClosure(ctx, cl)
	// And the same state its PNR push at departure: the records as booked,
	// now with the seat, sequence and bags the door gave each traveller.
	if perr := t.sendPNRGOV(ctx, f, day, cl.Flight); perr != nil && err == nil {
		err = perr
	}
	// The bags the hold never reported follow on the next flight over the
	// sector: sortation is told to load them on it and the arrival station
	// is told to expect a bag without its passenger (a BUM), which is what
	// a bag office reunites from.
	if rerr := t.rushBags(ctx, f, day, cl.Flight); rerr != nil && err == nil {
		err = rerr
	}
	return err
}

// rushBags sends a closed flight's short-shipped bags on the carrier's next
// flight over the same sector today. Without one, the bags wait for
// tomorrow and the panel says so.
func (t *Tenant) rushBags(ctx context.Context, f world.Flight, day time.Time, fl *dcs.Flight) error {
	if fl == nil || fl.Reconciliation == nil || len(fl.Reconciliation.NotLoaded) == 0 {
		return nil
	}
	short := fl.Reconciliation.NotLoaded
	key := dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}
	bags := make([]shortBag, 0, len(short))
	for _, b := range short {
		bags = append(bags, shortBag{tag: b.Tag, surname: b.Surname, ex: key})
	}
	next, ok := rushFlightFor(t.flights, f)
	if ok && t.rushDecision != nil && !t.rushDecision(ctx, f, day, len(short)) {
		ok = false
	}
	if !ok {
		t.recordShort(f, day, nil, bags)
		t.groundMu.Lock()
		t.rushed[key] = fmt.Sprintf("%d short-shipped, no later flight today: held for tomorrow", len(short))
		t.groundMu.Unlock()
		return nil
	}
	t.recordShort(f, day, &next, bags)
	wire := strings.ToUpper(day.Format("02Jan"))
	var firstErr error
	for _, b := range short {
		m := &baggage.Message{Kind: baggage.KindBUM, Version: "1" + f.From,
			Outbound: &baggage.FlightLeg{Flight: next.Carrier + next.Number, Date: wire, City: f.To},
			Tags:     []baggage.Tag{{Number: b.Tag, Count: 1}}, Surname: b.Surname,
			Elements: []string{".X/RUSH EX " + fl.Flight}}
		text, err := baggage.Build(m)
		if err != nil {
			return err
		}
		// Sortation loads it on the rush flight; the arrival station expects it.
		if err := t.sendTypeBFrom(ctx, t.stationAddress(f.From, deptCheckIn),
			[]string{t.stationAddress(f.From, deptBaggage), t.stationAddress(f.To, deptBaggage)}, text, "BUM"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	t.groundMu.Lock()
	t.rushed[key] = fmt.Sprintf("%d short-shipped, rushed on %s%s", len(short), next.Carrier, strings.TrimLeft(next.Number, "0"))
	t.groundMu.Unlock()
	t.log.Info("bags rushed", "flight", fl.Flight, "board", fl.Board, "bags", len(short), "on", next.Carrier+next.Number)
	return firstErr
}

// rushFlightFor is the carrier's next departure over the same sector after
// the flight given, today.
func rushFlightFor(flights []world.Flight, f world.Flight) (world.Flight, bool) {
	var best world.Flight
	found := false
	for _, g := range flights {
		if g.From != f.From || g.To != f.To || g.DepMin <= f.DepMin || g.Carrier != f.Carrier {
			continue
		}
		if !found || g.DepMin < best.DepMin {
			best, found = g, true
		}
	}
	return best, found
}

func (t *Tenant) rushedFor(fl *dcs.Flight) string {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.rushed[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}

// RushExpected is how many unaccompanied bags a station has been told to
// expect.
func (t *Tenant) RushExpected(station string) int {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.rushIn[station]
}

// SendClosure transmits a closure's messages. It is also the console's
// OnClose hook, so an agent closing a flight by hand sends the same set.
func (t *Tenant) SendClosure(ctx context.Context, cl *dcs.Closure) error {
	fl := cl.Flight
	leg, known := t.legOf(fl.Flight, fl.Board)
	tally := fl.Tally()
	// The door's count says who came only when the airport had the whole
	// list and its counter opened. A world that starts after a flight's
	// check-in hours, or that is still writing the day's bookings when
	// the name list goes, closes the flight on part of its passengers or
	// on none. Such a flight is not counted as flown that empty, and the
	// record of who came does not take it.
	whole := known && cl.Counts.Boarded > 0 && t.listedAll(leg, fl, tally)
	t.groundMu.Lock()
	t.boarded[fl.Flight+"/"+fl.Date] = cl.Counts.Boarded
	if t.carried != nil && whole {
		t.carried[fl.Flight+"/"+fl.Date+"/"+fl.Board] = cl.Counts.Boarded
	}
	t.groundMu.Unlock()

	var firstErr error
	checkIn, loadControl := t.stationAddress(fl.Board, deptCheckIn), t.stationAddress(fl.Board, deptLoad)
	send := func(from string, addrs []string, texts []string, kind string) {
		for _, text := range texts {
			if text == "" {
				continue
			}
			if err := t.sendTypeBFrom(ctx, from, addrs, text, kind); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	// Bags of passengers who did not board come off before anything else.
	for _, p := range cl.Offloaded {
		if err := t.announceBags(ctx, fl, p, "DEL"); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// What the passengers who flew bought beside the seat.
	if known {
		t.sellExtras(leg, fl)
	}
	// A passenger who took a seat at the door has bags that were tagged
	// then.
	for _, p := range cl.Onloaded {
		if err := t.announceBags(ctx, fl, p, ""); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	// The passengers turned away at the door are owed what the law says.
	// The volunteers were counted when they agreed.
	for _, p := range cl.Denied {
		if known && !p.Volunteer {
			t.noteDenied(leg, p)
		}
	}
	// Revenue management hears who came, cabin by cabin. A passenger who
	// checked in and did not fly, other than one denied boarding, left
	// the seat empty as a no-show does.
	if t.shown != nil && whole {
		for comp, c := range tally {
			t.shown(leg.Carrier, leg.Number, leg.From, comp, c.Booked, c.NoShows+c.Offloaded)
		}
	}
	send(checkIn, []string{t.Carrier.TTYAddress, t.watch}, cl.PFS, "PFS")
	send(checkIn, []string{t.stationAddress(fl.Dest, deptCheckIn)}, cl.PTM, "PTM")
	send(checkIn, []string{t.stationAddress(fl.Dest, deptCheckIn)}, cl.PSM, "PSM")
	send(checkIn, []string{t.stationAddress("HDQ", deptRevenue)}, cl.ETL, "ETL")
	send(loadControl, []string{t.stationAddress(fl.Dest, deptLoad), t.watch}, []string{cl.LDM}, "LDM")
	// A border crossed owes the arrival country its passenger list: who is on
	// board, with the document the record carried, before the aircraft
	// leaves. The agency's answer, when it has one, is its own message.
	if err := t.sendAPIS(ctx, cl.Flight); err != nil && firstErr == nil {
		firstErr = err
	}
	send(loadControl, []string{t.stationAddress(fl.Dest, deptLoad)}, []string{cl.CPM}, "CPM")
	return firstErr
}

// Cancelled says whether this carrier's departure control has the flight
// cancelled today.
func (t *Tenant) Cancelled(f world.Flight, day time.Time) bool { return t.isCancelled(f, day) }

// Boarded is the departure's boarded count at the door, 0 before it closed.
func (t *Tenant) Boarded(f world.Flight, day time.Time) int { return t.boardedFor(f, day) }

// listedAll reports whether the airport's list held the flight's
// bookings: at least 9 in 10 of the seats the carrier's inventory has
// sold on it. The few sold after the last list went are not on it.
func (t *Tenant) listedAll(leg world.Flight, fl *dcs.Flight, tally map[string]dcs.Tally) bool {
	if t.Inventory == nil || t.capacity == nil {
		return true
	}
	cabins, ok := t.capacity(leg.Carrier, leg.Number, fl.Date, leg.From)
	if !ok {
		return true
	}
	sold, listed := 0, 0
	for comp := range cabins {
		for _, n := range t.Inventory.SoldByClass(leg.Carrier, leg.Number, fl.Date, leg.From, comp) {
			sold += n
		}
	}
	for _, c := range tally {
		listed += c.Booked
	}
	return 10*listed >= 9*sold
}

// Carried is the passengers a departure left with, as its door counted
// them. ok is false for a flight departure control has not closed, and
// for one it closed without the whole list or without a counter. The
// count is kept until the day ends, after the flight has landed and its
// departure control record is gone.
func (t *Tenant) Carried(f world.Flight, day time.Time) (n int, ok bool) {
	key := flightKey(f, day)
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	n, ok = t.carried[key.Flight+"/"+key.Date+"/"+key.Board]
	return n, ok
}

// boardedFor is the departure's boarded count, for the movement message.
// A flight departure control never closed -- a boot mid-day, a chaos cut
// -- reports the schedule's typical load instead, and says so in the log.
func (t *Tenant) boardedFor(f world.Flight, day time.Time) int {
	key := flightKey(f, day)
	t.groundMu.Lock()
	n, ok := t.boarded[key.Flight+"/"+key.Date]
	t.groundMu.Unlock()
	if ok {
		return n
	}
	return f.Seats * 8 / 10
}

// Forget releases a flight's departure control record after it has landed.
func (t *Tenant) Forget(ctx context.Context, f world.Flight, day time.Time) {
	key := flightKey(f, day)
	_ = t.DCS.Forget(ctx, key)
	t.groundMu.Lock()
	delete(t.boarded, key.Flight+"/"+key.Date)
	delete(t.sortation, key.Flight+"/"+key.Date)
	delete(t.cancelled, key.Flight+"/"+key.Date)
	t.groundMu.Unlock()
}

// Summary is what the globe's drill-through shows about a departure.
type Summary struct {
	State  dcs.State  `json:"state"`
	Counts dcs.Counts `json:"counts"`
	Alerts int        `json:"alerts"`
	Dest   string     `json:"dest"`
	Board  string     `json:"board"`
	// Equipment and Version name the aircraft and its cabin: 321, Y220.
	Equipment    string `json:"equipment"`
	Version      string `json:"version"`
	Registration string `json:"registration,omitempty"`
	Crew         string `json:"crew,omitempty"`

	// The departure's clock: when control opened, when check-in closed,
	// when the flight closed. Sim time, as the station saw it.
	OpenedAt        time.Time  `json:"opened_at"`
	CheckInClosedAt *time.Time `json:"checkin_closed_at,omitempty"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	Cancelled       bool       `json:"cancelled,omitempty"`
	CancelReason    string     `json:"cancel_reason,omitempty"`

	// What reservations sent: how many PNL parts, whether the last arrived,
	// and how many ADLs followed.
	Parts    int  `json:"parts"`
	Complete bool `json:"complete"`
	ADLs     int  `json:"adls"`

	// Cabins is the seat map by compartment, with who holds it.
	Cabins []CabinSummary `json:"cabins"`
	// SSRs counts special service requests across the manifest, by code.
	SSRs map[string]int `json:"ssrs,omitempty"`
	// Inbound and Onward count passengers connecting onto and off this
	// flight; Infants and Children the passenger types on the list.
	Inbound  int `json:"inbound"`
	Onward   int `json:"onward"`
	Infants  int `json:"infants"`
	Children int `json:"children"`

	AlertList []dcs.Alert `json:"alert_list,omitempty"`

	// Load and Loadsheet exist once the flight has closed.
	Load      *dcs.Load `json:"load,omitempty"`
	Loadsheet string    `json:"loadsheet,omitempty"`
	// Bags is the door's reconciliation of the hold against the cabin.
	Bags *dcs.Reconciliation `json:"bags,omitempty"`
	// APIS is how many travellers the border agency was told about at the
	// door, when the flight crosses one.
	APIS int `json:"apis,omitempty"`
	// Substituted describes an aircraft change on the day, when there was one.
	Substituted string `json:"substituted,omitempty"`
	// PNRGOV is how many records the state's passenger information unit
	// was pushed for the flight, when it crosses a border.
	PNRGOV int `json:"pnrgov,omitempty"`
	// Retimed describes the day's time change, when the carrier announced
	// one to distribution.
	Retimed string `json:"retimed,omitempty"`
	// Rushed describes what became of the door's short-shipped bags.
	Rushed string `json:"rushed,omitempty"`
	// Traced is the bag office's story of them at the other end: the AHL
	// files raised when their passengers arrived without them, and the
	// matches that forwarded them.
	Traced string `json:"traced,omitempty"`
	// Slot is the flow management slot operations heard for the flight;
	// Delay is the day plan's account of the delay in its parts; Crew is
	// the crew's legality on the day. The last two are the world's, set
	// by the simulation, since the carrier does not hold the plan.
	Slot     string `json:"slot,omitempty"`
	Delay    string `json:"delay,omitempty"`
	CrewDuty string `json:"crew_duty,omitempty"`

	// Passengers is the manifest as departure control holds it, first
	// names first; Total says how long the whole list is.
	Passengers []PassengerRow `json:"passengers"`
	Total      int            `json:"total"`

	// Ops is the operations desk's side of the same flight.
	Ops OpsSummary `json:"ops"`
}

// CabinSummary is one compartment of the seat map.
type CabinSummary struct {
	Compartment string `json:"compartment"`
	Seats       int    `json:"seats"`
	Listed      int    `json:"listed"`
	Flying      int    `json:"flying"`
	Rows        string `json:"rows"` // e.g. "1-30 ABC DEF"
}

// PassengerRow is one line of the manifest.
type PassengerRow struct {
	Name     string `json:"name"` // as the wire writes it: SMITH/JOHNMR
	Class    string `json:"class"`
	Cabin    string `json:"cabin"`
	Seat     string `json:"seat,omitempty"`
	Status   string `json:"status"`
	Category string `json:"category,omitempty"`
	Sequence int    `json:"sequence,omitempty"`
	Bags     int    `json:"bags"`
	Locator  string `json:"locator,omitempty"`
	Onward   string `json:"onward,omitempty"`
	Inbound  string `json:"inbound,omitempty"`
	SSRs     string `json:"ssrs,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// OpsSummary is what the operations desk knows: the callsign it filed
// under, the type on the plan, whether the plan went, and what the towers
// have said back.
type OpsSummary struct {
	Callsign   string         `json:"callsign,omitempty"`
	ICAOType   string         `json:"icao_type,omitempty"`
	AFTN       string         `json:"aftn,omitempty"`
	FlightPlan bool           `json:"flight_plan"`
	ATS        map[string]int `json:"ats,omitempty"`
}

// Summarise reports a flight under departure control, by flight number in
// wire form (BA0117) or as the movement messages abbreviate it (BA117):
// everything the station and the ops desk hold on it. A carrier flies the
// same number over several legs in a day, so the boarding point picks the
// leg; "" takes whichever the station holds.
func (t *Tenant) Summarise(flight, board string) (*Summary, bool) {
	flight = normaliseFlight(flight)
	var fl *dcs.Flight
	for _, f := range t.DCS.Flights() {
		if f.Flight == flight && (board == "" || f.Board == board) {
			fl = f
			break
		}
	}
	if fl == nil {
		return nil, false
	}
	sum := &Summary{State: fl.State, Counts: fl.Counts(), Alerts: len(fl.Alerts), Dest: fl.Dest, Board: fl.Board,
		Equipment: fl.Equipment, Version: fl.Version, Registration: fl.Registration, Crew: fl.Crew,
		OpenedAt: fl.OpenedAt, CheckInClosedAt: fl.CheckInClosedAt, ClosedAt: fl.ClosedAt,
		Cancelled: fl.Cancelled, CancelReason: fl.CancelReason,
		Parts: len(fl.PartsSeen), Complete: fl.Complete, ADLs: fl.ADLs,
		AlertList: fl.Alerts, Load: fl.Load, Loadsheet: fl.Loadsheet, Bags: fl.Reconciliation, APIS: t.apisSentFor(fl), Substituted: t.substitutedFor(fl), PNRGOV: t.pnrgovSentFor(fl), Retimed: t.retimedFor(fl), Rushed: t.rushedFor(fl), Traced: t.tracedFor(fl), Slot: t.slotFor(fl), Total: len(fl.Passengers),
		SSRs: map[string]int{}, Passengers: []PassengerRow{}}
	listed, flying := map[string]int{}, map[string]int{}
	for _, p := range fl.Passengers {
		if p.Active() {
			listed[p.Compartment]++
		}
		if p.Flying() {
			flying[p.Compartment]++
		}
		for _, r := range p.SSRs {
			sum.SSRs[r.Code]++
		}
		if p.Inbound != nil {
			sum.Inbound++
		}
		if p.Onward != nil {
			sum.Onward++
		}
		switch p.Type {
		case dcs.PaxInfant:
			sum.Infants++
		case dcs.PaxChild:
			sum.Children++
		}
		if len(sum.Passengers) < 60 {
			row := PassengerRow{Name: p.Display(), Class: p.Class, Cabin: p.Compartment, Seat: p.Seat,
				Status: string(p.Status), Category: string(p.Category), Sequence: p.Sequence,
				Bags: len(p.Bags), Locator: p.Locator, Reason: p.OffloadReason}
			if p.Onward != nil {
				row.Onward = p.Onward.Flight + " " + p.Onward.Dest
				switch {
				case p.Onward.Refused != "":
					row.Onward += " · through check-in refused: " + p.Onward.Refused
				case p.Onward.Seat != "":
					row.Onward += " · through-checked, seat " + p.Onward.Seat
				case p.Onward.Carrier != "":
					row.Onward += " · through check-in requested"
				}
			}
			if p.Inbound != nil {
				row.Inbound = p.Inbound.Flight
			}
			var codes []string
			for _, r := range p.SSRs {
				codes = append(codes, r.Code)
			}
			row.SSRs = strings.Join(codes, " ")
			sum.Passengers = append(sum.Passengers, row)
		}
	}
	if fl.Cabin != nil {
		for _, sec := range fl.Cabin.Layout.Sections {
			perRow := len(strings.ReplaceAll(sec.Letters, " ", ""))
			sum.Cabins = append(sum.Cabins, CabinSummary{Compartment: sec.Compartment,
				Seats: perRow * (sec.ToRow - sec.FromRow + 1), Listed: listed[sec.Compartment], Flying: flying[sec.Compartment],
				Rows: fmt.Sprintf("%d-%d %s", sec.FromRow, sec.ToRow, sec.Letters)})
		}
	}
	if len(sum.SSRs) == 0 {
		sum.SSRs = nil
	}
	if wf, ok := t.legOf(flight, fl.Board); ok {
		sum.Ops.Callsign = Callsign(t.Carrier, wf)
		if typ, ok := icaoTypes[wf.Equipment]; ok {
			sum.Ops.ICAOType = typ.code
		}
		sum.Ops.AFTN = t.AFTNAddress(wf.From)
	}
	t.groundMu.Lock()
	sum.Ops.FlightPlan = t.filed[flight]
	if seen := t.atsByFlight[flight]; len(seen) > 0 {
		sum.Ops.ATS = map[string]int{}
		for k, v := range seen {
			sum.Ops.ATS[string(k)] = v
		}
	}
	t.groundMu.Unlock()
	return sum, true
}

// legOf finds the scheduled leg of a flight number that boards at a station.
func (t *Tenant) legOf(flight, board string) (world.Flight, bool) {
	for _, f := range t.flights {
		if f.Carrier+f.Number == flight && f.From == board {
			return f, true
		}
	}
	f, ok := t.flightsByNum[flight]
	return f, ok
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// pullBags tells the sortation system to take unaccompanied bags off the
// aircraft, one BSM DEL per passenger whose bags are named.
func (t *Tenant) pullBags(ctx context.Context, key dcs.Key, bags []dcs.BagRef) {
	fl, err := t.DCS.Flight(key)
	if err != nil {
		return
	}
	pulled := map[int]bool{}
	for _, b := range bags {
		if pulled[b.PassengerID] {
			continue
		}
		pulled[b.PassengerID] = true
		for _, p := range fl.Passengers {
			if p.ID == b.PassengerID {
				if err := t.announceBags(ctx, fl, p, "DEL"); err != nil {
					t.log.Debug("bag pull not announced", "flight", key.Flight, "tag", b.Tag, "err", err)
				}
			}
		}
	}
	t.log.Info("unaccompanied bags pulled at the door", "flight", key.Flight, "board", key.Board, "bags", len(bags))
}

// sendAPIS sends the flight-close passenger list to the border control
// agency when the flight lands in another country.
func (t *Tenant) sendAPIS(ctx context.Context, fl *dcs.Flight) error {
	if t.border == "" || t.countryOf == nil || fl == nil {
		return nil
	}
	from, to := t.countryOf(fl.Board), t.countryOf(fl.Dest)
	if from == "" || to == "" || from == to {
		return nil
	}
	m := gateway.APISFor(fl, gateway.APISOptions{
		Function: paxlst.FuncCloseOnBoard, OnBoardOnly: true,
		TxnRef:         fl.Flight + "-" + fl.Date + "-" + fl.Board,
		ContactSurname: t.Carrier.Designator + " OPS", ContactGiven: fl.Board,
	})
	if len(m.People) == 0 {
		return nil
	}
	peer := &gateway.Peer{Name: "net", Carrier: t.border, Format: store.FormatEDIFACT}
	if err := t.Gateway.SendAPIS(ctx, peer, m); err != nil {
		return err
	}
	t.groundMu.Lock()
	t.apisSent[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}] = len(m.People)
	t.groundMu.Unlock()
	return nil
}

func (t *Tenant) apisSentFor(fl *dcs.Flight) int {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.apisSent[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}

// Substitute is the aircraft going technical before departure: a smaller
// type from the fleet takes the flight. Departure control rebuilds the cabin
// and re-seats or displaces, the inventory's cabins shrink to the new
// aircraft's, and distribution hears the change as an ASM EQT -- under the
// marketing carrier's code too when the leg is a codeshare -- so the
// bookings land on the schedule-change queue for agents to reprotect. It
// returns the type substituted, or "" when the fleet offers nothing smaller.
func (t *Tenant) Substitute(ctx context.Context, f world.Flight, day time.Time) (string, error) {
	if t.isCancelled(f, day) {
		return "", nil
	}
	key := flightKey(f, day)
	fl, err := t.DCS.Flight(key)
	if err != nil || fl.State == dcs.StateClosed {
		return "", nil
	}
	best := SmallerType(fl.Equipment)
	if best == "" {
		return "", nil
	}
	reg := "SUB" + strings.TrimLeft(f.Number, "0")
	ch, err := t.DCS.ChangeEquipment(ctx, key, dcs.Equipment{Type: best, Registration: reg, Dest: f.To, Crew: crewFor(best)})
	if err != nil {
		return "", err
	}
	t.overrides.set(f.Carrier, f.Number, f.From, best)
	t.groundMu.Lock()
	t.substituted[key] = fmt.Sprintf("%s → %s, %d re-seated, %d denied boarding", ch.From, ch.To, len(ch.Reseated), len(ch.Displaced))
	t.groundMu.Unlock()
	err = t.sendASM(ctx, f, day, fmt.Sprintf("EQT\n%s%s/%s\nJ %s\n%s", "%s", "%s", "%s", best, legLine(f, f.DepMin, f.ArrMin)))
	t.log.Info("aircraft substituted", "flight", key.Flight, "board", key.Board, "from", ch.From, "to", ch.To,
		"kept", ch.Kept, "reseated", len(ch.Reseated), "displaced", len(ch.Displaced))
	return best, err
}

// sendASM sends one ad hoc schedule message about a flight to every
// distribution system, and the same again under the marketing carrier's
// code when the leg is a codeshare: a change only the operating code hears
// about strands the codeshare's bookings. The body names the flight with
// three %s verbs for carrier, number and date.
func (t *Tenant) sendASM(ctx context.Context, f world.Flight, day time.Time, body string) error {
	date := strings.ToUpper(day.Format("02Jan"))
	err := t.SendSchedule(ctx, "ASM\nUTC\n"+fmt.Sprintf(body, f.Carrier, f.Number, date))
	if f.Marketing != "" && f.Marketing != f.Carrier && f.MarketingNumber != "" {
		if merr := t.SendSchedule(ctx, "ASM\nUTC\n"+fmt.Sprintf(body, f.Marketing, f.MarketingNumber, date)); merr != nil && err == nil {
			err = merr
		}
	}
	return err
}

// legLine is the schedule message's leg: board point and time, off point
// and time, the way the SSIM chapter 7 leg line reads them.
func legLine(f world.Flight, depMin, arrMin int) string {
	return fmt.Sprintf("%s%s %s%s", f.From, hhmmOf(depMin), f.To, hhmmOf(arrMin))
}

// Retime is the carrier learning, a couple of hours out, that the flight
// will leave late enough to tell people: an ASM TIM with the new times goes
// to every distribution system, whose held segments move to the new times
// at TK -- confirming, advise times changed -- and whose agents get the
// task. Short delays are not announced; the movement message says what
// happened, the schedule change says what will.
func (t *Tenant) Retime(ctx context.Context, f world.Flight, day time.Time, depDelay, arrDelay int) error {
	if depDelay <= 0 || t.isCancelled(f, day) {
		return nil
	}
	key := flightKey(f, day)
	t.groundMu.Lock()
	_, done := t.retimed[key]
	t.groundMu.Unlock()
	if done {
		return nil
	}
	if arrDelay < 0 {
		arrDelay = 0
	}
	err := t.sendASM(ctx, f, day, fmt.Sprintf("TIM\n%s%s/%s\n%s", "%s", "%s", "%s", legLine(f, f.DepMin+depDelay, f.ArrMin+arrDelay)))
	if err != nil {
		return err
	}
	t.groundMu.Lock()
	t.retimed[key] = fmt.Sprintf("STD %s → ETD %s (+%d), ASM TIM to distribution", hhmmOf(f.DepMin), hhmmOf(f.DepMin+depDelay), depDelay)
	t.groundMu.Unlock()
	t.log.Info("flight retimed", "flight", key.Flight, "board", key.Board, "delay", depDelay)
	return nil
}

func (t *Tenant) retimedFor(fl *dcs.Flight) string {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.retimed[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}

// PushPNRGOV is the reservations push a state's PNR regime takes before
// departure -- here once, when the name list goes to the airport -- with
// the records as booked and no check-in data yet. The door's push follows
// at close with the seats and bags.
func (t *Tenant) PushPNRGOV(ctx context.Context, f world.Flight, day time.Time) error {
	if t.isCancelled(f, day) || !t.crossesBorder(f.From, f.To) {
		return nil
	}
	key := flightKey(f, day)
	fl, err := t.DCS.Flight(key)
	if err != nil {
		// Not under control yet: the push is the reservation's alone.
		fl = &dcs.Flight{Key: key, Carrier: f.Carrier, Dest: f.To}
	}
	return t.sendPNRGOV(ctx, f, day, fl)
}

// crossesBorder reports whether a sector leaves one country for another,
// as far as the world's country table knows.
func (t *Tenant) crossesBorder(from, to string) bool {
	if t.border == "" || t.countryOf == nil {
		return false
	}
	a, b := t.countryOf(from), t.countryOf(to)
	return a != "" && b != "" && a != b
}

// sendPNRGOV pushes the flight's records to the state, and remembers how
// many for the panel.
func (t *Tenant) sendPNRGOV(ctx context.Context, f world.Flight, day time.Time, fl *dcs.Flight) error {
	if fl == nil || !t.crossesBorder(fl.Board, fl.Dest) {
		return nil
	}
	push, err := t.Gateway.PNRGOVFor(ctx, fl, gateway.PNRGOVOptions{
		Departs: day.Add(time.Duration(f.DepMin) * time.Minute), Arrives: day.Add(time.Duration(f.ArrMin) * time.Minute), Station: f.From,
	})
	if err != nil {
		return err
	}
	if len(push.Records) == 0 {
		return nil
	}
	peer := &gateway.Peer{Name: "net", Carrier: t.border, Format: store.FormatEDIFACT}
	if err := t.Gateway.SendPNRGOV(ctx, peer, push); err != nil {
		return err
	}
	t.groundMu.Lock()
	t.pnrgovSent[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}] = len(push.Records)
	t.groundMu.Unlock()
	return nil
}

func (t *Tenant) pnrgovSentFor(fl *dcs.Flight) int {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.pnrgovSent[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}

func hhmmOf(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d%02d", min/60, min%60)
}

// Downgauged reports whether a leg flies a smaller aircraft than it sold:
// its inventory may then rightly hold more than the cabin, and the live
// invariant check says so rather than calling it an oversell.
func (t *Tenant) Downgauged(flightNum, board string) bool {
	return t.overrides != nil && t.overrides.get(t.Carrier.Designator, flightNum, board) != ""
}

func (t *Tenant) substitutedFor(fl *dcs.Flight) string {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.substituted[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}

// SmallerType is the aircraft the fleet would substitute for one that has
// gone technical: the nearest smaller type, between three fifths and
// nineteen twentieths of the seats -- a real substitution, not a toy -- or
// "" when the fleet offers none.
func SmallerType(current string) string {
	fleet := fleetData()
	cur, ok := fleet.Type(current)
	if !ok {
		return ""
	}
	have := cur.Cabin.Seats()
	best, bestSeats := "", 0
	for code, at := range fleet.Types {
		n := at.Cabin.Seats()
		if n >= have || n < have*3/5 || n > have*19/20 {
			continue
		}
		if n > bestSeats || (n == bestSeats && code < best) {
			best, bestSeats = code, n
		}
	}
	return best
}
