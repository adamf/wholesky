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
	"fmt"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/baggage"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/jetway/pkg/pnl"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/jetway/pkg/typeb"

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

// crewFor is the cockpit/cabin complement the LDM reports, by type.
func crewFor(equipment string) string {
	switch equipment {
	case "AT7":
		return "2/2"
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
	t.Gateway.Ground = t
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
// synthetic behaviour lives: a few in a hundred no-show; a share connect
// off an inbound flight and miss the connection when that flight is late.
func (t *Tenant) arrival(pt party, f world.Flight, day time.Time) (minutesOut int, inbound *dcs.Connection, ok bool) {
	h := hashOf(pt.key + f.Carrier + f.Number + day.Format("0102"))
	r := h % 100
	switch {
	case r < 4:
		return 0, nil, false // no-show
	case r < 18:
		// Connecting off one of this carrier's own arrivals into f.From.
		inb, found := t.inboundFor(f, h)
		if !found {
			break
		}
		delay := 0
		if t.inboundDelay != nil {
			delay = t.inboundDelay(inb, day)
		}
		arrives := f.DepMin - (inb.ArrMin%1440 + delay) // minutes before departure
		if arrives < minimumConnection {
			return 0, nil, false // misconnected: they are not on this flight
		}
		conn := &dcs.Connection{Flight: inb.Carrier + inb.Number, Date: strings.ToUpper(day.Format("02Jan")), Station: f.From, Dest: f.To}
		return min(arrives-10, 170), conn, true
	}
	return 46 + (h/100)%125, nil, true // 46..170 minutes before departure
}

// minimumConnection is the minutes a connection needs at any station here.
const minimumConnection = 45

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
		if gap >= minimumConnection && gap <= 300 {
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
	var cands []world.Flight
	arr := f.ArrMin % 1440
	for _, g := range t.flights {
		if g.From != f.To || g.Carrier != f.Carrier {
			continue
		}
		gap := g.DepMin - arr
		if gap < 0 {
			gap += 1440
		}
		if gap >= minimumConnection && gap <= 300 {
			cands = append(cands, g)
		}
	}
	if len(cands) == 0 {
		return nil
	}
	g := cands[(h/7)%len(cands)]
	return &dcs.Connection{Flight: g.Carrier + g.Number, Date: strings.ToUpper(day.Format("02Jan")), Station: f.To, Dest: g.To, Class: "Y"}
}

// bagsFor decides what a party checks: about six in ten check something,
// one or two pieces, at real-looking weights.
func bagsFor(h int) []int {
	if (h/7)%10 >= 6 {
		return nil
	}
	n := 1 + (h/70)%2
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, 12+(h/(13*(i+1)))%17)
	}
	return out
}

// CheckIn is one wave of the counter: every listed party that has arrived
// by now and has not yet been accepted is checked in, seated, tagged, and
// its bags announced to the sortation system.
func (t *Tenant) CheckIn(ctx context.Context, f world.Flight, day time.Time, minutesOut int) error {
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
			Bags: bagsFor(h), Onward: t.onwardFor(f, day, h), Inbound: inbound,
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
		if len(acc.Tags) > 0 {
			if err := t.announceBags(ctx, acc.Flight, acc.Passengers[0], ""); err != nil && firstErr == nil {
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

// CloseCheckIn closes the counter; standbys clear into whatever is left.
func (t *Tenant) CloseCheckIn(ctx context.Context, f world.Flight, day time.Time) error {
	_, _, err := t.DCS.CloseCheckIn(ctx, flightKey(f, day))
	return err
}

// Board is one wave of the gate: every accepted passenger whose turn has
// come walks on. One in a few hundred never does.
func (t *Tenant) Board(ctx context.Context, f world.Flight, day time.Time, minutesOut int) error {
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
	key := flightKey(f, day)
	h := hashOf(f.Carrier + f.Number + day.Format("0102"))
	cargo, mail := cargoFor(f, h)
	cl, err := t.DCS.CloseFlight(ctx, key, dcs.CloseOptions{Fuel: fuelFor(f), Cargo: cargo, Mail: mail, Force: true})
	if err != nil {
		return err
	}
	return t.SendClosure(ctx, cl)
}

// SendClosure transmits a closure's messages. It is also the console's
// OnClose hook, so an agent closing a flight by hand sends the same set.
func (t *Tenant) SendClosure(ctx context.Context, cl *dcs.Closure) error {
	fl := cl.Flight
	t.groundMu.Lock()
	t.boarded[fl.Flight+"/"+fl.Date] = cl.Counts.Boarded
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
	send(checkIn, []string{t.Carrier.TTYAddress, t.watch}, cl.PFS, "PFS")
	send(checkIn, []string{t.stationAddress(fl.Dest, deptCheckIn)}, cl.PTM, "PTM")
	send(checkIn, []string{t.stationAddress(fl.Dest, deptCheckIn)}, cl.PSM, "PSM")
	send(checkIn, []string{t.stationAddress("HDQ", deptRevenue)}, cl.ETL, "ETL")
	send(loadControl, []string{t.stationAddress(fl.Dest, deptLoad), t.watch}, []string{cl.LDM}, "LDM")
	send(loadControl, []string{t.stationAddress(fl.Dest, deptLoad)}, []string{cl.CPM}, "CPM")
	return firstErr
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
}

// Summarise reports a flight under departure control, by flight number in
// wire form (BA0117) or as the movement messages abbreviate it (BA117).
func (t *Tenant) Summarise(flight string) (*Summary, bool) {
	if len(flight) > 2 {
		num := strings.TrimLeft(flight[2:], "0")
		for len(num) < 4 {
			num = "0" + num
		}
		flight = flight[:2] + num
	}
	fl, ok := t.DCS.Find(flight, "")
	if !ok {
		return nil, false
	}
	return &Summary{State: fl.State, Counts: fl.Counts(), Alerts: len(fl.Alerts), Dest: fl.Dest, Board: fl.Board,
		Equipment: fl.Equipment, Version: fl.Version, Registration: fl.Registration}, true
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
