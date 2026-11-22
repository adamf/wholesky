package fill

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"

	"github.com/adamf/wholesky/internal/equipment"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// measured is what a filled world sold.
type measured struct {
	plan         Plan
	seats, sold  int
	records      int
	class        map[string]int // passengers by class
	channel      map[string]int
	repriced     int
	fare, market float64                        // per passenger-leg: fare paid, the report's average for the leg's length
	byModel      map[equipment.Model][3]float64 // fare, market, passengers
	byCarrier    map[string][2]float64          // fare, passengers
}

func measure(t *testing.T, m *world.Manifest, tf *tariff.Synthetic, opts Options) measured {
	t.Helper()
	out := measured{class: map[string]int{}, channel: map[string]int{}, byModel: map[equipment.Model][3]float64{}, byCarrier: map[string][2]float64{}}
	sink := func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
		for _, r := range recs {
			if r.Origin.Channel == "codeshare" || r.Pricing == nil {
				continue
			}
			out.records++
			pax := float64(len(r.Pricing.Passengers))
			out.channel[r.Origin.Channel] += len(r.Pricing.Passengers)
			for _, rm := range r.Remarks {
				if strings.HasPrefix(rm.Text, "FARE AS CHOSEN") {
					out.repriced++
				}
			}
			for _, sg := range r.Segments {
				out.sold += sg.Seats
				out.class[sg.Class] += sg.Seats
			}
			// The report's average is for the trip from origin to
			// destination, one way, whatever the route.
			first, last := r.Segments[0], r.Segments[len(r.Segments)-1]
			paid := float64(r.Pricing.Base+r.Pricing.CarrierFees) / 100
			want := tariff.MarketFare(tf.Distance(first.Board, last.Off)) * pax
			out.fare += paid
			out.market += want
			b := out.byModel[equipment.ModelOf(first.Carrier)]
			b[0], b[1], b[2] = b[0]+paid, b[1]+want, b[2]+pax
			out.byModel[equipment.ModelOf(first.Carrier)] = b
			c := out.byCarrier[first.Carrier]
			c[0], c[1] = c[0]+paid, c[1]+pax
			out.byCarrier[first.Carrier] = c
		}
		return nil
	}
	plan, err := Day(context.Background(), m, opts, sink)
	if err != nil {
		t.Fatal(err)
	}
	out.seats, out.plan = plan.Seats, plan
	return out
}

func usWorld(t *testing.T) *world.Manifest {
	t.Helper()
	m, err := world.Compile(world.CompileOptions{DataDir: "../../data", Seed: 1, Countries: []string{"United States"}, MaxCarriers: 12})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return m
}

