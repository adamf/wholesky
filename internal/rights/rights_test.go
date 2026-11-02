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
