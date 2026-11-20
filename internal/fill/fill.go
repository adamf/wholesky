// Package fill writes the bookings a day already has before the day runs.
//
// The demand generator books the way a day of travel books: sells arriving
// through the distribution systems as the clock runs. That is the right
// shape for the trickle on the day, and the wrong one for the load a flight
// carries: on the morning of a real travel day nearly every seat that will
// fly was sold weeks ago. This package is those weeks. It reads a compiled
// schedule and produces, for each operating carrier, the records that would
// be holding seats at midnight -- parties, itineraries that connect on legs
// that actually connect, classes, special service requests, tickets, the
// carrier's own locator and the selling channel's -- deterministically from
// a seed, so a day can be filled again after it has been purged and come
// out the same.
//
// Who holds those seats is decided by the same market the day's own
// travellers are (internal/market). For each flight a pool of travellers
// is drawn, and they buy in the order they would have: the ones who book
// months ahead first. Each takes the fare that costs them least among
// those the rules will sell that far out, in the channel they shop in,
// in a class the flight's nested limits still have room in, or goes
// without when every fare left is more than they will pay. So the early
// leisure travellers take the deep discounts until those close, the
// later ones pay more, and the seats the limits held back go to the
// travellers who book in the last week at the fares that week sells.
// What a flight carries and what it earned both come out of that, and a
// carrier that raises its fares fills fewer seats.
//
// It writes only the carrier's book of record. A record sold through a
// distribution system carries that system's locator, but the system's own
// copy is not written: the recorded day treats what was sold before the day
// as already at the carrier, which is where check-in and the name list
// read it from.
package fill

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/inventory"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// Options shape a fill.
type Options struct {
	// LoadFactor is the share of each flight's seats holding a booking at
	// midnight, before the day's own selling. Flights vary around it. Zero
	// fills nothing.
	LoadFactor float64
	// Seed makes the fill reproducible; the same seed fills the same day.
	Seed int64
	// Day is the flown date: segments depart on it and carry its wire date.
	Day time.Time
	// Channels are the distribution systems that sold the records, by
	// designator; Direct is the share sold by the carrier itself. Default
	// 1G, 1S, 1A and a quarter direct.
	Channels []string
	Direct   float64
	// Connecting is the share of parties that continue onto a second leg of
	// the same carrier from where the first one lands. Default a fifth.
	Connecting float64
	// Batch is how many records reach the sink at once. Default 2000.
	Batch int
	// Marketed is the share of a codeshare leg's parties sold under the
	// marketing carrier's code rather than the operator's. Default nine in
	// ten: a regional flying for a major sells almost nothing itself.
	Marketed float64
	// Cabins is the seats a flight offers by compartment, so a party is put
	// in a cabin that has room for it on every leg: the business share of
	// a Hawaii widebody is not the business cabin's size. Nil is one
	// economy cabin of the flight's seats.
	Cabins func(f world.Flight) map[string]int
	// Cellos is the share of parties travelling with an instrument too
	// precious for the hold. It rides in a seat of its own, booked as a
	// name of its own -- SURNAME/CBBG -- with the SSR that tells the
	// airport what is sitting in 14C. Default one party in three thousand.
	Cellos float64
	// Secret is the locator secret convention the carriers' systems use, so
	// filled locators are allocated from the same permutation as live ones
	// and cannot collide with them: the fill takes counters from the top of
	// the space, live selling from the bottom.
	Secret func(code string) []byte
	// Accounting gives a carrier's three-digit accounting code, which leads
	// its ticket numbers. Default derives one from the designator.
	Accounting func(code string) string
	// Tariff, when set, is the fares the travellers are shown and prices
	// every record as of the day it was booked: the fare basis on the
	// segment, the money on the record and on the coupons. Without one the
	// classes are drawn at random and nothing is priced.
	Tariff fare.Tariff
	// Base is a market's lowest regular fare in cents, from which a
	// traveller's willingness to pay is measured. Nil takes it from the
	// carrier's own full fare.
	Base func(origin, destination string, km int) int64
	// Mix is a market's travellers. Nil is the default mix for the
	// market's length.
	Mix func(origin, destination string, km int) market.Mix
	// Offered reports whether a carrier is sold in a channel. Nil is
	// everywhere.
	Offered func(carrier, channel string) bool
	// Seller names who sold a record that the carrier did not sell
	// itself: the agency's number, the distribution system it booked
	// through, and whether it booked over a direct connection to the
	// carrier, in which case no system's locator is on the record. Nil
	// leaves the agent a number of the system's own.
	Seller func(rng *rand.Rand, kind, carrier string, f world.Flight) (agent, system string, connected bool)
	// Connect is the least time a passenger needs between two flights at
	// the station where the first lands, in minutes: the minimum
	// connecting time. Nil is 45 minutes everywhere.
	Connect func(in, out world.Flight) int
	// Share scales a flight's pool of travellers: over one for a carrier
	// priced under the others in its market, under one for a carrier priced
	// over them. Nil is one.
	Share func(f world.Flight) float64
	// RM, when set, is the carriers' revenue management memory. Each
	// flight's booking window is written to it, frame by frame: which
	// class was the lowest on sale, for what share of the travellers who
	// looked, and what was sold. A flight whose record is long enough
	// (rm.Warm departures) is sold under the limits its own record sets,
	// worked out again at the start of each time frame. A flight without
	// one is sold under the tariff's fixed limits. Nil keeps no record and
	// sells every flight under the fixed limits.
	RM *rm.Memory
	// Remembered says the records already hold this day's booking
	// windows, as when a day is filled a second time: the flights are
	// sold under the records and nothing is added to them.
	Remembered bool
	// Dry sells the day and writes no records: the sink is not called.
	// The carriers' revenue management still gets its record, which is
	// what a burn-in is for, and the plan still counts what was sold.
	Dry bool
}

// pool is how many travellers consider a flight for each seat it is meant
// to carry. Some find every fare left too dear and stay at home, so the
// pool is larger than the load: measured over a filled world at the
// fares as filed, with each flight's limits set from its own record, 100
// travellers in the pool put 80 in seats. On the tariff's fixed limits
// they put 57 in seats, and the pool was 100 for each 57 while every
// flight sold that way.
const pool = 1.0 / 0.80

