package rights

import "testing"

func eur(v float64) int64 { return int64(v*EURinUSD*100 + 0.5) }

// Article 7(1) by band, Article 5(3) for weather, Article 7(2) halving.
func TestEU261(t *testing.T) {
	cases := []struct {
		name string
		d    Disruption
		comp int64
	}{
		{"short cancellation, crew", Disruption{Regime: EU261, KM: 900, Cause: Crew, Cancelled: true}, eur(250)},
		{"medium, three hours late, technical", Disruption{Regime: EU261, KM: 2000, Cause: Technical, ArrDelay: 185}, eur(400)},
		{"long-haul, four hours late", Disruption{Regime: EU261, KM: 6000, Cause: Carrier, ArrDelay: 240}, eur(600)},
		{"intra-EU beyond 3500 stays in the middle band", Disruption{Regime: EU261, KM: 3700, Intra: true, Cause: Crew, Cancelled: true}, eur(400)},
		{"two hours fifty-nine late is not compensated", Disruption{Regime: EU261, KM: 900, Cause: Crew, ArrDelay: 179}, 0},
		{"weather is extraordinary", Disruption{Regime: EU261, KM: 900, Cause: Weather, Cancelled: true}, 0},
		{"ATC is extraordinary", Disruption{Regime: EU261, KM: 6000, Cause: ATC, ArrDelay: 400}, 0},
		{"rerouted, arriving within two hours: nothing", Disruption{Regime: EU261, KM: 900, Cause: Crew, Cancelled: true, Rerouted: true, ArrDelay: 90}, 0},
		{"rerouted, two hours late: half", Disruption{Regime: EU261, KM: 900, Cause: Crew, Cancelled: true, Rerouted: true, ArrDelay: 120}, eur(250) / 2},
		{"rerouted long-haul, five hours late: full", Disruption{Regime: EU261, KM: 6000, Cause: Crew, Cancelled: true, Rerouted: true, ArrDelay: 300}, eur(600)},
		{"UK in pounds", Disruption{Regime: UK261, KM: 900, Cause: Crew, Cancelled: true}, 27940},
		{"US pays no compensation", Disruption{Regime: US, KM: 900, Cause: Crew, Cancelled: true}, 0},
	}
	for _, c := range cases {
		if got := For(c.d).Compensation; got != c.comp {
			t.Errorf("%s: %d, want %d", c.name, got, c.comp)
		}
	}
}

// Article 6(1) and 9: care after two, three or four hours, whatever the
// cause; a hotel for a night.
func TestCare(t *testing.T) {
	if o := For(Disruption{Regime: EU261, KM: 900, Cause: Weather, DepDelay: 120}); o.Care != mealCents {
		t.Errorf("weather still owes care: %+v", o)
	}
	if o := For(Disruption{Regime: EU261, KM: 2000, Cause: Weather, DepDelay: 150}); o.Care != 0 {
		t.Errorf("a medium flight's care starts at three hours: %+v", o)
	}
	if o := For(Disruption{Regime: EU261, KM: 900, Cause: Weather, Cancelled: true, Overnight: true}); o.Care != mealCents+hotelCents {
		t.Errorf("overnight: %+v", o)
	}
	// US: the carriers' commitments cover only what they caused.
	if o := For(Disruption{Regime: US, KM: 900, Cause: Weather, Cancelled: true, Overnight: true}); o.Care != 0 {
		t.Errorf("US weather: %+v", o)
	}
	if o := For(Disruption{Regime: US, KM: 900, Cause: Crew, Cancelled: true, Overnight: true}); o.Care != mealCents+hotelCents {
		t.Errorf("US crew: %+v", o)
	}
}

// 14 CFR 260.2: a significant delay is three hours domestic, six
// international.
func TestUSRefundThresholds(t *testing.T) {
	if For(Disruption{Regime: US, Domestic: true, ArrDelay: 179}).RefundRight || !For(Disruption{Regime: US, Domestic: true, ArrDelay: 180}).RefundRight {
		t.Error("domestic threshold")
	}
	if For(Disruption{Regime: US, ArrDelay: 359}).RefundRight || !For(Disruption{Regime: US, ArrDelay: 360}).RefundRight {
		t.Error("international threshold")
	}
}

// Article 3: departing a member state, or arriving in one on its carrier.
func TestRegime(t *testing.T) {
	for _, c := range []struct {
		from, to, carrier string
		want              Regime
	}{
		{"Germany", "United States", "United States", EU261},
		{"United States", "Germany", "United States", US},
		{"United States", "Germany", "Germany", EU261},
		{"United Kingdom", "Spain", "Ireland", UK261},
		{"Japan", "Australia", "Japan", None},
		{"Turkey", "Switzerland", "Switzerland", EU261},
	} {
		if got := Of(c.from, c.to, c.carrier); got != c.want {
			t.Errorf("%s-%s on a %s carrier: %s, want %s", c.from, c.to, c.carrier, got, c.want)
		}
	}
}

