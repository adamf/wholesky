package sim

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/market"
	"github.com/adamf/wholesky/internal/revenue"
)

// marketSample runs n travellers through the shop without booking them
// and reports what they chose.
type marketSample struct {
	n, noOffer, pricedOut, bought int
	class                         map[string]int
	channel                       map[string]int
	carrier                       map[string]int
	connecting, interline, round  int
	business                      int
	paid, base                    float64        // per direction, per passenger
	by                            map[string]int // carrier/channel
}

func sample(s *Sim, n int, seed int64, carriers []string) marketSample {
	out := marketSample{class: map[string]int{}, channel: map[string]int{}, carrier: map[string]int{}, by: map[string]int{}}
	for i := 0; i < n; i++ {
		rng := rand.New(rand.NewSource(seed<<20 ^ int64(i)))
		o, d, km := s.pickMarket(rng, carriers)
		if o == "" {
			continue
		}
		out.n++
		t, base := s.travellerFor(rng, o, d, km)
		g := s.GDSes[0]
		var ag distribution.Agency
		if t.Channel != market.Direct {
			ag, _ = distribution.Pick(rng, s.commerce.agencies, t.Channel, s.econ.region[o])
			g = s.nodeOf(ag, g)
		}
		found := s.shop(rng, g, t, ag, s.pathsFor(rng, o, d))
		if len(found) == 0 {
			out.noOffer++
			continue
		}
		offers := make([]market.Offer, len(found))
		for k := range found {
			offers[k] = found[k].offer
		}
		pick := t.Choose(offers)
		if pick < 0 {
			out.pricedOut++
			continue
		}
		sh := found[pick]
		out.bought++
		out.class[sh.quote.Passengers[0].Segments[0].Class]++
		out.channel[sh.channel]++
		out.carrier[sh.codes[0][0]]++
		if len(sh.out) > 1 {
			out.connecting++
			if sh.out[0].Carrier != sh.out[1].Carrier {
				out.interline++
			}
		}
		dirs := 1.0
		if len(sh.back) > 0 {
			out.round++
			dirs = 2
		}
		if t.Kind == market.Business {
			out.business++
		}
		out.paid += float64(sh.quote.Base.Amount+sh.quote.CarrierFees.Amount) / float64(t.Party) / dirs
		out.base += float64(base)
		out.by[sh.codes[0][0]+"/"+sh.channel]++
	}
	return out
}