// The filled day against the record. The Consumer Airfare Report's line
// says what travellers paid in a market of a length; a world filled to a
// load factor must carry that load, at that fare on average, with the
// ultra-low-cost carriers at about three quarters of it (the report's
// lowest-fare carrier averages 0.749 of its market). Every fare a
// traveller chose must be one the pricing sells them. This test is what
// holds tariff's mixRatio, the carriers' levels and this package's pool
// where they are: move the ladder, the limits or the travellers and it
// says which way to move them back.
//
// The day measured is one the carriers have learned their limits for:
// 30 days of booking windows are sold first, and each flight's limits on
// the day come from its own record (internal/rm). The report is of
// airlines whose revenue management knows its flights, so that is the
// day to hold against it. Those airlines overbook, and about 1 booking
// in 10 does not come, so the carriers here do both. On the tariff's
// fixed limits, with no cabin sold past its seats, the same world flies
// 0.68 full.
func TestFillSellsTheReportsFares(t *testing.T) {
	if raceEnabled {
		t.Skip("a calibration over a million bookings on one goroutine: CI runs it without the race detector")
	}
	m := usWorld(t)
	tf := tariff.FromManifest(m)
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	mem := rm.New()
	opts := Options{LoadFactor: 0.82, Day: day, Tariff: tf, Base: tf.BaseFare, RM: mem,
		Cabins: func(f world.Flight) map[string]int { return host.Cabins(f, 0) }}
	const learning = 30
	for d := 0; d < learning; d++ {
		dry := opts
		dry.Seed, dry.Dry = 3+int64(d)*7919, true
		plan, err := Day(context.Background(), m, dry, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d == 0 {
			// The first day has no record: fixed limits, and no cabin
			// sold past its seats.
			lf := float64(plan.Boarded) / float64(plan.Seats)
			t.Logf("on the fixed limits the world books %.3f of its seats and flies %.3f", float64(plan.Sold)/float64(plan.Seats), lf)
			if lf < 0.65 || lf > 0.71 {
				t.Errorf("on the fixed limits the world carries %.3f", lf)
			}
			if plan.Over != 0 || plan.Denied != 0 {
				t.Errorf("with no record the cabins were authorised %d over their seats and %d passengers were denied boarding", plan.Over, plan.Denied)
			}
		}
	}
	// The day measured is sold under the record and is not added to it,
	// so the dearer day below is sold under the same limits.
	opts.Seed, opts.Remembered = 3+learning*7919, true
	got := measure(t, m, tf, opts)
	// The report's load factor is of passengers carried, so the load to
	// hold against it is the passengers who come and have a seat.
	plan := got.plan
	lf := float64(plan.Boarded) / float64(got.seats)
	t.Logf("%d flights, %d seats, %d sold (%.3f), %d records", len(m.Flights), got.seats, got.sold, float64(got.sold)/float64(got.seats), got.records)
	t.Logf("%d fly (%.3f of the seats), %d do not come (%.3f of the bookings), %d are denied boarding (%.2f in 10,000), %d seats spoil",
		plan.Boarded, lf, plan.NoShows, float64(plan.NoShows)/float64(plan.Sold), plan.Denied, 1e4*float64(plan.Denied)/float64(plan.Boarded), plan.Spoiled)
	if plan.Sold != got.sold {
		t.Errorf("the plan counts %d seats sold and the records hold %d", plan.Sold, got.sold)
	}
	if lf < 0.80 || lf > 0.84 {
		t.Errorf("a world filled to 0.82 carries %.3f: move pool", lf)
	}
	// About 10 percent of the bookings held at departure do not come
	// (GAO-20-191, 2019).
	if away := float64(plan.NoShows) / float64(plan.Sold); away < 0.08 || away > 0.11 {
		t.Errorf("%.3f of the bookings do not come: the report has about 0.10", away)
	}
	// In 2018, of each 100,000 passengers who boarded, about 44 were
	// denied boarding, 43 of them by agreement (GAO-20-191). In 2012 it
	// was about 100. Nothing here was fitted to that: the carriers set
	// their limits from what a denied boarding and an empty seat cost.
	if denied := 1e4 * float64(plan.Denied) / float64(plan.Boarded); denied < 2 || denied > 10 {
		t.Errorf("%.2f passengers in 10,000 are denied boarding: the report has 4.4 in 2018 and 10 in 2012", denied)
	}
	if plan.Boarded+plan.Denied+plan.NoShows != plan.Sold {
		t.Errorf("%d booked, and %d flew, %d were denied and %d did not come", plan.Sold, plan.Boarded, plan.Denied, plan.NoShows)
	}
	if got.repriced != 0 {
		t.Errorf("%d travellers chose a fare the pricing would not sell them", got.repriced)
	}
	level := got.fare / got.market
	t.Logf("fare paid over the report's line: %.3f", level)
	if level < 0.97 || level > 1.03 {
		t.Errorf("travellers paid %.3f of the report's fare: move tariff's mixRatio by that much", level)
	}
	of := func(model equipment.Model) float64 { b := got.byModel[model]; return b[0] / b[1] }
	t.Logf("network %.3f, low-cost %.3f, ultra-low-cost %.3f of the line", of(equipment.Network), of(equipment.LowCost), of(equipment.UltraLowCost))
	if !(of(equipment.Network) > of(equipment.LowCost) && of(equipment.LowCost) > of(equipment.UltraLowCost)) {
		t.Error("a network carrier is paid most and an ultra-low-cost carrier least")
	}
	if low := of(equipment.UltraLowCost) / level; low < 0.70 || low > 0.85 {
		t.Errorf("the lowest-fare carriers are at %.3f of their markets; the report has 0.749", low)
	}
	share := func(classes ...string) float64 {
		n := 0
		for _, c := range classes {
			n += got.class[c]
		}
		return float64(n) / float64(got.sold)
	}
	var cs []string
	for c := range got.class {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	for _, c := range cs {
		t.Logf("  class %s %.3f", c, share(c))
	}
	// One traveller in 11 pays full fare, booking in the last days. More
	// than half fly on the deep discounts they bought weeks ahead.
	if y := share("Y"); y < 0.05 || y > 0.12 {
		t.Errorf("full fare is %.3f of travellers", y)
	}
	if deep := share("N", "S", "V", "L"); deep < 0.45 || deep > 0.65 {
		t.Errorf("the deep discounts are %.3f of travellers", deep)
	}
	pax := 0
	for _, n := range got.channel {
		pax += n
	}
	if direct := float64(got.channel["direct"]) / float64(pax); direct < 0.45 || direct > 0.62 {
		t.Errorf("direct sales are %.3f; US carriers report 0.55 to 0.65 with network carriers at the bottom of it", direct)
	}

	// A carrier that puts its fares up a quarter fills fewer seats: some
	// of its travellers will not pay. What it earns on the ones who stay
	// decides whether it was worth it, which is the pricing desk's
	// question and not this test's.
	var big string
	for c, b := range got.byCarrier {
		if equipment.ModelOf(c) == equipment.Network && b[1] > got.byCarrier[big][1] {
			big = c
		}
	}
	tf.SetMultiplier(big, 1.25)
	dear := measure(t, m, tf, opts)
	tf.SetMultiplier(big, 1)
	was, now := got.byCarrier[big], dear.byCarrier[big]
	drop := 1 - now[1]/was[1]
	t.Logf("%s at 1.25 times its fares: %.0f passengers to %.0f (%.1f%% fewer), revenue %.0f to %.0f", big, was[1], now[1], drop*100, was[0], now[0])
	if drop < 0.05 || drop > 0.40 {
		t.Errorf("a 25%% fare rise lost %s %.1f%% of its passengers", big, drop*100)
	}
	for c, b := range got.byCarrier {
		if c != big && dear.byCarrier[c] != b {
			t.Errorf("%s's sales moved when %s changed its fares, and the fill has no rival's share in it", c, big)
			break
		}
	}
}
