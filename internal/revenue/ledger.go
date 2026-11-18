// Package revenue keeps what the day's tickets were sold for, by the leg
// they fly: the money in the air is the sum over the legs airborne now,
// the money taken is the sum over everything sold. The ledger is fed where
// the price is known -- the distribution system that priced the sale, or
// the filler that wrote the pre-sold day -- and read by the globe.
package revenue

import (
	"strings"
	"sync"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
)

// Ledger is a per-leg total in minor units, and an account for each
// carrier of what its sales earned and cost.
//
// A carrier's revenue is the fare and its own surcharges. The taxes a
// ticket carries are the governments' and the airports', collected and
// passed on, and are counted apart. A coupon one carrier flies on
// another's ticket is worth its share of the fare less the charge the
// two have agreed, which the ticketing carrier keeps. A sale costs what
// its channel costs. A booking cancelled gives everything back.
type Ledger struct {
	// Resolve, when set, maps a segment sold under a marketing code to the
	// leg that flies it: a distribution system's record says DL5203, the
	// aircraft in the air is 9E5203.
	Resolve func(carrier, number, board string) (operating string, ok bool)
	// Wire, when set, is the day the world flies as the wire writes it
	// (26NOV): a segment on another date is a return not flown today, and
	// its money is not today's revenue.
	Wire string
	// Charge, when set, is what the carrier that ticketed a coupon keeps
	// of its value when another carrier flew it, in hundredths of a
	// percent: the interline service charge, or a codeshare commission.
	Charge func(ticketed, flown string) int
	// Sale, when set, is what a ticket's sale cost the carrier that
	// ticketed it: the booking fees, commission, card cost and its own
	// cost of selling, in minor units.
	Sale func(r *pnr.PNR, ticketing string, segments int, fare, paid int64) Cost

	mu       sync.Mutex
	legs     map[string]int64
	seats    map[string]int
	total    int64
	accounts map[string]*Account
	agents   map[string]*Agent
	entries  map[string]*entry // by record locator, for a cancellation
	epoch    int64             // the times the ledger was emptied
}

// Agent is one agency's sales: what it sold and what it earned from the
// distribution system, the carriers and the travellers.
type Agent struct {
	Tickets int `json:"tickets"`
	// Connected is the tickets it sold over a direct connection.
	Connected int `json:"connected,omitempty"`
	// Sales is the fares it sold, Incentive what its distribution system
	// paid it, Commission what the carriers paid it, and Fees what it
	// charged travellers.
	Sales      int64 `json:"sales"`
	Incentive  int64 `json:"incentive"`
	Commission int64 `json:"commission"`
	Fees       int64 `json:"fees"`
	// Carriers is its tickets by the carrier that ticketed them.
	Carriers map[string]int `json:"carriers,omitempty"`
}

// Income is everything the agency earned.
func (a Agent) Income() int64 { return a.Incentive + a.Commission + a.Fees }

func (a *Agent) add(d Agent, sign int64) {
	a.Tickets += int(sign) * d.Tickets
	a.Connected += int(sign) * d.Connected
	a.Sales += sign * d.Sales
	a.Incentive += sign * d.Incentive
	a.Commission += sign * d.Commission
	a.Fees += sign * d.Fees
	for c, n := range d.Carriers {
		if a.Carriers == nil {
			a.Carriers = map[string]int{}
		}
		a.Carriers[c] += int(sign) * n
	}
}

// Cost is what a sale cost the carrier, by line, and who sold it.
type Cost struct {
	System, Commission, Payment, Own int64
	// Agent is the agency that sold the ticket, by its number, or "" for
	// the carrier's own sale. Incentive is what the distribution system
	// passes on to the agency out of System, and Fee what the agency
	// charged the traveller: both are the agency's income and neither is
	// the carrier's cost. Connected says the sale came over a direct
	// connection.
	Agent     string
	Incentive int64
	Fee       int64
	Connected bool
}

// Total is the whole cost of a sale.
func (c Cost) Total() int64 { return c.System + c.Commission + c.Payment + c.Own }

// Account is one carrier's sales.
type Account struct {
	// Tickets and Passengers count what the carrier ticketed.
	Tickets    int `json:"tickets"`
	Passengers int `json:"passengers"`
	// Flown is the value of the coupons the carrier flies, net of the
	// charges it paid the carriers that ticketed them. Fees is its own
	// surcharges on its own tickets.
	Flown int64 `json:"flown"`
	Fees  int64 `json:"fees"`
	// Kept is what the carrier kept on coupons other carriers flew on its
	// tickets: the service charge or commission, and its own surcharge on
	// them. Paid is what it paid others on coupons it flew for them.
	Kept int64 `json:"kept"`
	Paid int64 `json:"paid"`
	// Taxes is what its tickets collected for governments and airports.
	Taxes int64 `json:"taxes"`
	// System, Commission, Payment and Own are the cost of its sales.
	System     int64 `json:"system"`
	Commission int64 `json:"commission"`
	Payment    int64 `json:"payment"`
	Own        int64 `json:"own"`
	// Channels is its tickets by the channel that sold them.
	Channels map[string]int `json:"channels,omitempty"`
}