// Plan is what a fill produced.
type Plan struct {
	Carriers   int
	Flights    int
	Records    int
	Passengers int
	Seats      int // seats offered on the flights filled
	Connecting int // records with two legs
	// Marketed counts the records sold under a marketing carrier's code on
	// a leg another carrier flies: each is two records, one at each.
	Marketed int
	// Sold is the seats sold to the market's travellers, leg by leg, Fares
	// what they were asked for them in cents at the adult fare, and
	// Classes the seats by booking class.
	Sold    int
	Fares   int64
	Classes map[string]int
}

// Sink receives one carrier's records in batches, in a stable order.
type Sink func(ctx context.Context, carrier string, recs []*pnr.PNR) error

// counterBase is where filled locator counters start: the top half of the
// locator space, which live selling, counting up from one, never reaches.
const counterBase = uint64(1) << 29

func (o Options) defaults() Options {
	if len(o.Channels) == 0 {
		o.Channels = []string{"1G", "1S", "1A"}
	}
	if o.Direct == 0 {
		o.Direct = 0.25
	}
	if o.Connecting == 0 {
		o.Connecting = 0.2
	}
	if o.Batch <= 0 {
		o.Batch = 2000
	}
	if o.Cellos == 0 {
		o.Cellos = 1.0 / 3000
	}
	if o.Marketed == 0 {
		o.Marketed = 0.9
	}
	if o.Secret == nil {
		o.Secret = func(code string) []byte { return []byte("wholesky-" + code) }
	}
	if o.Accounting == nil {
		o.Accounting = func(code string) string {
			h := fnv.New32a()
			h.Write([]byte(code))
			return fmt.Sprintf("%03d", h.Sum32()%1000)
		}
	}
	if o.Day.IsZero() {
		o.Day = time.Now().UTC().Truncate(24 * time.Hour)
	}
	return o
}

