package fuel

import "math"

// Kind is a kind of hedge.
type Kind string

const (
	// Swap fixes the price: the carrier is paid the index less the fixed
	// price on each gallon, which is a cost when the index falls below.
	Swap Kind = "swap"
	// Cap is a call option: the carrier is paid the index less the strike
	// when the index is above it, and has paid a premium for that.
	Cap Kind = "cap"
	// Collar is a cap paid for by selling a floor: paid above the cap's
	// strike, paying below the floor's.
	Collar Kind = "collar"
)

// Hedge is one position: gallons a day from one world day to another.
type Hedge struct {
	Kind Kind `json:"kind"`
	// Strike is the swap's fixed price or the cap's strike; Floor is the
	// collar's lower strike. Dollars per gallon.
	Strike float64 `json:"strike"`
	Floor  float64 `json:"floor,omitempty"`
	// Gallons is the volume hedged each day.
	Gallons float64 `json:"gallons_per_day"`
	From    int     `json:"from"`
	To      int     `json:"to"`
	// Premium is what was paid up front, in dollars.
	Premium float64 `json:"premium,omitempty"`
	// Bought is the world day the position was taken.
	Bought int `json:"bought"`
}

// Settle is what a hedge pays the carrier on a world day, in dollars:
// negative when the carrier pays. It settles against the same averaged
// index the carrier's fuel is priced on, so a swap for the gallons a
// carrier burns fixes what it pays.
func (h Hedge) Settle(m *Market, day int) float64 {
	if day < h.From || day > h.To {
		return 0
	}
	p := m.Index(day)
	switch h.Kind {
	case Swap:
		return (p - h.Strike) * h.Gallons
	case Cap:
		return math.Max(0, p-h.Strike) * h.Gallons
	case Collar:
		return (math.Max(0, p-h.Strike) - math.Max(0, h.Floor-p)) * h.Gallons
	}
	return 0
}

// Value is what the rest of a hedge is expected to pay from a world day
// on, in dollars: its mark to market. A swap bought before a fall is a
// liability, and a dealer asks for cash against one that grows.
func (h Hedge) Value(m *Market, day int) float64 {
	v := 0.0
	for d := max(day, h.From); d <= h.To; d += 7 {
		ahead := d - day
		f := m.Forward(day, ahead)
		sd := math.Sqrt(variance(float64(ahead) / 365))
		days := float64(min(7, h.To-d+1))
		switch h.Kind {
		case Swap:
			v += (f - h.Strike) * h.Gallons * days
		case Cap:
			v += black(f, h.Strike, sd) * h.Gallons * days
		case Collar:
			v += (black(f, h.Strike, sd) - (black(f, h.Floor, sd) - (f - h.Floor))) * h.Gallons * days
		}
	}
	return v
}

// Quote is the terms a dealer offers on a world day for a hedge that
// runs from `start` days ahead for `days` days. A swap's price is the
// average forward plus a dealer's margin of one and a half per cent. A
// cap is struck at strikeOver times the forward (1.1 is ten per cent out
// of the money) and costs its Black-76 value with the market's own
// volatility and a tenth more for the dealer. A collar's floor is set so
// the floor sold pays for the cap bought.
func (m *Market) Quote(kind Kind, day, start, days int, gallons, strikeOver float64) Hedge {
	h := Hedge{Kind: kind, Gallons: gallons, From: day + start, To: day + start + days - 1, Bought: day}
	sum, n := 0.0, 0
	for d := start; d < start+days; d += 7 {
		sum += m.Forward(day, d)
		n++
	}
	fwd := sum / float64(n)
	if strikeOver <= 0 {
		strikeOver = 1
	}
	switch kind {
	case Swap:
		h.Strike = fwd * 1.015
	case Cap:
		h.Strike = fwd * strikeOver
		h.Premium = h.Value(m, day) * 1.10
	case Collar:
		h.Strike = fwd * strikeOver
		// Lower the floor until what it would cost the carrier matches
		// what the cap is worth.
		capOnly := Hedge{Kind: Cap, Strike: h.Strike, Gallons: gallons, From: h.From, To: h.To}
		want := capOnly.Value(m, day)
		lo, hi := fwd*0.3, fwd
		for i := 0; i < 40; i++ {
			h.Floor = (lo + hi) / 2
			// The floor's cost to the carrier is the cap's value less the
			// collar's.
			if cost := want - h.Value(m, day); cost > want {
				hi = h.Floor
			} else {
				lo = h.Floor
			}
		}
	}
	return h
}

// black is the Black-76 value of a call on a forward f at strike k, with
// sd the standard deviation of the log price at expiry, undiscounted.
func black(f, k, sd float64) float64 {
	if sd <= 0 || k <= 0 {
		return math.Max(0, f-k)
	}
	d1 := (math.Log(f/k) + sd*sd/2) / sd
	return f*phi(d1) - k*phi(d1-sd)
}

func phi(x float64) float64 { return 0.5 * (1 + math.Erf(x/math.Sqrt2)) }
