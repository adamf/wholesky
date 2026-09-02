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
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/pnr"

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
}

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
	for _, code := range codes {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		p, err := fillCarrier(ctx, code, byCarrier[code], opts, channels, sink)
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
	var plan Plan
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
	// party of n can still be put on: forty-five minutes to five hours after
	// arrival, the way a connection is built.
	onward := func(i, n int) int {
		f := flights[i]
		cands := byFrom[f.To]
		lo, hi := f.ArrMin+45, f.ArrMin+300
		start := sort.Search(len(cands), func(k int) bool { return flights[cands[k]].DepMin >= lo })
		var fit []int
		for k := start; k < len(cands) && flights[cands[k]].DepMin <= hi; k++ {
			j := cands[k]
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

	for i, f := range flights {
		for assigned[i] < target[i] {
			size := partySize(rng)
			if left := target[i] - assigned[i]; size > left {
				size = left
			}
			legs := []world.Flight{f}
			assigned[i] += size
			if rng.Float64() < opts.Connecting {
				if j := onward(i, size); j >= 0 {
					assigned[j] += size
					legs = append(legs, flights[j])
					plan.Connecting++
				}
			}
			rec := record(rng, code, acct, legs, size, opts, own, channels, &ticketSeq)
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
				marketedOut[m.Marketing] = append(marketedOut[m.Marketing], mrec)
				plan.Marketed++
				if len(marketedOut[m.Marketing]) >= opts.Batch {
					if err := sink(ctx, m.Marketing, marketedOut[m.Marketing]); err != nil {
						return plan, err
					}
					marketedOut[m.Marketing] = nil
				}
			}
			batch = append(batch, rec)
			if len(batch) >= opts.Batch {
				if err := flush(); err != nil {
					return plan, err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return plan, err
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
func record(rng *rand.Rand, carrier, acct string, legs []world.Flight, seats int, opts Options, own *allocator, channels map[string]*allocator, ticketSeq *uint64) *pnr.PNR {
	day := opts.Day
	surname := surnames[rng.Intn(len(surnames))]
	// Booked between a week and five months out, most of it six to ten
	// weeks before: a holiday is planned.
	booked := day.AddDate(0, 0, -(7 + int(math.Abs(rng.NormFloat64())*35) + rng.Intn(21)))
	booked = booked.Add(time.Duration(rng.Intn(14*60)+7*60) * time.Minute)
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
	class := bookingClass(rng)
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
	// record, or the carrier direct.
	if rng.Float64() < opts.Direct {
		rec.Origin = pnr.Origin{Party: carrier, Agent: "web", Channel: "direct"}
		for i := range rec.Tickets {
			rec.Tickets[i].IssuedBy = carrier
		}
	} else {
		ch := opts.Channels[rng.Intn(len(opts.Channels))]
		rec.Locators = []pnr.ExternalLocator{{Owner: ch, Value: channels[ch].take()}}
		rec.Origin = pnr.Origin{Party: ch, Agent: fmt.Sprintf("%s%04d", ch, rng.Intn(10000)), Channel: "gds"}
		for i := range rec.Tickets {
			rec.Tickets[i].IssuedBy = ch
		}
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