// Distribution is the whole cost of the carrier's sales.
func (a Account) Distribution() int64 { return a.System + a.Commission + a.Payment + a.Own }

func (a *Account) add(d Account, sign int64) {
	a.Tickets += int(sign) * d.Tickets
	a.Passengers += int(sign) * d.Passengers
	a.Flown += sign * d.Flown
	a.Fees += sign * d.Fees
	a.Kept += sign * d.Kept
	a.Paid += sign * d.Paid
	a.Taxes += sign * d.Taxes
	a.System += sign * d.System
	a.Commission += sign * d.Commission
	a.Payment += sign * d.Payment
	a.Own += sign * d.Own
	for ch, n := range d.Channels {
		if a.Channels == nil {
			a.Channels = map[string]int{}
		}
		a.Channels[ch] += int(sign) * n
	}
}

// entry is what one record put in the ledger, so that it can be taken
// out again.
type entry struct {
	legs     map[string]int64
	seats    map[string]int
	accounts map[string]Account
	agent    string
	sold     Agent
}

// Reset forgets everything, before a rebuild from the book.
func (l *Ledger) Reset() {
	l.mu.Lock()
	l.legs, l.seats, l.total = map[string]int64{}, map[string]int{}, 0
	l.accounts, l.entries, l.agents = map[string]*Account{}, map[string]*entry{}, map[string]*Agent{}
	l.epoch++
	l.mu.Unlock()
}

// Fill credits legs from a store's aggregate: each leg's share of the
// priced records that hold it.
func (l *Ledger) Fill(legs []store.LegRevenue) {
	for _, r := range legs {
		l.Add(l.key(r.Carrier, r.FlightNum, r.Board), r.Cents)
	}
}

func (l *Ledger) key(carrier, number, board string) string {
	if l.Resolve != nil {
		if op, ok := l.Resolve(carrier, number, board); ok && op != "" {
			carrier = op
		}
	}
	return Key(carrier, number, board)
}

// New returns an empty ledger.
func New() *Ledger {
	return &Ledger{legs: map[string]int64{}, seats: map[string]int{}, accounts: map[string]*Account{}, entries: map[string]*entry{}, agents: map[string]*Agent{}}
}

// Key names a leg: the operating carrier, the flight number without
// leading zeros, and the boarding point, which is how the globe names an
// aircraft.
func Key(carrier, number, board string) string {
	return strings.ToUpper(carrier) + strings.TrimLeft(number, "0") + "/" + strings.ToUpper(board)
}

// Add credits a leg.
func (l *Ledger) Add(key string, cents int64) {
	if cents == 0 {
		return
	}
	l.mu.Lock()
	l.legs[key] += cents
	l.total += cents
	l.mu.Unlock()
}

// Record credits a priced record: each air segment flown today with its
// share of the fare and of the carrier's own fees, to the leg that flies
// it; the carriers' accounts with what they flew, kept and paid; and the
// ticketing carrier's account with what the sale cost. A record already
// in the ledger is not counted twice. A record with no price adds
// nothing.
func (l *Ledger) Record(r *pnr.PNR) { l.record(r, true) }

// Load is Record for a book loaded in bulk: the day's advance sales,
// which are counted and not kept, because nothing will take them out
// one by one.
func (l *Ledger) Load(r *pnr.PNR) { l.record(r, false) }

