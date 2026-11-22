package sim

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/fare"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/world"
)

// Demand generates travellers continuously at roughly perMinute.
//
// This is the third demand model. The first booked one random segment
// and walked away. The second booked itineraries and lived with them --
// ticketed, cancelled, divided -- but chose a flight at random, so a
// carrier's fares decided nothing. This one is a market: a traveller
// wants to go from one place to another, has a most they will pay and a
// price on every inconvenience (internal/market), is shown what every
// carrier offers in the channel they shop in, and takes the offer that
// costs them least. A carrier that cuts a fare takes travellers from the
// ones that did not. A carrier that closes a class sends its travellers
// to another carrier, or home.
//
// The day the world flies is the day every traveller departs, and the
// booking window is folded into it: each traveller buys as of their own
// lead time, weeks or days before, and the fare rules count from there.
func (s *Sim) Demand(ctx context.Context, perMinute int, seed int64) {
	if perMinute <= 0 {
		return
	}
	var carriers []string
	for code := range s.Flights {
		if len(s.Flights[code]) > 0 {
			carriers = append(carriers, code)
		}
	}
	if len(carriers) == 0 {
		return
	}
	for i := 1; i < len(carriers); i++ {
		for j := i; j > 0 && carriers[j] < carriers[j-1]; j-- {
			carriers[j], carriers[j-1] = carriers[j-1], carriers[j]
		}
	}

	interval := time.Minute / time.Duration(perMinute)
	tick := time.NewTicker(interval)
	defer tick.Stop()
	report := time.NewTicker(30 * time.Second)
	defer report.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-report.C:
			s.log.Info("demand",
				"booked", s.DemBooked.Load(), "failed", s.DemFailed.Load(),
				"interline", s.DemInterline.Load(), "ndc", s.DemNDC.Load(),
				"ticketed", s.DemTicketed.Load(), "cancelled", s.DemCancelled.Load(),
				"split", s.DemSplit.Load(), "movements", s.Movements.Load())
		case <-tick.C:
			n++
			// Each traveller gets a private generator seeded from the draw,
			// so the concurrent lifecycles do not contend for one rng.
			sub := rand.New(rand.NewSource(seed<<16 ^ int64(n)))
			// Travellers spread across the distribution systems, the way real
			// demand arrives through whichever channel the agent uses.
			go s.placeDemand(ctx, s.GDSes[n%len(s.GDSes)], sub, s.sellableCarriers(carriers), n)
		}
	}
}

// sellOffset picks which day a traveller books for. The default is the day
// the world flies: a simulated day is minutes of wall time, so bookings
// spread over four dates gave the flown one a quarter of a volume that was
// already a sixtieth of real -- and every flight left nearly empty. A
// deployment that wants the selling window back sets SellDays above one.
func (s *Sim) sellOffset(rng *rand.Rand) int {
	days := s.sellDays
	if days <= 1 {
		return 0
	}
	return rng.Intn(days) - 1
}

// pickMarket draws the trip a traveller wants: the origin and
// destination of a flight in the schedule, or two times in five of a
// flight and one that leaves where it lands. Markets are drawn by their
// flights, so demand stands in proportion to what the schedule offers.
func (s *Sim) pickMarket(rng *rand.Rand, carriers []string) (o, d string, km int) {
	total := 0
	for _, c := range carriers {
		total += len(s.scheduleOf(c))
	}
	if total == 0 {
		return "", "", 0
	}
	r := rng.Intn(total)
	var f1 world.Flight
	for _, c := range carriers {
		fs := s.scheduleOf(c)
		if r < len(fs) {
			f1 = fs[r]
			break
		}
		r -= len(fs)
	}
	if s.isClosed(f1.From, f1.To) {
		return "", "", 0
	}
	if rng.Intn(100) < 60 || f1.ArrMin >= 23*60 {
		return f1.From, f1.To, f1.KM
	}
	onward := s.onwardFrom(f1.To)
	start := rng.Intn(maxInt(1, len(onward)))
	for i := 0; i < len(onward) && i < 80; i++ {
		f2 := onward[(start+i)%len(onward)]
		if f2.DepMin < f1.ArrMin+30 || f2.To == f1.From || s.isClosed(f2.From, f2.To) {
			continue
		}
		km := s.tariff.Distance(f1.From, f2.To)
		if km == 0 {
			km = f1.KM + f2.KM
		}
		// Nobody flies out and half way back: a connection more than one
		// and a half times the direct distance is not a trip.
		if (f1.KM+f2.KM)*2 > km*3+400 {
			continue
		}
		return f1.From, f2.To, km
	}
	return f1.From, f1.To, f1.KM
}

