package sim

import (
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/avail"
	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/airport"
	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/partners"
	"github.com/adamf/wholesky/internal/pricing"
	"github.com/adamf/wholesky/internal/revenue"
	"github.com/adamf/wholesky/internal/world"
)

// The commercial side of the world: who is partners with whom, how each
// carrier sells, and the travellers who shop. A traveller wants to go
// from one place to another; the world lays out the ways to do it --
// every carrier's nonstops, and the connections that make their minimum
// connecting time -- prices each at the lowest fare that can be sold in
// the traveller's channel, and the traveller takes the one that costs
// them least in all (internal/market). So a carrier that cuts a fare
// takes travellers from the carriers that did not, and one that closes
// a class sends its travellers down the road.

type commerce struct {
	partners *partners.Book
	// pair is this world's flights by market, FROM/TO.
	pair map[string][]world.Flight
	// home is the carrier with the most departures at each airport: the
	// one whose programme the people who live there belong to.
	home map[string]string

	// desk is the carriers' pricing desks, and seats the seats each
	// carrier flies in each market, by "A/B" with A before B.
	desk   *pricing.Desk
	deskMu sync.Mutex
	seats  map[string]map[string]int

	mu     sync.RWMutex
	policy map[string]distribution.Policy
	// noSurcharge are the carriers that have taken their fuel surcharge
	// off.
	noSurcharge map[string]bool
}

func (s *Sim) initCommerce(m *world.Manifest) {
	c := &commerce{pair: map[string][]world.Flight{}, home: map[string]string{}, policy: map[string]distribution.Policy{}, noSurcharge: map[string]bool{}}
	var codes []string
	for _, cr := range m.Carriers {
		codes = append(codes, cr.Designator)
	}
	var shares [][2]string
	seen := map[string]bool{}
	deps := map[string]map[string]int{}
	for _, f := range m.Flights {
		c.pair[f.From+"/"+f.To] = append(c.pair[f.From+"/"+f.To], f)
		if deps[f.From] == nil {
			deps[f.From] = map[string]int{}
		}
		deps[f.From][f.Carrier]++
		if f.Marketing != "" && f.Marketing != f.Carrier && !seen[f.Marketing+"/"+f.Carrier] {
			seen[f.Marketing+"/"+f.Carrier] = true
			shares = append(shares, [2]string{f.Marketing, f.Carrier})
		}
	}
	for ap, by := range deps {
		best := ""
		for code, n := range by {
			if n > by[best] || (n == by[best] && code < best) {
				best = code
			}
		}
		c.home[ap] = best
	}
	c.partners = partners.New(codes, shares)
	s.commerce = c
	if s.tariff != nil {
		s.tariff.Partners = c.partners.Of
		s.tariff.WebOnly = func(carrier string) bool { return s.policyOf(carrier).WebOnly }
		s.tariff.Surcharge = s.fuelSurcharge
	}
	s.initPricing(m)
	if s.Ledger != nil {
		s.Ledger.Wire = strings.ToUpper(s.BookingDate.Format("02Jan"))
		s.Ledger.Charge = c.charge
		s.Ledger.Sale = s.costOfSale
	}
}

// charge is what the carrier that ticketed a coupon keeps of its value
// when another carrier flew it, in hundredths of a percent, by what the
// two are to each other.
func (c *commerce) charge(ticketed, flown string) int {
	switch c.partners.Between(ticketed, flown) {
	case partners.Venture:
		return 0
	case partners.Codeshare:
		return partners.CodeshareCharge
	case partners.Alliance:
		return partners.AllianceCharge
	}
	return partners.InterlineCharge
}

// policyOf is how a carrier sells: its own policy when someone has set
// one, its kind's otherwise.
func (s *Sim) policyOf(carrier string) distribution.Policy {
	carrier = strings.ToUpper(carrier)
	if s.commerce != nil {
		s.commerce.mu.RLock()
		p, ok := s.commerce.policy[carrier]
		s.commerce.mu.RUnlock()
		if ok {
			return p
		}
	}
	return distribution.Default(equipment.ModelOf(carrier))
}