func (l *Ledger) record(r *pnr.PNR, keep bool) {
	if r == nil || r.Pricing == nil {
		return
	}
	var air []pnr.Segment
	for _, s := range r.Segments {
		if s.Type == pnr.SegmentAir {
			air = append(air, s)
		}
	}
	if len(air) == 0 {
		return
	}
	ticketing := strings.ToUpper(air[0].Carrier)
	e := &entry{legs: map[string]int64{}, seats: map[string]int{}, accounts: map[string]Account{}}
	acct := func(c string) Account { return e.accounts[c] }
	pax := max(len(r.Pricing.Passengers), 1)
	feesPerSeg := r.Pricing.CarrierFees / int64(len(air))
	var fareAll int64
	for idx, s := range air {
		if l.Wire != "" && s.WireDate != "" && !strings.EqualFold(s.WireDate, l.Wire) {
			continue
		}
		flown := strings.ToUpper(s.Carrier)
		if s.OperatingCarrier != "" {
			flown = strings.ToUpper(s.OperatingCarrier)
		}
		if l.Resolve != nil {
			if op, ok := l.Resolve(s.Carrier, s.FlightNum, s.Board); ok && op != "" {
				flown = strings.ToUpper(op)
			}
		}
		var value int64
		for _, pp := range r.Pricing.Passengers {
			if idx < len(pp.Segments) {
				value += pp.Segments[idx]
			}
		}
		fareAll += value
		var charge int64
		if flown != ticketing && l.Charge != nil {
			charge = value * int64(l.Charge(ticketing, flown)) / 10000
		}
		key := Key(flown, s.FlightNum, s.Board)
		// The surcharge is the ticketing carrier's; it goes with the leg
		// only when that carrier flies it.
		fees := int64(0)
		if flown == ticketing {
			fees = feesPerSeg
		}
		e.legs[key] += value - charge + fees
		e.seats[key] += max(s.Seats, len(r.Passengers))
		a := acct(flown)
		a.Flown += value - charge
		a.Paid += charge
		e.accounts[flown] = a
		t := acct(ticketing)
		t.Kept += charge
		t.Fees += feesPerSeg
		if flown != ticketing {
			t.Kept += feesPerSeg
		}
		e.accounts[ticketing] = t
	}
	if len(e.legs) == 0 {
		return
	}
	t := acct(ticketing)
	t.Tickets += pax
	t.Passengers += pax
	t.Taxes += r.Pricing.Taxes - r.Pricing.CarrierFees
	channel := r.Origin.Channel
	if channel == "" {
		channel = "agency"
	}
	t.Channels = map[string]int{channel: pax}
	if l.Sale != nil {
		c := l.Sale(r, ticketing, len(air)*pax, r.Pricing.Base+r.Pricing.CarrierFees, r.Pricing.Total)
		t.System, t.Commission, t.Payment, t.Own = c.System, c.Commission, c.Payment, c.Own
		if c.Agent != "" {
			e.agent = c.Agent
			e.sold = Agent{Tickets: pax, Sales: r.Pricing.Base + r.Pricing.CarrierFees, Incentive: c.Incentive, Commission: c.Commission,
				Fees: c.Fee * int64(pax), Carriers: map[string]int{ticketing: pax}}
			if c.Connected {
				e.sold.Connected = pax
			}
		}
	}
	e.accounts[ticketing] = t

	l.mu.Lock()
	defer l.mu.Unlock()
	if keep && r.RecordLocator != "" {
		if _, dup := l.entries[r.RecordLocator]; dup {
			return
		}
		if len(l.entries) > 400000 {
			// A day's worth is kept; an old record that cancels after that
			// keeps its place in the totals.
			l.entries = map[string]*entry{}
		}
		l.entries[r.RecordLocator] = e
	}
	l.apply(e, 1)
}

// Reverse takes a record back out: the traveller cancelled, the fare is
// refunded, the booking fee stays paid. It reports whether the record
// was in the ledger.
func (l *Ledger) Reverse(locator string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[locator]
	if !ok {
		return false
	}
	delete(l.entries, locator)
	// A distribution system charges for a cancelled segment too, and the
	// carrier's own cost of the sale was spent: those are left in, and so
	// is what the system paid the agency for the booking.
	for c, a := range e.accounts {
		a.System, a.Own = 0, 0
		e.accounts[c] = a
	}
	e.sold.Incentive = 0
	l.apply(e, -1)
	return true
}

func (l *Ledger) account(c string) *Account {
	if l.accounts == nil {
		l.accounts = map[string]*Account{}
	}
	a := l.accounts[c]
	if a == nil {
		a = &Account{}
		l.accounts[c] = a
	}
	return a
}

// apply adds an entry to the totals, or with sign -1 takes it out.
// Called with the lock held.
func (l *Ledger) apply(e *entry, sign int64) {
	for k, v := range e.legs {
		l.legs[k] += sign * v
		l.total += sign * v
	}
	if l.seats == nil {
		l.seats = map[string]int{}
	}
	for k, n := range e.seats {
		l.seats[k] += int(sign) * n
	}
	for c, a := range e.accounts {
		l.account(c).add(a, sign)
	}
	if e.agent != "" {
		if l.agents == nil {
			l.agents = map[string]*Agent{}
		}
		a := l.agents[e.agent]
		if a == nil {
			a = &Agent{}
			l.agents[e.agent] = a
		}
		a.add(e.sold, sign)
	}
}