// baseFare is a market's lowest regular fare, from which a traveller's
// willingness to pay is measured: the tariff's, over the great circle
// between the two ends when the tariff knows them.
func (s *Sim) baseFare(o, d string, km int) int64 {
	if direct := s.tariff.Distance(o, d); direct > 0 {
		km = direct
	}
	return s.tariff.BaseFare(o, d, km)
}

// mixFor is a market's travellers: who is on business and where each
// kind books, by the part of the world the trip starts in.
func (s *Sim) mixFor(o, d string, km int) market.Mix {
	region := ""
	long := km > 4000
	if s.econ != nil {
		region = s.econ.region[o]
		long = long || region != s.econ.region[d]
	}
	return market.MixFor(region, long)
}

// travellerFor draws the traveller for a market.
func (s *Sim) travellerFor(rng *rand.Rand, o, d string, km int) (market.Traveller, int64) {
	base := s.baseFare(o, d, km)
	t := market.Draw(rng, base, s.mixFor(o, d, km))
	t.Join(rng, s.commerce.home[o], base)
	return t, base
}

// placeDemand runs one traveller end to end: want a trip, shop, choose,
// book, and live with the consequences.
func (s *Sim) placeDemand(ctx context.Context, g *GDSNode, rng *rand.Rand, carriers []string, n int) {
	if s.tariff == nil || s.commerce == nil {
		return
	}
	o, d, km := s.pickMarket(rng, carriers)
	if o == "" {
		return
	}
	t, _ := s.travellerFor(rng, o, d, km)
	// A traveller who does not buy from the carrier buys from an agency:
	// one of the world's, by where the trip starts.
	var ag distribution.Agency
	if t.Channel != market.Direct {
		region := ""
		if s.econ != nil {
			region = s.econ.region[o]
		}
		ag, _ = distribution.Pick(rng, s.commerce.agencies, t.Channel, region)
		g = s.nodeOf(ag, g)
	}
	found := s.shop(rng, g, t, ag, s.pathsFor(rng, o, d))
	if len(found) == 0 {
		s.DemNoOffer.Add(1)
		return
	}
	offers := make([]market.Offer, len(found))
	for i := range found {
		offers[i] = found[i].offer
	}
	pick := t.Choose(offers)
	if pick < 0 {
		// Every fare on offer was more than this traveller would pay.
		s.DemPricedOut.Add(1)
		return
	}
	s.bookOffer(ctx, g, rng, t, found[pick], n)
}