// setPolicy changes how a carrier sells.
func (s *Sim) setPolicy(carrier string, change func(*distribution.Policy)) distribution.Policy {
	carrier = strings.ToUpper(carrier)
	p := s.policyOf(carrier)
	// A copy of the maps: the default's are shared.
	cp := distribution.Policy{Sells: map[string]bool{}, Surcharge: map[string]int64{}, Override: map[string]int{}, WebOnly: p.WebOnly, FullContent: p.FullContent}
	for k, v := range p.Sells {
		cp.Sells[k] = v
	}
	for k, v := range p.Surcharge {
		cp.Surcharge[k] = v
	}
	for k, v := range p.Override {
		cp.Override[k] = v
	}
	change(&cp)
	s.commerce.mu.Lock()
	s.commerce.policy[carrier] = cp
	s.commerce.mu.Unlock()
	return cp
}

// fuelSurcharge is a network carrier's own fee on a long international
// flight, in cents, by the formula the Japanese carriers publish and
// most others follow in effect: nothing while fuel is under a baseline,
// and a step for each ten dollars a barrel over it, on the two-month
// average price. The baseline here is two dollars a gallon (84 dollars a
// barrel); a step is 40 dollars on a long-haul sector and 17 on a
// medium-haul one. Carriers within the United States and low-cost
// carriers raise fares instead.
func (s *Sim) fuelSurcharge(carrier string, km int, international bool) int64 {
	if s.econ == nil || !international || km < 1500 || equipment.ModelOf(carrier) != equipment.Network {
		return 0
	}
	s.commerce.mu.RLock()
	off := s.commerce.noSurcharge[carrier]
	s.commerce.mu.RUnlock()
	if off {
		return 0
	}
	over := s.econ.fuelIndex(int(s.econ.day.Load())) - 2.00
	if over <= 0 {
		return 0
	}
	steps := over / 0.238
	per := 1700.0
	if km >= 3500 {
		per = 4000
	}
	return int64(steps*per/100) * 100
}

// regionOfChannel names the channel a record was sold through, as the
// distribution costs know them.
func saleChannel(r *pnr.PNR) string {
	switch c := strings.ToLower(r.Origin.Channel); c {
	case market.Direct, market.OTA, market.TMC, market.Agency:
		return c
	case "ndc":
		return market.Direct
	}
	return market.Agency
}

// costOfSale is what a ticket's sale cost the carrier that ticketed it.
func (s *Sim) costOfSale(r *pnr.PNR, ticketing string, segments int, fareCents, paid int64) revenue.Cost {
	sale := distribution.Sale{Channel: saleChannel(r), Segments: segments, Fare: fareCents, Paid: paid}
	if s.econ != nil {
		for _, sg := range r.Segments {
			if sg.Type == pnr.SegmentAir {
				sale.Region = s.econ.region[sg.Board]
				break
			}
		}
		sale.Home = sale.Region == s.econ.region[s.carriers[ticketing].Hub]
	}
	c := s.policyOf(ticketing).CostOf(sale)
	return revenue.Cost{System: c.System, Commission: c.Commission, Payment: c.Payment, Own: c.Own}
}

// connectTime is the least time a passenger needs between two flights at
// a station: the station's minimum connecting time for the pair's
// status, ten minutes less for a carrier connecting to itself at its own
// hub, which is what hub carriers file for their online connections.
func (s *Sim) connectTime(in, out world.Flight) int {
	h := in.To
	intl := func(f world.Flight) bool { return s.airports[f.From].Country != s.airports[f.To].Country }
	deps := 0
	if s.econ != nil {
		deps = s.econ.deps[h]
	}
	m, _ := airport.MCT(h, deps, airport.Status(intl(in), intl(out)))
	if in.Carrier == out.Carrier && s.carriers[in.Carrier].Hub == h {
		m = max(20, m-10)
	}
	return m
}

// path is one way to fly a market: one flight, or two that connect.
type path []world.Flight