// Epoch counts the times the ledger was emptied. A count of the books
// that began before the last of them is a count of books the ledger no
// longer speaks for.
func (l *Ledger) Epoch() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.epoch
}

// Holds reports whether the ledger holds a record by its locator: a
// sale recorded as it was made, which can be taken out again.
func (l *Ledger) Holds(locator string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.entries[locator]
	return ok
}

// Replace takes out what a store's aggregate put in and puts in what a
// record-by-record count of the same books found. The aggregate is one
// query and is there at once, with the taxes in and no accounts; the
// count takes as long as the books are large. It does nothing, and says
// so, if the ledger was emptied after epoch.
//
// A sale made while the count ran may be in both: the count read it
// from the book, and the ledger recorded it as it was made. The count
// keeps such a record by its locator. One the ledger already holds comes
// back out of the count. The rest the ledger holds from now on, so that
// the same sale recorded late is not counted again.
func (l *Ledger) Replace(coarse []store.LegRevenue, exact *Ledger, epoch int64) bool {
	exact.mu.Lock()
	legs, seats, total := exact.legs, exact.seats, exact.total
	accounts, agents, entries := exact.accounts, exact.agents, exact.entries
	exact.mu.Unlock()
	keys := make([]string, len(coarse))
	for i, r := range coarse {
		keys[i] = l.key(r.Carrier, r.FlightNum, r.Board)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.epoch != epoch {
		return false
	}
	for i, r := range coarse {
		l.legs[keys[i]] -= r.Cents
		l.total -= r.Cents
		if l.legs[keys[i]] == 0 {
			delete(l.legs, keys[i])
		}
	}
	for k, v := range legs {
		l.legs[k] += v
	}
	l.total += total
	for k, n := range seats {
		l.seats[k] += n
	}
	for c, a := range accounts {
		l.account(c).add(*a, 1)
	}
	for code, a := range agents {
		if l.agents[code] == nil {
			l.agents[code] = &Agent{}
		}
		l.agents[code].add(*a, 1)
	}
	for locator, e := range entries {
		if _, held := l.entries[locator]; held {
			l.apply(e, -1)
			continue
		}
		l.entries[locator] = e
	}
	return true
}

// Agents is every agency's sales, by its number.
func (l *Ledger) Agents() map[string]Agent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Agent, len(l.agents))
	for code, a := range l.agents {
		cp := *a
		cp.Carriers = map[string]int{}
		for k, v := range a.Carriers {
			cp.Carriers[k] = v
		}
		out[code] = cp
	}
	return out
}

// Merge adds another machine's count of an agency's sales to this one.
func (a *Agent) Merge(b Agent) { a.add(b, 1) }

// AccountOf is a carrier's account so far.
func (l *Ledger) AccountOf(carrier string) Account {
	l.mu.Lock()
	defer l.mu.Unlock()
	if a := l.accounts[strings.ToUpper(carrier)]; a != nil {
		out := *a
		out.Channels = map[string]int{}
		for k, v := range a.Channels {
			out.Channels[k] = v
		}
		return out
	}
	return Account{}
}

// Accounts is every carrier's account, for a machine that gathers them.
func (l *Ledger) Accounts() map[string]Account {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]Account, len(l.accounts))
	for c, a := range l.accounts {
		cp := *a
		cp.Channels = map[string]int{}
		for k, v := range a.Channels {
			cp.Channels[k] = v
		}
		out[c] = cp
	}
	return out
}

// Merge adds another machine's account to one of this machine's, for the
// machine that gathers every machine's.
func (a *Account) Merge(b Account) { a.add(b, 1) }

// AddSeats counts passengers sold on a leg, for the carriers whose books
// are not here: an external node's, a joined world's.
func (l *Ledger) AddSeats(key string, n int) {
	l.mu.Lock()
	if l.seats == nil {
		l.seats = map[string]int{}
	}
	l.seats[key] += n
	l.mu.Unlock()
}

// Seats is the passengers sold over the legs named.
func (l *Ledger) Seats(keys []string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, k := range keys {
		n += l.seats[k]
	}
	return n
}

// SeatsByLeg is every leg's passenger count.
func (l *Ledger) SeatsByLeg() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]int, len(l.seats))
	for k, v := range l.seats {
		out[k] = v
	}
	return out
}

// Sum is the total over the legs named.
func (l *Ledger) Sum(keys []string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var n int64
	for _, k := range keys {
		n += l.legs[k]
	}
	return n
}

// Total is everything credited.
func (l *Ledger) Total() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}