// bookOffer books what a traveller chose, at the fare they were quoted,
// with whoever sells it: the carrier's own system, the traveller's
// agency through its distribution system, or the agency over its direct
// connection to the carrier.
func (s *Sim) bookOffer(ctx context.Context, g *GDSNode, rng *rand.Rand, t market.Traveller, sh shopped, n int) {
	surname := fmt.Sprintf("DEMAND%06d", n)
	q := sh.quote
	seller := sh.codes[0][0]
	pol := s.policyOf(seller)
	extra := pol.Surcharge[sh.channel]
	direct := sh.channel == market.Direct
	if !direct && sh.agency.Kind != market.TMC {
		extra += sh.agency.Fee
	}
	if extra > 0 {
		q = withFee(q, extra)
	}
	legs := append(append(path{}, sh.out...), sh.back...)
	classes := q.Passengers[0].Segments
	interline := false
	for _, f := range legs[1:] {
		if f.Carrier != legs[0].Carrier {
			interline = true
		}
	}
	counted := func() {
		s.DemBooked.Add(1)
		if interline {
			s.DemInterline.Add(1)
		}
		if len(sh.out) > 1 {
			s.DemConnecting.Add(1)
		}
		if len(sh.back) > 0 {
			s.DemRoundTrip.Add(1)
		}
		s.noteSale(t, sh, q)
	}
	// The carrier's own system, when this machine runs it, sells what is
	// under its own code: its site's bookings, and the orders of an agency
	// connected to it directly.
	own := s.ownNode(seller)
	if own != nil && !sh.ownCode(seller) {
		own = nil
	}
	connected := !direct && own != nil && sh.agency.Connect && pol.Connect && rng.Float64() < sh.agency.ConnectShare

	// One person on one flight one way can be an NDC order: the HTTP
	// surface a carrier's site and a connected agency call. The node
	// prices an order itself, as of now, so only a fare whose advance
	// purchase is met today goes this way, and an order the node refuses
	// is booked the ordinary way.
	if len(legs) == 1 && t.Party == 1 && q == sh.quote && (connected || (direct && rng.Intn(2) == 0)) {
		f := legs[0]
		daysOut := int(time.Until(s.BookingDate.Add(time.Duration(f.DepMin)*time.Minute)).Hours() / 24)
		rule := q.Components[0].Fare.Rule
		// A carrier's own node takes the order; where this machine does
		// not run the carrier, a distribution system's does.
		node, name, id := seller, seller+" WEB", "WEB"
		if connected {
			name, id = sh.agency.Name, sh.agency.Code
		} else if own == nil {
			node, name, id = "", "WHOLESKY OTA", "SKY00OTA"
			if g != nil && len(rule.Sale.Channels) == 0 {
				node = g.Designator
			}
		}
		if node != "" && rule.AdvancePurchaseDays <= daysOut {
			if err := s.orderAt(node, name, id, seller, sh.codes[0][1], f.From, f.To, f.DepMin, classes[0].Class, surname); err == nil {
				s.DemNDC.Add(1)
				switch {
				case connected:
					s.DemConnected.Add(1)
				case own != nil:
					s.DemDirect.Add(1)
				}
				counted()
				return
			}
		}
	}

	segs := make([]gateway.BookingSegment, 0, len(legs))
	for i, f := range legs {
		day := s.BookingDate
		if i >= len(sh.out) {
			day = day.AddDate(0, 0, sh.stay)
		}
		segs = append(segs, gateway.BookingSegment{
			Carrier: sh.codes[i][0], FlightNum: sh.codes[i][1], Class: classes[i].Class,
			Date: strings.ToUpper(day.Format("02Jan")), Board: f.From, Off: f.To, Seats: t.Party,
		})
	}
	pax := make([]gateway.BookingPassenger, 0, t.Party)
	for p := 0; p < t.Party; p++ {
		pax = append(pax, gateway.BookingPassenger{
			Surname: surname, Given: fmt.Sprintf("PAX%d", p+1), Title: "MR",
		})
	}
	// A few travellers in every hundred need something at the airport --
	// a wheelchair, an unaccompanied child, a medical case -- and the
	// request rides the booking so the name list can carry it to check-in
	// and the PSM can carry it to the arrival station.
	var ssrs []gateway.BookingSSR
	if r := rng.Intn(100); r < 3 {
		codes := []string{"WCHR", "WCHR", "UMNR", "MEDA", "BLND", "DEAF", "MAAS"}
		ssrs = append(ssrs, gateway.BookingSSR{Code: codes[rng.Intn(len(codes))], Carrier: seller})
	}
	req := &gateway.BookingRequest{
		Passengers: pax, Segments: segs, SSRs: ssrs, Channel: sh.channel,
		Country: s.airports[legs[0].From].Country, Account: t.Account, Quote: q,
	}
	switch {
	case direct && own != nil:
		// The carrier's own site, on the carrier's own system.
		req.Agent, req.ReceivedFrom = "WEB", seller+" WEB"
		locator, ok := s.atCarrier(ctx, own, req)
		if !ok {
			return
		}
		s.DemDirect.Add(1)
		counted()
		s.liveWith(ctx, own.Gateway, own.Store, rng, locator, t.Party)
		return
	case connected:
		// The agency's direct connection, for what an order cannot carry.
		req.Agent, req.ReceivedFrom, req.Channel = sh.agency.Code, sh.agency.Name, distribution.NDC
		locator, ok := s.atCarrier(ctx, own, req)
		if !ok {
			return
		}
		s.DemConnected.Add(1)
		counted()
		s.liveWith(ctx, own.Gateway, own.Store, rng, locator, t.Party)
		return
	case direct:
		// The carrier's site, where this machine does not run the
		// carrier: through a distribution system, as before.
		req.Agent = "wholesky direct"
	default:
		req.Agent, req.ReceivedFrom = sh.agency.Code, sh.agency.Name
		if req.Agent == "" {
			req.Agent = "wholesky " + sh.channel
		}
	}
	if g == nil {
		return
	}
	res, err := g.GW.Book(ctx, req)
	if err != nil {
		s.noteDemandFailure(err)
		return
	}
	counted()
	s.liveWith(ctx, g.GW, g.Store, rng, res.PNR.RecordLocator, t.Party)
}

// noteSale counts what the travellers paid, for the instruments that say
// how the market is doing: the fare by kind of traveller.
func (s *Sim) noteSale(t market.Traveller, sh shopped, q *fare.Quote) {
	dirs := int64(1)
	if len(sh.back) > 0 {
		dirs = 2
	}
	s.DemDirections.Add(dirs * int64(t.Party))
	s.DemFarePaid.Add(q.Base.Amount + q.CarrierFees.Amount)
	if t.Kind == market.Business {
		s.DemBusiness.Add(1)
	}
}