// pathsFor lays out the ways to fly from o to d today: the nonstops of
// every carrier, and the connections over any point that make the
// minimum connecting time and do not wait more than five hours. A sample
// of each when there are many, as a shopping display shows a page and
// not the timetable.
func (s *Sim) pathsFor(rng *rand.Rand, o, d string) []path {
	var nonstop, connect []path
	from := s.onwardFrom(o)
	if len(from) == 0 {
		return nil
	}
	start := rng.Intn(len(from))
	scanned := 0
	for i := 0; i < len(from) && scanned < 600; i++ {
		f1 := from[(start+i)%len(from)]
		scanned++
		if s.isClosed(f1.From, f1.To) {
			continue
		}
		if f1.To == d {
			if len(nonstop) < 14 {
				nonstop = append(nonstop, path{f1})
			}
			continue
		}
		if len(connect) >= 40 {
			continue
		}
		onward := s.commerce.pair[f1.To+"/"+d]
		if len(onward) == 0 {
			continue
		}
		var best *world.Flight
		bestWait := 0
		for k := range onward {
			f2 := &onward[k]
			wait := f2.DepMin - f1.ArrMin
			if wait < s.connectTime(f1, *f2) || wait > 300 {
				continue
			}
			// The same carrier's flight before anyone else's, then the
			// shortest wait.
			own := f2.Carrier == f1.Carrier
			if best == nil || (own && best.Carrier != f1.Carrier) || (own == (best.Carrier == f1.Carrier) && wait < bestWait) {
				best, bestWait = f2, wait
			}
		}
		if best != nil && !s.isClosed(best.From, best.To) {
			connect = append(connect, path{f1, *best})
		}
	}
	// Of the connections, the carrier's own and its partners' first, then
	// the shortest: eight of them.
	rank := func(p path) int {
		if p[0].Carrier == p[1].Carrier {
			return 0
		}
		return s.commerce.partners.Rank(p[0].Carrier, p[1].Carrier)
	}
	sort.SliceStable(connect, func(i, j int) bool {
		if ri, rj := rank(connect[i]), rank(connect[j]); ri != rj {
			return ri < rj
		}
		return connect[i][1].ArrMin-connect[i][0].DepMin < connect[j][1].ArrMin-connect[j][0].DepMin
	})
	if len(connect) > 8 {
		connect = connect[:8]
	}
	return append(nonstop, connect...)
}

// returnOf finds the way home for a path: the same carriers the other
// way round, connecting where the way out connected. nil when a carrier
// does not fly back.
func (s *Sim) returnOf(p path) path {
	var back path
	prevArr := -1
	for i := len(p) - 1; i >= 0; i-- {
		leg := p[i]
		var pick *world.Flight
		cands := s.commerce.pair[leg.To+"/"+leg.From]
		for k := range cands {
			f := &cands[k]
			if f.Carrier != leg.Carrier {
				continue
			}
			if prevArr >= 0 && (f.DepMin < prevArr+45 || f.DepMin > prevArr+300) {
				continue
			}
			if pick == nil || f.DepMin < pick.DepMin {
				pick = f
			}
		}
		if pick == nil {
			return nil
		}
		back = append(back, *pick)
		prevArr = pick.ArrMin
	}
	return back
}

// sold is how a flight is sold: under its marketing carrier's code nine
// times in ten when it has one, as codeshares are.
func sold(rng *rand.Rand, f world.Flight) (carrier, number string) {
	if f.Marketing != "" && f.Marketing != f.Carrier && f.MarketingNumber != "" && rng.Intn(10) < 9 {
		return f.Marketing, f.MarketingNumber
	}
	return f.Carrier, f.Number
}

// shopped is one offer with what it takes to book it.
type shopped struct {
	offer   market.Offer
	quote   *fare.Quote
	out     path
	back    path
	stay    int
	codes   [][2]string // carrier and number each segment is sold under
	channel string
}

