package market

import (
	"math"
	"math/rand"
	"testing"
)

// Willingness to pay, against the table the PODS formula gives: of
// leisure travellers 42 in 100 will pay 1.25 times the lowest fare, 18
// will pay 1.5 times and 3 twice; of business travellers all will pay
// twice, 93 three times, 81 four times and half 7.5 times.
func TestWillingnessToPay(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const base = 20000
	n := map[Kind]int{}
	pay := map[Kind]map[float64]int{Leisure: {}, Business: {}}
	for i := 0; i < 200000; i++ {
		tr := Draw(rng, base, Mix{Business: 0.5, Leisure: [4]float64{1}, Corporate: [4]float64{1}})
		if tr.Cabin != "" {
			t.Fatal("a market with no premium share drew a front-cabin traveller")
		}
		n[tr.Kind]++
		for _, m := range []float64{1.25, 1.5, 2, 3, 4, 7.5} {
			if float64(tr.Most) >= m*base {
				pay[tr.Kind][m]++
			}
		}
	}
	for _, c := range []struct {
		k    Kind
		m    float64
		want float64
	}{{Leisure, 1.25, 0.42}, {Leisure, 1.5, 0.18}, {Leisure, 2, 0.03}, {Business, 2, 1.00}, {Business, 3, 0.93}, {Business, 4, 0.81}, {Business, 7.5, 0.50}} {
		got := float64(pay[c.k][c.m]) / float64(n[c.k])
		if math.Abs(got-c.want) > 0.012 {
			t.Errorf("%v travellers who will pay %.2f times the lowest fare: %.3f, the formula gives %.2f", c.k, c.m, got, c.want)
		}
	}
}

// The PODS example structure: Y 800, M 400, B 300, Q 200, with M wanting
// a Saturday night, B that and a change fee, Q those and no refund. The
// model's coefficients were tuned so that business travellers on average
// rank them Y, M, B, Q and leisure travellers Q, B, M, Y, with
// generalised costs of 800, 850, 900, 950 and 800, 750, 700, 650.
func TestRestrictionsSortTheTravellers(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	offers := []Offer{
		{Price: 80000, Refundable: true},
		{Price: 40000, Refundable: true, SaturdayNight: true},
		{Price: 30000, Refundable: true, SaturdayNight: true, ChangeFee: true},
		{Price: 20000, SaturdayNight: true, ChangeFee: true},
	}
	sum := map[Kind][4]float64{}
	n := map[Kind]float64{}
	for i := 0; i < 100000; i++ {
		tr := Draw(rng, 20000, Mix{Business: 0.5, Leisure: [4]float64{1}, Corporate: [4]float64{1}})
		// The traveller who does not want the Saturday night, with money
		// no object, at the hour they like, in their own channel.
		tr.SaturdayNight, tr.Most, tr.HourCost, tr.Channel = false, 1<<40, 0, ""
		s := sum[tr.Kind]
		for k, o := range offers {
			c, _ := tr.Cost(o)
			s[k] += float64(c) / 100
		}
		sum[tr.Kind] = s
		n[tr.Kind]++
	}
	want := map[Kind][4]float64{Business: {800, 850, 900, 950}, Leisure: {800, 750, 700, 650}}
	for k, s := range sum {
		for i := range s {
			if got := s[i] / n[k]; math.Abs(got-want[k][i]) > 6 {
				t.Errorf("%v: generalised cost of fare %d is %.0f, PODS gives %.0f", k, i+1, got, want[k][i])
			}
		}
	}
}

// Leisure travellers book early and business travellers late: half of
// business demand arrives inside two weeks and five in six leisure
// travellers have booked by then.
func TestBookingCurves(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	inside := map[Kind]int{}
	n := map[Kind]int{}
	for i := 0; i < 100000; i++ {
		tr := Draw(rng, 20000, MixFor("North America", false))
		n[tr.Kind]++
		if tr.Lead < 14 {
			inside[tr.Kind]++
		}
		if tr.Lead < 0 || tr.Lead > 160 {
			t.Fatalf("lead of %d days", tr.Lead)
		}
	}
	if b := float64(inside[Business]) / float64(n[Business]); b < 0.45 || b > 0.55 {
		t.Errorf("business travellers booking inside two weeks: %.2f", b)
	}
	if l := float64(inside[Leisure]) / float64(n[Leisure]); l < 0.13 || l > 0.21 {
		t.Errorf("leisure travellers booking inside two weeks: %.2f", l)
	}
	if share := float64(n[Business]) / float64(n[Business]+n[Leisure]); math.Abs(share-0.30) > 0.01 {
		t.Errorf("business share %.2f", share)
	}
}

