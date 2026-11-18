package sim

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/market"
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
		found := s.shop(rng, s.GDSes[0], t, s.pathsFor(rng, o, d))
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