// shop prices every path for a traveller and returns the offers.
func (s *Sim) shop(rng *rand.Rand, g *GDSNode, t market.Traveller, paths []path) []shopped {
	day := s.BookingDate
	at := func(d time.Time, min int) time.Time { return d.Add(time.Duration(min) * time.Minute) }
	var out []shopped
	for _, p := range paths {
		codes := make([][2]string, 0, len(p)*2)
		for _, f := range p {
			c, n := sold(rng, f)
			codes = append(codes, [2]string{c, n})
		}
		seller := codes[0][0]
		pol := s.policyOf(seller)
		// Where an aircraft on the way has no front cabin the traveller
		// sits behind.
		want := t
		for _, f := range p {
			if _, ok := host.Cabins(f, s.capacity)[t.Cabin]; t.Cabin != "" && !ok {
				want.Cabin = ""
			}
		}
		var back path
		if t.RoundTrip {
			back = s.returnOf(p)
		}
		// The stays to price: the one wanted, and for a traveller home
		// before the weekend the one that holds a Saturday night, which
		// some fares want and which they can have at a cost.
		stays := []int{t.Stay}
		if back != nil && !t.SaturdayNight {
			stays[0] = min(t.Stay, daysToSunday(day)-1)
			if stays[0] < 0 {
				stays[0] = 0
			}
			stays = append(stays, daysToSunday(day))
		} else if back != nil {
			stays[0] = max(t.Stay, daysToSunday(day))
		}
		channels := []string{t.Channel}
		if !pol.Offered(t.Channel) {
			// Not sold where the traveller shops: they find it on the
			// carrier's own site two times in three.
			if rng.Intn(3) == 0 {
				continue
			}
			channels = []string{market.Direct}
		} else if t.Channel != market.Direct && (pol.WebOnly || pol.Surcharge[t.Channel] > 0) {
			channels = append(channels, market.Direct)
		}
		for si, stay := range stays {
			for _, ch := range channels {
				req := fare.Request{Purchased: at(day, p[0].DepMin).AddDate(0, 0, -t.Lead), Sale: fare.Sale{Channel: ch, Account: t.Account},
					Distance: s.tariff.Miles}
				for i := 0; i < t.Party; i++ {
					req.Passengers = append(req.Passengers, fare.Adult)
				}
				for i, f := range p {
					req.Segments = append(req.Segments, fare.Segment{Carrier: codes[i][0], FlightNum: codes[i][1], Origin: f.From, Destination: f.To,
						Depart: at(day, f.DepMin), Arrive: at(day, f.ArrMin), Miles: int(float64(f.KM) / 1.609)})
				}
				all := codes
				if back != nil {
					home := day.AddDate(0, 0, stay)
					all = append([][2]string{}, codes...)
					for _, f := range back {
						c, n := f.Carrier, f.Number
						// Home under the code they went out on.
						for i, o := range p {
							if o.Carrier == f.Carrier && codes[i][0] != o.Carrier && f.Marketing == codes[i][0] && f.MarketingNumber != "" {
								c, n = f.Marketing, f.MarketingNumber
							}
						}
						all = append(all, [2]string{c, n})
						req.Segments = append(req.Segments, fare.Segment{Carrier: c, FlightNum: n, Origin: f.From, Destination: f.To,
							Depart: at(home, f.DepMin), Arrive: at(home, f.ArrMin), Miles: int(float64(f.KM) / 1.609)})
					}
				}
				nOut := len(p)
				req.Open = func(seg int, class string) bool {
					if !want.Sells(class) {
						return false
					}
					if seg >= nOut {
						return true // the way home is weeks off
					}
					f := p[seg]
					k := avail.NewKey(codes[seg][0], codes[seg][1], day, f.From, f.To, class)
					return g.GW.Avail == nil || g.GW.Avail.Status(k) != avail.Closed
				}
				q, err := fare.Price(s.tariff, req)
				if err != nil {
					continue
				}
				dirs := int64(1)
				if back != nil {
					dirs = 2
				}
				// What the airline charges: the fare and its own fees. The
				// taxes are the same whoever flies, and what a traveller
				// will pay is measured in fares.
				price := (q.Base.Amount+q.CarrierFees.Amount)/int64(t.Party)/dirs + pol.Surcharge[ch]
				o := market.Offer{Carriers: []string{seller}, Connections: len(p) - 1, Depart: p[0].DepMin, Price: price, Channel: ch,
					Refundable: true, SaturdayNight: si == 1, Bonus: pol.Favour(ch, price)}
				for _, c := range q.Components {
					if !c.Fare.Rule.Refundable {
						o.Refundable = false
					}
					if c.Fare.Rule.ChangeFee.Amount > 0 || c.Fare.Rule.NoChanges {
						o.ChangeFee = true
					}
				}
				for _, f := range p[1:] {
					o.Carriers = append(o.Carriers, f.Carrier)
				}
				if t.Loyal != "" && seller != t.Loyal {
					o.Partner = s.commerce.partners.Between(t.Loyal, seller) != partners.None
				}
				out = append(out, shopped{offer: o, quote: q, out: p, back: back, stay: stay, codes: all, channel: ch})
			}
		}
	}
	return out
}

// daysToSunday is the days from a date to the Sunday after it: the
// shortest stay that holds a Saturday night.
func daysToSunday(d time.Time) int {
	n := (7 - int(d.Weekday())) % 7
	if n == 0 {
		n = 7
	}
	return n
}