// A traveller takes the offer that costs them least in all. A cut on one
// carrier wins leisure travellers from the other and few business
// travellers; the business traveller pays more for the nonstop at the
// right hour; and an offer over what a traveller will pay is not an
// offer.
func TestChoice(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	take := func(aPrice, bPrice int64) (leisureA, businessA float64) {
		var la, ln, ba, bn float64
		for i := 0; i < 40000; i++ {
			tr := Draw(rng, 20000, MixFor("Europe", false))
			tr.Channel = Direct
			offers := []Offer{
				{Carriers: []string{"AA"}, Price: aPrice, Depart: 9 * 60, Channel: Direct},
				{Carriers: []string{"BB"}, Price: bPrice, Depart: 15 * 60, Channel: Direct},
			}
			pick := tr.Choose(offers)
			if pick < 0 {
				continue
			}
			if tr.Kind == Leisure {
				ln++
				if pick == 0 {
					la++
				}
			} else {
				bn++
				if pick == 0 {
					ba++
				}
			}
		}
		return la / ln, ba / bn
	}
	evenL, evenB := take(22000, 22000)
	cutL, cutB := take(22000, 19800) // B cuts ten per cent
	if evenL < 0.35 || evenL > 0.65 {
		t.Errorf("at the same fare the two carriers split leisure travellers %.2f", evenL)
	}
	if evenL-cutL < 0.15 {
		t.Errorf("a ten per cent cut moved leisure travellers from %.2f to %.2f", evenL, cutL)
	}
	if evenB-cutB > evenL-cutL {
		t.Errorf("the cut moved business travellers (%.2f to %.2f) more than leisure travellers (%.2f to %.2f)", evenB, cutB, evenL, cutL)
	}

	b := Traveller{Kind: Business, Most: 200000, ConnectionCost: 12000, HourCost: 1600, Preferred: 8 * 60, Channel: TMC, SwitchCost: 2000, Loyal: "AA", LoyaltyValue: 3000}
	offers := []Offer{
		{Carriers: []string{"ZZ"}, Connections: 1, Price: 30000, Depart: 8 * 60, Refundable: true, Channel: TMC},
		{Carriers: []string{"AA"}, Price: 41000, Depart: 8*60 + 30, Refundable: true, Channel: TMC},
		{Carriers: []string{"AA"}, Price: 250000, Depart: 8 * 60, Refundable: true, Channel: TMC},
		{Carriers: []string{"YY"}, Price: 39500, Depart: 8*60 + 30, Refundable: true, Channel: Direct},
	}
	if pick := b.Choose(offers); pick != 1 {
		t.Errorf("the business traveller takes the nonstop on their own airline in their own channel: took %d", pick)
	}
	if c, ok := b.Cost(offers[2]); ok {
		t.Errorf("an offer over what the traveller will pay was costed at %d", c)
	}
	if none := (Traveller{Most: 100}).Choose(offers); none != -1 {
		t.Error("a traveller who can afford nothing bought something")
	}
	// A partner of the traveller's airline is worth half as much as the
	// airline itself.
	own, _ := b.Cost(Offer{Carriers: []string{"AA"}, Price: 40000, Depart: 8 * 60, Refundable: true, Channel: TMC})
	partner, _ := b.Cost(Offer{Carriers: []string{"BA"}, Partner: true, Price: 40000, Depart: 8 * 60, Refundable: true, Channel: TMC})
	other, _ := b.Cost(Offer{Carriers: []string{"ZZ"}, Price: 40000, Depart: 8 * 60, Refundable: true, Channel: TMC})
	if !(own < partner && partner < other) || other-partner != 1500 {
		t.Errorf("own airline %d, its partner %d, another %d", own, partner, other)
	}
}

// Who sits at the front: about 7 travellers in 100 on a short route and
// 13 on a long one, most of them on business, every one of them able to
// pay a business cabin's fare, and none of them sold a seat behind.
func TestFrontCabin(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	const base = 20000
	for _, c := range []struct {
		long     bool
		min, max float64
	}{{false, 0.055, 0.085}, {true, 0.115, 0.15}} {
		front, business, first := 0, 0, 0
		const n = 100000
		for i := 0; i < n; i++ {
			tr := Draw(rng, base, MixFor("Europe", c.long))
			if tr.Cabin == "" {
				if !tr.Sells("Y") || tr.Sells("J") {
					t.Fatal("an economy traveller is sold economy classes")
				}
				continue
			}
			front++
			if tr.Kind == Business {
				business++
			}
			if tr.Cabin == "F" {
				first++
				if !tr.Sells("F") || tr.Sells("J") {
					t.Fatal("a first-class traveller is sold first")
				}
			} else if !tr.Sells("D") || tr.Sells("Y") {
				t.Fatal("a business-class traveller is sold business")
			}
			if tr.Most < 4*base {
				t.Fatalf("a front-cabin traveller will pay only %d", tr.Most)
			}
		}
		if share := float64(front) / n; share < c.min || share > c.max {
			t.Errorf("long haul %v: %.3f of travellers at the front", c.long, share)
		}
		if float64(business)/float64(front) < 0.5 || first == 0 || first > front/5 {
			t.Errorf("long haul %v: %d at the front, %d on business, %d in first", c.long, front, business, first)
		}
	}
}