// withFee is a quote with a carrier's own charge added to every
// passenger: the distribution cost charge a carrier puts on a ticket
// sold through a channel it would rather not pay for.
func withFee(q *fare.Quote, cents int64) *fare.Quote {
	out := *q
	out.Passengers = append([]fare.PassengerQuote(nil), q.Passengers...)
	for i := range out.Passengers {
		p := &out.Passengers[i]
		p.Taxes = append(append([]fare.TaxLine(nil), p.Taxes...), fare.TaxLine{Code: "OB", Amount: fare.Money{Amount: cents, Currency: q.Currency}, CarrierImposed: true})
		p.CarrierFees.Amount += cents
		p.CarrierFees.Currency = q.Currency
		p.Total.Amount += cents
		out.Taxes.Amount += cents
		out.CarrierFees.Amount += cents
		out.CarrierFees.Currency = q.Currency
		out.Total.Amount += cents
	}
	return &out
}

// liveWith is the rest of a booking's life: settle, then usually ticket,
// sometimes cancel, occasionally divide.
func (s *Sim) liveWith(ctx context.Context, gw *gateway.Gateway, st store.Store, rng *rand.Rand, locator string, party int) {
	deadline := time.Now().Add(20 * time.Second)
	for {
		ok, err := settledIn(ctx, st, locator)
		if err != nil || time.Now().After(deadline) {
			return
		}
		if ok {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(150 * time.Millisecond):
		}
	}

	roll := rng.Intn(100)
	switch {
	case roll < 8:
		// A ticketed booking's money goes back before the seats do: the
		// refund the settlement plan reports, then the cancellation.
		if rec, err := st.GetPNR(ctx, locator); err == nil && rec.Ticketed() {
			if _, err := gw.Refund(ctx, locator, gateway.RefundOptions{By: "demand", Reason: "traveller cancelled"}); err == nil {
				s.DemRefunded.Add(1)
			}
		}
		if _, err := gw.Cancel(ctx, locator, gateway.CancelOptions{
			By: "demand", Reason: "traveller cancelled",
		}); err == nil {
			s.DemCancelled.Add(1)
			s.demMu.Lock()
			s.DemCancelledLocs = append(s.DemCancelledLocs, locator)
			s.demMu.Unlock()
		}
		return
	case roll < 63:
		rec, err := st.GetPNR(ctx, locator)
		if err != nil || len(rec.Segments) == 0 {
			return
		}
		code := s.accountingCode(rec.Segments[0].Carrier)
		if _, err := gw.IssueTickets(ctx, locator, gateway.IssueOptions{
			AirlineCode: code, IssuedBy: "demand",
		}); err == nil {
			s.DemTicketed.Add(1)
		}
	}

	// A party sometimes divides -- somebody's plans changed -- which is the
	// operation whose advisories and divergences the map draws.
	if party >= 2 && rng.Intn(100) < 12 {
		if _, err := gw.Split(ctx, gateway.SplitRequest{
			Locator: locator, Passengers: []int{party},
			By: "demand", Reason: "traveller separated",
		}); err == nil {
			s.DemSplit.Add(1)
		}
	}
}

// accountingCode is the three-digit stock code a carrier's tickets are
// issued against. A ticket's code names its carrier to settlement and to
// interline billing, so each carrier of the world has its own: the code
// starts from a sum of the designator and moves up to the next free one
// when another carrier has it. Taken from the sum alone, 2 carriers could
// share a code, and one's tickets settled in the other's statement.
func (s *Sim) accountingCode(carrier string) string {
	s.acctOnce.Do(func() {
		codes := make([]string, 0, len(s.carriers))
		for code := range s.carriers {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		s.acct = make(map[string]string, len(codes))
		taken := map[int]bool{}
		for _, code := range codes {
			n := stockSum(code)
			for i := 0; i < 900 && taken[n]; i++ {
				n = 100 + (n-100+1)%900
			}
			taken[n] = true
			s.acct[code] = fmt.Sprintf("%03d", n)
		}
	})
	if code, ok := s.acct[carrier]; ok {
		return code
	}
	return fmt.Sprintf("%03d", stockSum(carrier))
}

// stockSum is where a designator's stock code starts: 100 to 999.
func stockSum(carrier string) int {
	h := 0
	for _, r := range carrier {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return 100 + h%900
}

type worldFlight = world.Flight

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// noteDemandFailure counts a booking that did not happen and, for one in a
// hundred, says why: a day of silent failures cost an evening once, when
// every booking on the recorded day failed and the log showed a counter.
func (s *Sim) noteDemandFailure(err error) {
	if n := s.DemFailed.Add(1); n%100 == 1 {
		s.log.Warn("demand booking failed", "err", err, "failed", n)
	}
}