// Day fills every carrier's flights for the day and hands each carrier's
// records to the sink. Carriers are filled in designator order, flights in
// departure order, so the output is stable for a seed.
func Day(ctx context.Context, m *world.Manifest, opts Options, sink Sink) (Plan, error) {
	opts = opts.defaults()
	var plan Plan
	if opts.LoadFactor <= 0 {
		return plan, nil
	}
	byCarrier := map[string][]world.Flight{}
	for _, f := range m.Flights {
		byCarrier[f.Carrier] = append(byCarrier[f.Carrier], f)
	}
	codes := make([]string, 0, len(byCarrier))
	for c := range byCarrier {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	// One allocator per selling channel, shared across carriers, so a
	// channel's locators are unique across the day the way they are in
	// the channel's own system.
	channels := map[string]*allocator{}
	for _, ch := range opts.Channels {
		channels[ch] = &allocator{a: pnr.NewLocatorAllocator(opts.Secret(ch)), next: counterBase}
	}
	plans, errs := make([]Plan, len(codes)), make([]error, len(codes))
	if opts.Dry {
		// A dry day writes nothing and shares nothing between carriers, so
		// the carriers sell side by side.
		var wg sync.WaitGroup
		gate := make(chan struct{}, runtime.GOMAXPROCS(0))
		for i, code := range codes {
			wg.Add(1)
			gate <- struct{}{}
			go func() {
				defer func() { <-gate; wg.Done() }()
				plans[i], errs[i] = fillCarrier(ctx, code, byCarrier[code], opts, channels, sink)
			}()
		}
		wg.Wait()
	}
	for i, code := range codes {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		p, err := plans[i], errs[i]
		if !opts.Dry {
			p, err = fillCarrier(ctx, code, byCarrier[code], opts, channels, sink)
		}
		if err != nil {
			return plan, fmt.Errorf("fill %s: %w", code, err)
		}
		plan.Carriers++
		plan.Flights += p.Flights
		plan.Records += p.Records
		plan.Passengers += p.Passengers
		plan.Seats += p.Seats
		plan.Connecting += p.Connecting
		plan.Marketed += p.Marketed
		plan.Sold += p.Sold
		plan.Fares += p.Fares
		if plan.Classes == nil {
			plan.Classes = map[string]int{}
		}
		for c, n := range p.Classes {
			plan.Classes[c] += n
		}
	}
	return plan, nil
}

type allocator struct {
	a    *pnr.LocatorAllocator
	next uint64
}

func (a *allocator) take() string {
	l := a.a.Allocate(a.next)
	a.next++
	return l
}

// fillCarrier fills one carrier's day.
func fillCarrier(ctx context.Context, code string, flights []world.Flight, opts Options, channels map[string]*allocator, sink Sink) (Plan, error) {
	plan := Plan{Classes: map[string]int{}}
	sort.Slice(flights, func(i, j int) bool {
		if flights[i].DepMin != flights[j].DepMin {
			return flights[i].DepMin < flights[j].DepMin
		}
		if flights[i].Number != flights[j].Number {
			return flights[i].Number < flights[j].Number
		}
		return flights[i].From < flights[j].From
	})
	h := fnv.New64a()
	h.Write([]byte(code))
	rng := rand.New(rand.NewSource(opts.Seed ^ int64(h.Sum64())))
	own := &allocator{a: pnr.NewLocatorAllocator(opts.Secret(code)), next: counterBase}
	acct := opts.Accounting(code)
	var ticketSeq uint64 = 2_000_000_000
	// The marketing carriers' records go to their own books; each marketing
	// carrier allocates its own locators, from a range no live sale reaches.
	others := map[string]*allocator{}
	marketedOut := map[string][]*pnr.PNR{}
	ownerFor := func(cr string) *allocator {
		a, ok := others[cr]
		if !ok {
			// The same base for every carrier would repeat itself across
			// books; offset by the operating carrier so the marketing
			// carrier's records from different operators do not collide.
			h := fnv.New32a()
			h.Write([]byte(code))
			a = &allocator{a: pnr.NewLocatorAllocator(opts.Secret(cr)), next: counterBase + uint64(h.Sum32()%1000)*100000}
			others[cr] = a
		}
		return a
	}

	// Targets per flight, and what connections have already put on each.
	target := make([]int, len(flights))
	assigned := make([]int, len(flights))
	byFrom := map[string][]int{} // flight indexes by boarding point, in departure order
	for i, f := range flights {
		target[i] = int(math.Round(float64(f.Seats) * loadFactor(opts.LoadFactor, f, rng)))
		byFrom[f.From] = append(byFrom[f.From], i)
		plan.Seats += f.Seats
	}
	plan.Flights = len(flights)

	// onward finds a later leg of this carrier out of where f lands that a
	// party of n can still be put on: from the station's minimum connecting
	// time to five hours after arrival, the way a connection is built.
	onward := func(i, n int) int {
		f := flights[i]
		cands := byFrom[f.To]
		lo, hi := f.ArrMin+45, f.ArrMin+300
		if opts.Connect != nil {
			lo = f.ArrMin + 20 // no station connects faster
		}
		start := sort.Search(len(cands), func(k int) bool { return flights[cands[k]].DepMin >= lo })
		var fit []int
		for k := start; k < len(cands) && flights[cands[k]].DepMin <= hi; k++ {
			j := cands[k]
			if opts.Connect != nil && flights[j].DepMin < f.ArrMin+opts.Connect(f, flights[j]) {
				continue
			}
			if j != i && assigned[j]+n <= target[j] && flights[j].To != f.From {
				fit = append(fit, j)
				if len(fit) == 4 {
					break
				}
			}
		}
		if len(fit) == 0 {
			return -1
		}
		return fit[rng.Intn(len(fit))]
	}

	// Seats sold per compartment on each flight; the class a party draws
	// has to fit its cabin on every leg, or it is moved.
	sold := make([]map[string]int, len(flights))
	for i := range sold {
		sold[i] = map[string]int{}
	}
	cabins := make([]map[string]int, len(flights))
	cabinsOf := func(i int) map[string]int {
		if cabins[i] != nil {
			return cabins[i]
		}
		cabins[i] = map[string]int{"Y": flights[i].Seats}
		if opts.Cabins != nil {
			if c := opts.Cabins(flights[i]); len(c) > 0 {
				cabins[i] = c
			}
		}
		return cabins[i]
	}
	fits := func(idx []int, class string, size int) bool {
		for _, i := range idx {
			cab := cabinsOf(i)
			if comp := inventory.CompartmentFor(class, cab); sold[i][comp]+size > cab[comp] {
				return false
			}
		}
		return true
	}
	// place draws a class the cabins can take on every leg and books the
	// seats in them. A full premium cabin sends the party to economy; a
	// full economy cabin upgrades it if a premium cabin has room; when no
	// cabin has room the answer is empty and the flight is full.
	place := func(idx []int, size int) string {
		class := bookingClass(rng)
		if !fits(idx, class, size) {
			alts := []string{"Y", "B", "M", "H", "Q", "K", "J", "F"}
			switch class {
			case "F", "A", "P", "R", "J", "C", "D", "I", "Z":
				alts = alts[:6]
			default:
				alts = alts[6:]
			}
			class = ""
			for _, alt := range alts[rng.Intn(len(alts)):] {
				if fits(idx, alt, size) {
					class = alt
					break
				}
			}
			if class == "" {
				return ""
			}
		}
		for _, i := range idx {
			sold[i][inventory.CompartmentFor(class, cabinsOf(i))] += size
		}
		return class
	}

	// The market the travellers shop: each market's fares, cheapest
	// first, and each cabin's nested limits.
	type rung struct {
		class           string
		amount          int64
		ap              int
		refundable, fee bool
		channels, accts []string
	}
	rungsOf := map[string][]rung{}
	rungs := func(o, d string) []rung {
		if r, ok := rungsOf[o+"/"+d]; ok {
			return r
		}
		var out []rung
		for _, f := range opts.Tariff.Fares(code, o, d) {
			// The fill's bookings are one way: a fare that wants a return
			// or a Saturday night away is not for them.
			if f.RoundTrip || f.Rule.SaturdayNight || f.OneWay.Amount <= 0 {
				continue
			}
			out = append(out, rung{class: f.Class, amount: f.OneWay.Amount, ap: f.Rule.AdvancePurchaseDays, refundable: f.Rule.Refundable,
				fee: f.Rule.ChangeFee.Amount > 0 || f.Rule.NoChanges, channels: f.Rule.Sale.Channels, accts: f.Rule.Sale.Accounts})
		}
		sort.SliceStable(out, func(a, b int) bool { return out[a].amount < out[b].amount })
		rungsOf[o+"/"+d] = out
		return out
	}
	// across is the fares for a trip over two legs: the carrier's through
	// fares where it files them, and otherwise each class's two local
	// fares end on end.
	across := func(a, b world.Flight) []rung {
		if r := rungs(a.From, b.To); len(r) > 0 {
			return r
		}
		key := a.From + "/" + a.To + "/" + b.To
		if r, ok := rungsOf[key]; ok {
			return r
		}
		var out []rung
		for _, x := range rungs(a.From, a.To) {
			for _, y := range rungs(b.From, b.To) {
				if x.class != y.class || len(x.accts) != len(y.accts) || len(x.channels) != len(y.channels) {
					continue
				}
				out = append(out, rung{class: x.class, amount: x.amount + y.amount, ap: max(x.ap, y.ap), refundable: x.refundable && y.refundable,
					fee: x.fee || y.fee, channels: x.channels, accts: x.accts})
				break
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].amount < out[j].amount })
		rungsOf[key] = out
		return out
	}
	limits := make([]map[string]int, len(flights)) // by class, the seats it and the classes under it may take
	limitOf := func(i int, class string) (int, bool) {
		if limits[i] == nil {
			limits[i] = map[string]int{}
			for comp, seats := range cabinsOf(i) {
				for _, a := range tariff.Authorizations(comp, seats) {
					limits[i][a.Class] = a.Authorized
				}
			}
		}
		n, ok := limits[i][class]
		return n, ok
	}
	has := func(list []string, v string) bool {
		if len(list) == 0 {
			return true
		}
		for _, x := range list {
			if strings.EqualFold(x, v) {
				return true
			}
		}
		return false
	}
	// Revenue management, cabin by cabin: the record the carrier keeps of
	// this departure, and the limits its earlier departures set.
	revman := make([]map[string]*cabinRM, len(flights))
	rmOf := func(i int, comp string) *cabinRM {
		if opts.RM == nil {
			return nil
		}
		if c := revman[i][comp]; c != nil {
			return c
		}
		f := flights[i]
		c := &cabinRM{seats: cabinsOf(i)[comp], at: map[string]int{}}
		for _, cl := range tariff.Ladder() {
			if cabinClass(market.CabinOf(cl)) == comp {
				c.at[cl] = len(c.classes)
				c.classes = append(c.classes, cl)
			}
		}
		c.fare, c.ap = make([]float64, len(c.classes)), make([]int, len(c.classes))
		for k := range c.ap {
			c.ap[k] = -1
		}
		for _, r := range rungs(f.From, f.To) {
			// The class's fare is the one anyone may buy; a contract's
			// fare in the same class is a discount on it.
			if k, ok := c.at[r.class]; ok && (c.ap[k] < 0 || (len(r.accts) == 0 && float64(r.amount) > c.fare[k])) {
				c.fare[k], c.ap[k] = float64(r.amount), r.ap
			}
		}
		// The record is kept in shares of the cabin's highest fare, so a
		// carrier that moves all its fares does not find its record in
		// yesterday's money.
		c.top = 1
		if len(c.fare) > 0 && c.fare[0] > 0 {
			c.top = c.fare[0]
		}
		c.cab = opts.RM.Cabin(rm.Key(code, f.Number, f.From, comp), c.classes)
		c.log, c.learned = c.cab.Begin(), c.cab.Learned()
		for g := range c.book {
			c.book[g] = make([]int, len(c.classes))
		}
		if revman[i] == nil {
			revman[i] = map[string]*cabinRM{}
		}
		revman[i][comp] = c
		return c
	}
	// isOpen reports whether a class has room for a party on a flight: in
	// the cabin, and under the class's limit. The limit is the tariff's
	// fixed one, counted on all the cabin has sold, until the flight's own
	// record sets it. Then it is the seats left at the start of the time
	// frame, less the seats the record holds back from the class, less
	// what the frame has sold in the class and below it.
	isOpen := func(i int, comp, class string, frame, party int) bool {
		cab := cabinsOf(i)
		if sold[i][comp]+party > cab[comp] {
			return false
		}
		if c := rmOf(i, comp); c != nil && c.learned {
			if k, ok := c.at[class]; ok {
				ctl := c.controls(frame)[k]
				if ctl.Closed {
					return false
				}
				left := c.seats - int(math.Round(ctl.Protect))
				for g := 0; g < frame; g++ {
					left -= c.total[g]
				}
				for _, n := range c.book[frame][k:] {
					left -= n
				}
				return left >= party
			}
		}
		limit, ok := limitOf(i, class)
		if !ok || limit > cab[comp] {
			limit = cab[comp]
		}
		return sold[i][comp]+party <= limit
	}
	// lowest is the lowest class on sale in a cabin for a party in a time
	// frame: the strategy in force, as the carrier's record names it. A
	// class is on sale when its fare is filed, its advance purchase days
	// are met through the frame, and it is open.
	lowest := func(i int, c *cabinRM, comp string, frame, party int) string {
		for k := len(c.classes) - 1; k >= 0; k-- {
			if c.ap[k] >= 0 && c.ap[k] <= rm.FrameFrom(frame) && isOpen(i, comp, c.classes[k], frame, party) {
				return c.classes[k]
			}
		}
		return ""
	}
	// choose is the class a traveller buys for the legs: the fare that
	// costs them least in all among those they can be sold and will pay.
	// looked says, leg by leg, that the carrier's record already has this
	// traveller's look at the flight.
	choose := func(idx []int, t market.Traveller, looked []bool) string {
		shown := rungs(flights[idx[0]].From, flights[idx[0]].To)
		if len(idx) == 2 {
			shown = across(flights[idx[0]], flights[idx[1]])
		}
		// Where an aircraft has no front cabin the traveller sits behind.
		want := t
		for _, i := range idx {
			cab := cabinsOf(i)
			if _, ok := cab[t.Cabin]; t.Cabin != "" && !ok {
				want.Cabin = ""
			}
		}
		frame := rm.FrameOf(t.Lead)
		var state [2]string
		if opts.RM != nil {
			for li, i := range idx {
				comp := inventory.CompartmentFor(cabinClass(want.Cabin), cabinsOf(i))
				c := rmOf(i, comp)
				state[li] = lowest(i, c, comp, frame, t.Party)
				if !looked[li] {
					c.log.Expose(frame, state[li], 1)
					looked[li] = true
				}
			}
		}
		var offers []market.Offer
		var classes []string
		var amounts []int64
		for _, r := range shown {
			if !want.Sells(r.class) || r.ap > t.Lead || !has(r.channels, t.Channel) || (len(r.accts) > 0 && (t.Account == "" || !has(r.accts, t.Account))) {
				continue
			}
			open := true
			for _, i := range idx {
				if !isOpen(i, inventory.CompartmentFor(r.class, cabinsOf(i)), r.class, frame, t.Party) {
					open = false
					break
				}
			}
			if !open {
				continue
			}
			offers = append(offers, market.Offer{Price: r.amount, Refundable: r.refundable, ChangeFee: r.fee, Channel: t.Channel, Depart: t.Preferred})
			classes = append(classes, r.class)
			amounts = append(amounts, r.amount)
		}
		pick := t.Choose(offers)
		if pick < 0 {
			return ""
		}
		plan.Sold += t.Party * len(idx)
		plan.Fares += amounts[pick] * int64(t.Party)
		plan.Classes[classes[pick]] += t.Party * len(idx)
		km := 0
		for _, i := range idx {
			km += max(flights[i].KM, 1)
		}
		for li, i := range idx {
			comp := inventory.CompartmentFor(classes[pick], cabinsOf(i))
			sold[i][comp] += t.Party
			if c := rmOf(i, comp); c != nil {
				// A fare over two legs is the record's by the distance of
				// each.
				paid := float64(amounts[pick]) * float64(t.Party) * float64(max(flights[i].KM, 1)) / float64(km)
				c.sell(frame, state[li], classes[pick], t.Party, paid)
			}
		}
		return classes[pick]
	}
	// travellers draws the pool for a flight with room for need more
	// seats, earliest bookers first.
	travellers := func(f world.Flight, need int) []market.Traveller {
		share := 1.0
		if opts.Share != nil {
			share = opts.Share(f)
		}
		base := int64(0)
		if opts.Base != nil {
			base = opts.Base(f.From, f.To, f.KM)
		} else {
			for _, r := range rungs(f.From, f.To) {
				if r.class == "Y" {
					base = int64(float64(r.amount) / 3.6)
				}
			}
		}
		mix := market.MixFor("", f.KM > 4000)
		if opts.Mix != nil {
			mix = opts.Mix(f.From, f.To, f.KM)
		}
		seats := int(float64(need)*pool*share + 0.5)
		var out []market.Traveller
		for n := 0; n < seats; {
			t := market.Draw(rng, base, mix)
			if opts.Offered != nil && !opts.Offered(code, t.Channel) {
				// Not sold where this traveller shops: two in three find
				// the carrier's own site.
				if rng.Intn(3) == 0 {
					n += t.Party
					continue
				}
				t.Channel = market.Direct
			}
			n += t.Party
			out = append(out, t)
		}
		sort.SliceStable(out, func(a, b int) bool { return out[a].Lead > out[b].Lead })
		return out
	}

	batch := make([]*pnr.PNR, 0, opts.Batch)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := sink(ctx, code, batch); err != nil {
			return err
		}
		batch = make([]*pnr.PNR, 0, opts.Batch)
		return nil
	}
	// emit writes one party's record and the marketing carrier's copy of
	// it.
	emit := func(legs []world.Flight, size int, class string, t *market.Traveller) error {
		rec := record(rng, code, acct, legs, size, class, t, opts, own, channels, &ticketSeq)
		plan.Records++
		plan.Passengers += len(rec.Passengers)
		// A codeshare leg sold under the marketing carrier's code: the
		// marketing carrier holds the booking the passenger made, and
		// the operator holds the record its interline sell created,
		// each pointing at the other.
		if m := legs[0]; len(legs) == 1 && m.Marketing != "" && m.Marketing != code && m.MarketingNumber != "" && rng.Float64() < opts.Marketed {
			mrec := marketedCopy(rec, m, ownerFor(m.Marketing), opts.Accounting(m.Marketing), &ticketSeq)
			rec.Locators = []pnr.ExternalLocator{{Owner: m.Marketing, Value: mrec.RecordLocator}}
			rec.Origin = pnr.Origin{Party: m.Marketing, Agent: "interline", Channel: "codeshare"}
			// The operator's copy came over the interline link, which
			// carries no fare: the marketing carrier holds the price.
			rec.Pricing = nil
			for i := range rec.Tickets {
				for j := range rec.Tickets[i].Coupons {
					rec.Tickets[i].Coupons[j].Amount, rec.Tickets[i].Coupons[j].Currency = "", ""
				}
			}
			marketedOut[m.Marketing] = append(marketedOut[m.Marketing], mrec)
			plan.Marketed++
			if len(marketedOut[m.Marketing]) >= opts.Batch {
				if err := sink(ctx, m.Marketing, marketedOut[m.Marketing]); err != nil {
					return err
				}
				marketedOut[m.Marketing] = nil
			}
		}
		batch = append(batch, rec)
		if len(batch) >= opts.Batch {
			return flush()
		}
		return nil
	}

	for i, f := range flights {
		if opts.Tariff != nil {
			// The market: the pool buys in booking order until the flight
			// has its load or the pool is spent.
			for _, t := range travellers(f, target[i]-assigned[i]) {
				if assigned[i]+t.Party > f.Seats {
					if opts.RM != nil {
						// The aircraft is full: the record has a traveller
						// who found nothing on sale.
						cabin := t.Cabin
						if _, ok := cabinsOf(i)[cabin]; !ok {
							cabin = ""
						}
						comp := inventory.CompartmentFor(cabinClass(cabin), cabinsOf(i))
						rmOf(i, comp).log.Expose(rm.FrameOf(t.Lead), "", 1)
					}
					continue
				}
				t := t
				legs, idx := []world.Flight{f}, []int{i}
				class := ""
				var looked [2]bool
				if rng.Float64() < opts.Connecting {
					if j := onward(i, t.Party); j >= 0 {
						if class = choose([]int{i, j}, t, looked[:]); class != "" {
							legs, idx = append(legs, flights[j]), append(idx, j)
						}
					}
				}
				if class == "" {
					class = choose(idx, t, looked[:1])
				}
				if class == "" {
					continue // every fare left is more than they will pay
				}
				for _, k := range idx {
					assigned[k] += t.Party
				}
				if len(idx) > 1 {
					plan.Connecting++
				}
				if opts.Dry {
					plan.Records++
					plan.Passengers += t.Party
					continue
				}
				if err := emit(legs, t.Party, class, &t); err != nil {
					return plan, err
				}
			}
			continue
		}
		for assigned[i] < target[i] {
			size := partySize(rng)
			if left := target[i] - assigned[i]; size > left {
				size = left
			}
			legs := []world.Flight{f}
			idx := []int{i}
			j := -1
			if rng.Float64() < opts.Connecting {
				if j = onward(i, size); j >= 0 {
					idx = append(idx, j)
				}
			}
			class := place(idx, size)
			if class == "" && j >= 0 {
				// The connection's cabins will not take the party; travel
				// the one leg.
				idx, j = idx[:1], -1
				class = place(idx, size)
			}
			if class == "" {
				// Cabin by cabin the aircraft is full: the target over the
				// whole seat count overstated what its cabins can take.
				assigned[i] = target[i]
				break
			}
			assigned[i] += size
			if j >= 0 {
				assigned[j] += size
				legs = append(legs, flights[j])
				plan.Connecting++
			}
			if err := emit(legs, size, class, nil); err != nil {
				return plan, err
			}
		}
	}
	if err := flush(); err != nil {
		return plan, err
	}
	// The booking windows close: each flight's goes into its record.
	for _, cabins := range revman {
		for _, c := range cabins {
			if !opts.Remembered {
				c.cab.Record(c.log)
			}
		}
	}
	codes := make([]string, 0, len(marketedOut))
	for cr := range marketedOut {
		codes = append(codes, cr)
	}
	sort.Strings(codes)
	for _, cr := range codes {
		if len(marketedOut[cr]) == 0 {
			continue
		}
		if err := sink(ctx, cr, marketedOut[cr]); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// cabinRM is one flight's cabin as its carrier's revenue management has
// it while the booking window is sold: the record being written, and
// the controls the flight's earlier departures give.
type cabinRM struct {
	cab     *rm.Cabin
	log     *inventory.Log
	seats   int
	classes []string // highest fare first
	at      map[string]int
	fare    []float64 // the leg's own fare by class
	top     float64   // the fare of the cabin's highest class
	ap      []int     // advance purchase days by class; -1 when not filed
	learned bool
	ctl     [rm.Frames][]inventory.Control
	book    [rm.Frames][]int // seats sold by time frame and class
	total   [rm.Frames]int   // seats sold by time frame
}

// controls is how the cabin's classes are sold in a time frame, worked
// out when the frame is first reached.
func (c *cabinRM) controls(frame int) []inventory.Control {
	if c.ctl[frame] == nil {
		c.ctl[frame] = c.cab.Controls(frame, 1, func(class string) float64 { return c.fare[c.at[class]] / c.top })
	}
	return c.ctl[frame]
}

// sell writes a sale to the record and to the frame's counts.
func (c *cabinRM) sell(frame int, lowest, class string, seats int, paid float64) {
	k, ok := c.at[class]
	if !ok {
		return
	}
	c.book[frame][k] += seats
	c.total[frame] += seats
	c.log.Sale(frame, lowest, class, seats, paid/c.top)
}

// cabinClass is the compartment of a traveller's cabin, as a class that
// sits in it.
func cabinClass(cabin string) string {
	switch cabin {
	case "F":
		return "F"
	case "C":
		return "C"
	}
	return "Y"
}

// marketedCopy is the marketing carrier's record of a sale on a leg it
// does not fly: the same party, the segment under the marketing code with
// the operator named, its own locator and tickets, and the channel's
// locator where the operator's copy carries the marketing carrier's.
func marketedCopy(op *pnr.PNR, leg world.Flight, owner *allocator, acct string, ticketSeq *uint64) *pnr.PNR {
	m := *op
	m.ID = ""
	m.RecordLocator = owner.take()
	m.Passengers = append([]pnr.Passenger(nil), op.Passengers...)
	m.Segments = make([]pnr.Segment, len(op.Segments))
	for i, sg := range op.Segments {
		sg.Carrier, sg.FlightNum, sg.OperatingCarrier = leg.Marketing, leg.MarketingNumber, leg.Carrier
		m.Segments[i] = sg
	}
	m.SSRs = append([]pnr.SSR(nil), op.SSRs...)
	for i := range m.SSRs {
		m.SSRs[i].Carrier = leg.Marketing
	}
	m.Remarks = []pnr.Remark{{Text: fmt.Sprintf("OPERATED BY %s AS %s%s", leg.Carrier, leg.Carrier, strings.TrimLeft(leg.Number, "0"))}}
	if op.Pricing != nil {
		pc := *op.Pricing
		pc.Passengers = append([]pnr.PassengerPricing(nil), op.Pricing.Passengers...)
		m.Pricing = &pc
	}
	m.Tickets = make([]pnr.Ticket, len(op.Tickets))
	for i, t := range op.Tickets {
		*ticketSeq++
		t.Number = pnr.TicketNumber{AirlineCode: acct, Serial: fmt.Sprintf("%010d", *ticketSeq)}
		t.Coupons = append([]pnr.Coupon(nil), t.Coupons...)
		m.Tickets[i] = t
	}
	// The channel's locator stays on the record the passenger booked; the
	// operator's copy carries this record's locator instead.
	m.Locators = append([]pnr.ExternalLocator(nil), op.Locators...)
	return &m
}

// loadFactor is one flight's share of seats sold before the day: the
// target, moved by the hour (the banks people want run fuller than the
// late evening) and by the flight's own luck.
func loadFactor(base float64, f world.Flight, rng *rand.Rand) float64 {
	lf := base + (rng.Float64()*0.22 - 0.12)
	hour := (f.DepMin / 60) % 24
	switch {
	case hour >= 6 && hour <= 9, hour >= 15 && hour <= 19:
		lf += 0.05
	case hour >= 21 || hour < 5:
		lf -= 0.10
	}
	return math.Max(0.30, math.Min(0.98, lf))
}

// partySize is how many seats one booking takes: holiday travel skews to
// pairs and families.
func partySize(rng *rand.Rand) int {
	r := rng.Float64()
	switch {
	case r < 0.42:
		return 1
	case r < 0.74:
		return 2
	case r < 0.87:
		return 3
	case r < 0.96:
		return 4
	case r < 0.99:
		return 5
	default:
		return 6
	}
}

// record is one booking: a party on one or two legs, with everything a
// record at the carrier carries.
func record(rng *rand.Rand, carrier, acct string, legs []world.Flight, seats int, class string, who *market.Traveller, opts Options, own *allocator, channels map[string]*allocator, ticketSeq *uint64) *pnr.PNR {
	day := opts.Day
	surname := surnames[rng.Intn(len(surnames))]
	// Booked between a week and five months out, most of it six to ten
	// weeks before: a holiday is planned. A traveller from the market
	// booked when they did.
	booked := day.AddDate(0, 0, -(7 + int(math.Abs(rng.NormFloat64())*35) + rng.Intn(21)))
	if who != nil {
		booked = day.AddDate(0, 0, -who.Lead)
	}
	if who != nil {
		// At some minute of that day no later than the hour the flight
		// leaves, so the days in hand are whole.
		booked = booked.Add(time.Duration(rng.Intn(max(legs[0].DepMin, 1))) * time.Minute)
	} else {
		booked = booked.Add(time.Duration(rng.Intn(14*60)+7*60) * time.Minute)
	}
	rec := &pnr.PNR{
		RecordLocator: own.take(),
		Status:        pnr.StatusTicketed,
		CreatedAt:     booked,
		UpdatedAt:     booked.Add(time.Duration(rng.Intn(48)) * time.Hour),
		ReceivedFrom:  "P",
		Contacts:      []pnr.Contact{{Type: "phone", Text: fmt.Sprintf("%s %03d-%03d-%04d-M", homeCity(legs[0]), 200+rng.Intn(800), rng.Intn(1000), rng.Intn(10000))}},
	}

	// The party: adults, a share of children in parties of two or more, and
	// now and then an infant on an adult's lap, who takes no seat. One party
	// in a few thousand has a cello: it takes the last seat of the party,
	// under the passenger's surname, and is as real a seat as any.
	cello := seats >= 2 && rng.Float64() < opts.Cellos
	people := seats
	if cello {
		people--
	}
	kids := 0
	if people >= 2 && rng.Float64() < 0.28 {
		kids = 1 + rng.Intn(min(people-1, 3))
	}
	for i := 0; i < people; i++ {
		p := pnr.Passenger{Ref: i + 1, Surname: surname, Type: pnr.PaxAdult}
		if i < kids {
			p.Type = pnr.PaxChild
			p.Given = givenChild[rng.Intn(len(givenChild))]
			if rng.Intn(2) == 0 {
				p.Title = "MSTR"
			} else {
				p.Title = "MISS"
			}
		} else if rng.Intn(2) == 0 {
			p.Given, p.Title = givenM[rng.Intn(len(givenM))], "MR"
		} else {
			p.Given = givenF[rng.Intn(len(givenF))]
			p.Title = []string{"MRS", "MS", "MS"}[rng.Intn(3)]
		}
		if p.Type == pnr.PaxAdult && rng.Float64() < 0.35 {
			p.FrequentFlyer = []string{fmt.Sprintf("%s%09d", carrier, rng.Intn(1_000_000_000))}
		}
		rec.Passengers = append(rec.Passengers, p)
	}
	if cello {
		rec.Passengers = append(rec.Passengers, pnr.Passenger{Ref: len(rec.Passengers) + 1, Surname: surname, Given: "CBBG", Type: pnr.PaxAdult})
	}
	if kids < people && rng.Float64() < 0.03 {
		rec.Passengers = append(rec.Passengers, pnr.Passenger{Ref: len(rec.Passengers) + 1, Surname: surname,
			Given: givenChild[rng.Intn(len(givenChild))], Title: "INF", Type: pnr.PaxInfant, Infant: true})
	}

	// The itinerary, in one booking class throughout.
	for k, f := range legs {
		dep := day.Add(time.Duration(f.DepMin) * time.Minute)
		arr := day.Add(time.Duration(f.ArrMin) * time.Minute)
		sg := pnr.Segment{
			Ref: k + 1, Type: pnr.SegmentAir, Carrier: f.Carrier, FlightNum: f.Number, Class: class,
			Depart: day, WireDate: strings.ToUpper(day.Format("02Jan")),
			DepartTime: dep.Format("1504"), ArriveTime: arr.Format("1504"),
			Board: f.From, Off: f.To, Status: "HK", Seats: seats,
		}
		rec.Segments = append(rec.Segments, sg)
		if f.Marketing != "" && rng.Float64() < 0.7 {
			rec.Remarks = append(rec.Remarks, pnr.Remark{Text: fmt.Sprintf("CODESHARE SOLD AS %s%s OPERATED BY %s%s",
				f.Marketing, strings.TrimLeft(f.MarketingNumber, "0"), f.Carrier, strings.TrimLeft(f.Number, "0"))})
		}
	}

	// Special service requests, at the rates a holiday day runs them.
	for _, p := range rec.Passengers {
		switch {
		case p.Given == "CBBG":
			rec.SSRs = append(rec.SSRs, ssr("CBBG", carrier, p.Ref, len(legs), "CELLO"))
		case p.Type == pnr.PaxInfant:
			rec.SSRs = append(rec.SSRs, ssr("INFT", carrier, p.Ref, len(legs), fmt.Sprintf("%s/%s", p.Surname, p.Given)))
		case p.Type == pnr.PaxChild:
			rec.SSRs = append(rec.SSRs, ssr("CHLD", carrier, p.Ref, len(legs), ""))
			if rng.Float64() < 0.04 {
				rec.SSRs = append(rec.SSRs, ssr("UMNR", carrier, p.Ref, len(legs), "UM12"))
			}
		default:
			r := rng.Float64()
			switch {
			case r < 0.025:
				rec.SSRs = append(rec.SSRs, ssr([]string{"WCHR", "WCHR", "WCHS", "WCHC"}[rng.Intn(4)], carrier, p.Ref, len(legs), ""))
			case r < 0.035:
				rec.SSRs = append(rec.SSRs, ssr([]string{"VGML", "KSML", "GFML", "DBML"}[rng.Intn(4)], carrier, p.Ref, len(legs), ""))
			case r < 0.041:
				rec.SSRs = append(rec.SSRs, ssr("PETC", carrier, p.Ref, len(legs), "1 DOG 6KG"))
			case r < 0.046:
				rec.SSRs = append(rec.SSRs, ssr("BLND", carrier, p.Ref, len(legs), ""))
			}
		}
	}

	// Tickets: one per name, a coupon per leg, issued when the record was
	// made or within the day.
	issued := booked.Add(time.Duration(rng.Intn(24)) * time.Hour)
	for _, p := range rec.Passengers {
		*ticketSeq++
		t := pnr.Ticket{Number: pnr.TicketNumber{AirlineCode: acct, Serial: fmt.Sprintf("%010d", *ticketSeq)},
			PaxRef: p.Ref, IssuedAt: issued, IssuedBy: "FILL"}
		for k := range legs {
			t.Coupons = append(t.Coupons, pnr.Coupon{Number: k + 1, SegmentRef: k + 1, Status: pnr.CouponOpen})
		}
		rec.Tickets = append(rec.Tickets, t)
	}

	// Who sold it: a distribution system, with its own locator on the
	// record, or the carrier direct. A traveller from the market bought
	// where they shop: on the carrier's own site, or from an online
	// agency, a travel management company or an agent, each of which
	// books through a distribution system.
	direct := rng.Float64() < opts.Direct
	kind := "gds"
	if who != nil {
		direct, kind = who.Channel == market.Direct, who.Channel
	}
	if direct {
		rec.Origin = pnr.Origin{Party: carrier, Agent: "web", Channel: "direct"}
		for i := range rec.Tickets {
			rec.Tickets[i].IssuedBy = carrier
		}
	} else {
		ch := opts.Channels[rng.Intn(len(opts.Channels))]
		agent, connected := fmt.Sprintf("%s%04d", ch, rng.Intn(10000)), false
		if who != nil && opts.Seller != nil {
			if a, system, direct := opts.Seller(rng, kind, carrier, legs[0]); a != "" {
				agent, connected = a, direct
				if channels[system] != nil {
					ch = system
				}
			}
		}
		if connected {
			// The agency booked on the carrier's own system: the record
			// is the carrier's alone, and names the agency.
			rec.Origin = pnr.Origin{Party: carrier, Agent: agent, Channel: "ndc"}
			ch = carrier
		} else {
			rec.Locators = []pnr.ExternalLocator{{Owner: ch, Value: channels[ch].take()}}
			rec.Origin = pnr.Origin{Party: ch, Agent: agent, Channel: kind}
		}
		for i := range rec.Tickets {
			rec.Tickets[i].IssuedBy = ch
		}
	}
	if who != nil {
		rec.Origin.Account = who.Account
	}
	if opts.Tariff != nil && !price(rec, opts.Tariff, booked, legs) && who != nil {
		// The traveller chose a fare the pricing would not sell them: the
		// two disagree about a rule, and the record says so.
		rec.Remarks = append(rec.Remarks, pnr.Remark{Text: "FARE AS CHOSEN NOT SOLD - REPRICED Y FROM " + class})
	}
	return rec
}

func ssr(code, carrier string, pax, legs int, text string) pnr.SSR {
	return pnr.SSR{Code: code, Carrier: carrier, Status: "HK", Count: 1, PaxRef: pax, Text: text}
}

// bookingClass draws the class a party sits in: mostly economy fare
// buckets, a business share, a few first.
func bookingClass(rng *rand.Rand) string {
	r := rng.Float64()
	switch {
	case r < 0.02:
		return "F"
	case r < 0.11:
		return []string{"J", "C", "D"}[rng.Intn(3)]
	case r < 0.30:
		return "Y"
	case r < 0.55:
		return []string{"B", "M", "H"}[rng.Intn(3)]
	default:
		return []string{"Q", "K", "L", "V", "S", "N"}[rng.Intn(6)]
	}
}

// homeCity is where the phone number is from: the boarding point, mostly.
func homeCity(f world.Flight) string { return f.From }

func legKM(legs []world.Flight, k int) int {
	if k < len(legs) {
		return legs[k].KM
	}
	return 0
}

// price prices a record against the tariff as of its booking date. The
// class was drawn at random; if the rules will not sell it this far out the
// party is moved to full fare, which is what the passenger would have been
// offered.
func price(rec *pnr.PNR, t fare.Tariff, booked time.Time, legs []world.Flight) (asChosen bool) {
	asChosen = true
	req := fare.Request{Purchased: booked, Sale: fare.Sale{Channel: rec.Origin.Channel, Account: rec.Origin.Account}}
	for k, s := range rec.Segments {
		// The times are the schedule's, counted from the start of the day:
		// a leg that leaves after midnight leaves after the one before it.
		dep, arr := s.Depart, time.Time{}
		if k < len(legs) {
			dep = s.Depart.Add(time.Duration(legs[k].DepMin) * time.Minute)
			arr = s.Depart.Add(time.Duration(legs[k].ArrMin) * time.Minute)
		}
		req.Segments = append(req.Segments, fare.Segment{Carrier: s.Carrier, FlightNum: s.FlightNum, Origin: s.Board, Destination: s.Off, Class: s.Class,
			Depart: dep, Arrive: arr, Miles: int(float64(legKM(legs, k)) / 1.609)})
	}
	for _, p := range rec.Passengers {
		switch {
		case p.Infant:
			req.Passengers = append(req.Passengers, fare.Infant)
		case p.Type == pnr.PaxChild:
			req.Passengers = append(req.Passengers, fare.Child)
		default:
			req.Passengers = append(req.Passengers, fare.Adult)
		}
	}
	q, err := fare.Price(t, req)
	if err != nil {
		for i := range req.Segments {
			req.Segments[i].Class = "Y"
		}
		q, err = fare.Price(t, req)
		if err != nil {
			return false
		}
		for i := range rec.Segments {
			rec.Segments[i].Class = "Y"
		}
		asChosen = false
	}
	pr := &pnr.Pricing{Currency: q.Currency, Base: q.Base.Amount, Taxes: q.Taxes.Amount, Total: q.Total.Amount, PricedAt: booked,
		CarrierFees: q.CarrierFees.Amount, Calc: q.Calc}
	for i, pq := range q.Passengers {
		pp := pnr.PassengerPricing{Ref: i + 1, Type: string(pq.Type), Base: pq.Base.Amount, Total: pq.Total.Amount}
		for _, tl := range pq.Taxes {
			pp.Taxes += tl.Amount.Amount
		}
		for _, sf := range pq.Segments {
			pp.Bases = append(pp.Bases, sf.Basis)
			pp.Segments = append(pp.Segments, sf.Base.Amount)
		}
		pr.Passengers = append(pr.Passengers, pp)
	}
	for i := range rec.Segments {
		if i < len(q.Passengers[0].Segments) {
			rec.Segments[i].FareBasis = q.Passengers[0].Segments[i].Basis
		}
	}
	rec.Pricing = pr
	// The coupons carry the value: the segment's base plus the passenger's
	// taxes spread across their coupons.
	for ti := range rec.Tickets {
		tk := &rec.Tickets[ti]
		var pp *pnr.PassengerPricing
		for k := range pr.Passengers {
			if pr.Passengers[k].Ref == tk.PaxRef {
				pp = &pr.Passengers[k]
			}
		}
		if pp == nil {
			continue
		}
		for ci := range tk.Coupons {
			idx := tk.Coupons[ci].SegmentRef - 1
			if idx < 0 || idx >= len(pp.Segments) {
				continue
			}
			v := pp.Segments[idx] + pp.Taxes/int64(len(pp.Segments))
			tk.Coupons[ci].Amount = fmt.Sprintf("%d.%02d", v/100, v%100)
			tk.Coupons[ci].Currency = pr.Currency
		}
	}
	return asChosen
}