func bootQuiet(t *testing.T) (*Sim, []string, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	s, err := Boot(ctx, smallWorld(t), Options{GDSCount: 1, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	morning(s)
	var carriers []string
	for code := range s.Flights {
		if len(s.Flights[code]) > 0 {
			carriers = append(carriers, code)
		}
	}
	sort.Strings(carriers)
	return s, carriers, ctx
}

// The market answers to what carriers do. The same travellers are sent
// through the shop before and after each move, so what changes is the
// move's doing: a carrier that cuts its fares takes travellers from the
// others, one that raises them loses travellers, one that leaves a
// channel loses most of that channel's sales, and one that puts a charge
// on a channel moves its sales to its own site.
func TestTheMarketAnswersToFares(t *testing.T) {
	if raceEnabled {
		t.Skip("seven thousand travellers priced on one goroutine: CI runs it without the race detector")
	}
	s, carriers, _ := bootQuiet(t)
	const n, seed = 1500, 7
	base := sample(s, n, seed, carriers)
	t.Logf("%d travellers: %d found no way to go, %d were priced out, %d bought", base.n, base.noOffer, base.pricedOut, base.bought)
	t.Logf("carriers %v, channels %v, classes %v", base.carrier, base.channel, base.class)
	t.Logf("%d connect (%d on two carriers), %d come back, %d on business", base.connecting, base.interline, base.round, base.business)
	if base.bought < n*7/10 || base.pricedOut == 0 || base.connecting == 0 || base.round == 0 || base.business == 0 {
		t.Fatalf("the market is not one: %+v", base)
	}
	if len(base.class) < 5 {
		t.Errorf("travellers buy up and down the ladder: %v", base.class)
	}
	// The carrier with the second most sales, so there is somebody to
	// take from and somebody to lose to.
	codes := append([]string(nil), carriers...)
	sort.Slice(codes, func(i, j int) bool { return base.carrier[codes[i]] > base.carrier[codes[j]] })
	c := codes[1]
	was := base.carrier[c]

	s.tariff.SetMultiplier(c, 0.80)
	cut := sample(s, n, seed, carriers)
	s.tariff.SetMultiplier(c, 1.25)
	dear := sample(s, n, seed, carriers)
	s.tariff.SetMultiplier(c, 1)
	t.Logf("%s sold %d; at 0.80 of its fares %d; at 1.25 of them %d", c, was, cut.carrier[c], dear.carrier[c])
	if cut.carrier[c] < was*11/10 {
		t.Errorf("a 20%% cut took %s from %d sales to %d", c, was, cut.carrier[c])
	}
	if dear.carrier[c] > was*9/10 {
		t.Errorf("a 25%% rise took %s from %d sales to %d", c, was, dear.carrier[c])
	}
	// The cut's travellers came from somewhere: the others sold less, and
	// some who were priced out now fly.
	var othersWas, othersNow int
	for _, o := range codes {
		if o != c {
			othersWas += base.carrier[o]
			othersNow += cut.carrier[o]
		}
	}
	if othersNow >= othersWas || cut.pricedOut >= base.pricedOut {
		t.Errorf("the cut took nothing from the others (%d to %d) or brought nobody new (%d priced out to %d)", othersWas, othersNow, base.pricedOut, cut.pricedOut)
	}

	// Out of the travel management companies' screens.
	perChannel := func(m marketSample, carrier, channel string) int { return m.by[carrier+"/"+channel] }
	tmcWas := perChannel(base, c, market.TMC)
	s.setPolicy(c, func(p *distribution.Policy) { p.Sells[market.TMC] = false })
	out := sample(s, n, seed, carriers)
	if tmcWas == 0 || perChannel(out, c, market.TMC) != 0 || out.carrier[c] >= was {
		t.Errorf("%s left the channel: its sales there went %d to %d, its sales in all %d to %d", c, tmcWas, perChannel(out, c, market.TMC), was, out.carrier[c])
	}
	// Back in, with sixteen dollars on every ticket sold there.
	s.setPolicy(c, func(p *distribution.Policy) { p.Sells[market.TMC] = true; p.Surcharge[market.TMC] = 1600 })
	charged := sample(s, n, seed, carriers)
	t.Logf("%s through travel management companies: %d; out of the channel %d; with a 16 dollar charge %d there and %d direct (was %d)",
		c, tmcWas, perChannel(out, c, market.TMC), perChannel(charged, c, market.TMC), perChannel(charged, c, market.Direct), perChannel(base, c, market.Direct))
	if perChannel(charged, c, market.TMC) >= tmcWas || perChannel(charged, c, market.Direct) <= perChannel(base, c, market.Direct) {
		t.Errorf("the charge moved nothing: %d to %d in the channel, %d to %d direct", tmcWas, perChannel(charged, c, market.TMC), perChannel(base, c, market.Direct), perChannel(charged, c, market.Direct))
	}
}

// A filled day's scorecard counts the selling: revenue is fares and the
// carrier's own fees with the taxes left out, every ticket has a channel,
// and what the channels cost is on the bill, at a few per cent of the
// fares.
func TestScorecardCountsTheSelling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s, err := Boot(ctx, smallWorld(t), Options{GDSCount: 1, Fill: 0.8, FillSeed: 3, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.fillDay(ctx)
	s.rebuildLedger(ctx)
	w := seatWorld{s}
	counted := 0
	var taxes int64
	for code := range s.Tenants {
		acct := s.Ledger.AccountOf(code)
		if acct.Tickets == 0 {
			continue
		}
		counted++
		sc := w.score(code)
		dist, cards := sc.Costs["distribution"], sc.Costs["card fees"]
		sold := acct.Flown + acct.Kept
		t.Logf("%s: %d tickets %v; fares %d, taxes collected %d; distribution %d (%.1f%%), card fees %d (%.1f%%)", code, acct.Tickets, acct.Channels,
			sold/100, acct.Taxes/100, dist/100, 100*float64(dist)/float64(sold), cards/100, 100*float64(cards)/float64(sold))
		taxes += acct.Taxes
		if dist <= 0 || cards <= 0 {
			t.Errorf("%s: distribution %d, card fees %d", code, dist, cards)
		}
		if share := float64(dist+cards) / float64(sold); share < 0.02 || share > 0.12 {
			t.Errorf("%s spends %.1f%% of its fares on selling", code, share*100)
		}
		n := 0
		for _, k := range acct.Channels {
			n += k
		}
		if n != acct.Tickets || acct.Channels[market.Direct] == 0 || sc.Sales.Tickets != acct.Tickets {
			t.Errorf("%s: %d tickets, %v by channel", code, acct.Tickets, acct.Channels)
		}
		if !s.policyOf(code).Offered(market.Agency) && acct.Channels[market.Agency] != 0 {
			t.Errorf("%s does not sell through agents and sold %d tickets there", code, acct.Channels[market.Agency])
		}
	}
	if counted < 2 || taxes <= 0 {
		t.Fatalf("%d carriers sold, %d collected in taxes", counted, taxes)
	}
}

// morning holds the world's clock at four in the morning: the day's
// flights are all still to come, whatever hour the test runs at.
func morning(s *Sim) { s.clock.Sync(time.Now(), 4*60, 0) }

// After a restart on a database the ledger has its accounts back. The
// aggregate that answers at once has the taxes in and no accounts; the
// count that follows record by record replaces it. A sale made while the
// count runs is in the book the count reads and is also recorded as it
// is made: it is counted once, whichever of the two comes first.
func TestLedgerIsRecountedFromADatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	s, err := Boot(ctx, smallWorld(t), Options{GDSCount: 1, Fill: 0.8, FillSeed: 3, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	s.fillDay(ctx)
	// What the books hold, counted record by record from memory.
	s.rebuildLedger(ctx)
	s.rebuildLedger(ctx)
	want := map[string]revenue.Account{}
	for code := range s.Tenants {
		want[code] = s.Ledger.AccountOf(code)
	}
	total := s.Ledger.Total()
	if total == 0 {
		t.Fatal("the fill sold nothing")
	}
	var f worldFlight
	for code := range s.Tenants {
		if len(s.Flights[code]) > 0 {
			f = s.Flights[code][0]
			break
		}
	}
	// Two sales as the world makes them now: in the carrier's book, and
	// recorded in the ledger when the sale is heard. The first is heard
	// while the count runs and the second after it is done.
	sale := func(locator string) *pnr.PNR {
		return &pnr.PNR{RecordLocator: locator, CreatedAt: time.Now(), Status: pnr.StatusTicketed,
			Origin:     pnr.Origin{Party: f.Carrier, Channel: "direct"},
			Passengers: []pnr.Passenger{{Ref: 1, Surname: "LIVE"}},
			Segments:   []pnr.Segment{{Ref: 1, Type: pnr.SegmentAir, Carrier: f.Carrier, FlightNum: f.Number, Board: f.From, Off: f.To, WireDate: s.Ledger.Wire, Seats: 1}},
			Pricing:    &pnr.Pricing{Currency: "USD", Base: 12300, Taxes: 2000, Total: 14300, Passengers: []pnr.PassengerPricing{{Ref: 1, Segments: []int64{12300}, Taxes: 2000, Total: 14300}}}}
	}
	first, second := sale("LIVE01"), sale("LIVE02")
	// The same books in the store of a carrier on a database: a split
	// store, with the records in one store and the messages in another.
	// The ledger reads it in one aggregate and then record by record.
	for code, tn := range s.Tenants {
		mem, ok := tn.Store.(*store.Mem)
		if !ok {
			t.Skip("the tenants' books are not in memory")
		}
		if code == f.Carrier {
			if err := mem.LoadPNRs(ctx, []*pnr.PNR{first, second}, "test"); err != nil {
				t.Fatal(err)
			}
		}
		tn.Store = store.Split{Messages: store.NewMem(), Records: mem}
	}
	s.rebuildLedger(ctx)
	s.Ledger.Record(first)
	tickets := func() int { return s.Ledger.AccountOf(f.Carrier).Tickets }
	for deadline := time.Now().Add(60 * time.Second); tickets() < want[f.Carrier].Tickets+2; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the accounts never came back: %+v, want %+v and 2 more tickets", s.Ledger.AccountOf(f.Carrier), want[f.Carrier])
		}
	}
	s.Ledger.Record(second)
	if got := s.Ledger.Total(); got != total+2*12300 {
		t.Errorf("after the recount the ledger holds %d, want the fill's %d and 2 sales of 12300", got, total)
	}
	for code, w := range want {
		got := s.Ledger.AccountOf(code)
		if code == f.Carrier {
			got.Flown, got.Taxes, got.Tickets, got.Passengers = got.Flown-2*12300, got.Taxes-2*2000, got.Tickets-2, got.Passengers-2
			got.Channels["direct"] -= 2
		}
		if got.Flown != w.Flown || got.Taxes != w.Taxes || got.Tickets != w.Tickets || got.Passengers != w.Passengers || got.Kept != w.Kept || got.Paid != w.Paid ||
			got.Channels["direct"] != w.Channels["direct"] || got.Channels["ota"] != w.Channels["ota"] {
			t.Errorf("%s after the recount: %+v, want %+v", code, got, w)
		}
		// What the sales cost, for a carrier the two live sales are not
		// in.
		if code != f.Carrier && (got.System != w.System || got.Commission != w.Commission || got.Payment != w.Payment || got.Own != w.Own) {
			t.Errorf("%s after the recount, the cost of its sales: %+v, want %+v", code, got, w)
		}
	}
	// A count of books that have been counted again since is dropped.
	stale := revenue.New()
	stale.Wire = s.Ledger.Wire
	stale.Load(sale("STALE1"))
	if s.Ledger.Replace(nil, stale, s.Ledger.Epoch()-1) || s.Ledger.Total() != total+2*12300 {
		t.Errorf("a count from before the last rebuild was taken in: the ledger holds %d", s.Ledger.Total())
	}
}