// 14 CFR 250.5, worked by hand. A fare of $180 one way on a domestic
// flight: nothing when the alternative arrives 45 minutes late, twice
// the fare ($360) at 90 minutes, 4 times ($720) at 2 hours 30. A fare of
// $700 meets the limits: $1,075 and $2,150. On a flight to another
// country the first tier runs to 4 hours.
func TestDeniedBoardingUnitedStates(t *testing.T) {
	us := func(fare int64, wait int, intra bool) Bumped {
		return Bumped{Regime: US, KM: 1000, Intra: intra, Outbound: true, Fare: fare, Wait: wait}
	}
	for _, c := range []struct {
		name string
		b    Bumped
		want int64
	}{
		{"45 minutes", us(18000, 45, true), 0},
		{"1 hour exactly", us(18000, 60, true), 0},
		{"90 minutes", us(18000, 90, true), 36000},
		{"2 hours exactly", us(18000, 120, true), 72000},
		{"2 hours 30", us(18000, 150, true), 72000},
		{"$700 at 90 minutes", us(70000, 90, true), 107500},
		{"$700 at 5 hours", us(70000, 300, true), 215000},
		{"abroad at 3 hours 20", us(18000, 200, false), 36000},
		{"abroad at 5 hours", us(18000, 300, false), 72000},
	} {
		if got := DeniedBoarding(c.b).Compensation; got != c.want {
			t.Errorf("%s: %d cents, want %d", c.name, got, c.want)
		}
	}
	// A smaller aircraft, or a flight that leaves from a foreign airport,
	// owes no compensation. Care is still owed for a long wait.
	smaller := us(18000, 300, true)
	smaller.Smaller = true
	if o := DeniedBoarding(smaller); o.Compensation != 0 || o.Care != mealCents {
		t.Errorf("a smaller aircraft: %+v; want no compensation and a meal", o)
	}
	inbound := us(18000, 300, false)
	inbound.Outbound = false
	if o := DeniedBoarding(inbound); o.Compensation != 0 {
		t.Errorf("a flight from a foreign airport: %+v; want no compensation", o)
	}
	night := us(18000, 600, true)
	night.Overnight = true
	if o := DeniedBoarding(night); o.Compensation != 72000 || o.Care != mealCents+hotelCents || !o.RefundRight {
		t.Errorf("overnight: %+v; want $720, a meal and a hotel", o)
	}
}

// Regulation 261, Article 4(3) with Article 7. A flight of 1,000 km:
// 250 euros, which is $270.00, or half ($135.00) when the alternative
// arrives within 2 hours. A flight of 6,000 km out of the Union: 600
// euros ($648.00), or half within 4 hours. The cause does not matter, so
// a smaller aircraft owes the same.
func TestDeniedBoardingEurope(t *testing.T) {
	for _, c := range []struct {
		name string
		b    Bumped
		want int64
	}{
		{"1,000 km, 5 hours", Bumped{Regime: EU261, KM: 1000, Intra: true, Outbound: true, Wait: 300}, 27000},
		{"1,000 km, 100 minutes", Bumped{Regime: EU261, KM: 1000, Intra: true, Outbound: true, Wait: 100}, 13500},
		{"6,000 km, 5 hours", Bumped{Regime: EU261, KM: 6000, Outbound: true, Wait: 300}, 64800},
		{"6,000 km, 3 hours 30", Bumped{Regime: EU261, KM: 6000, Outbound: true, Wait: 210}, 32400},
		{"a smaller aircraft", Bumped{Regime: EU261, KM: 1000, Intra: true, Outbound: true, Wait: 300, Smaller: true}, 27000},
	} {
		o := DeniedBoarding(c.b)
		if o.Compensation != c.want || o.Care != mealCents {
			t.Errorf("%s: %+v, want %d cents and a meal", c.name, o, c.want)
		}
	}
	if o := DeniedBoarding(Bumped{Regime: None, KM: 1000, Fare: 18000, Wait: 300}); o.Compensation != 0 || o.Care != 0 {
		t.Errorf("under no regime: %+v", o)
	}
}

// A carrier offers a volunteer twice the fare, to the first tier's limit.
func TestOffer(t *testing.T) {
	if got := Offer(18000); got != 36000 {
		t.Errorf("a $180 fare: %d cents, want 36000", got)
	}
	if got := Offer(70000); got != 107500 {
		t.Errorf("a $700 fare: %d cents, want 107500", got)
	}
}
