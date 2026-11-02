// Package noc is an airline's network operations centre: the desk that
// watches the day's signals -- weather forecast and in force, the Network
// Manager's regulations, crews near their limits, reserves left at each
// base, aircraft gone technical -- and decides how the schedule meets a
// disruption before it arrives. Its tools project what a disruption will
// cost in revenue and money: a scenario of uncertain events is played
// through the day plan and the crew roster many times, by Monte Carlo or
// Latin hypercube sampling, with own delays drawn from a recorded day
// when one is given, and the spread of outcomes is compared across the
// recovery policies the desk can choose -- absorb the delays, thin the
// schedule into the weather ahead of time, bring in standby crews.
// Sensitivity runs swing one uncertainty at a time to show which one the
// answer turns on.
//
// The model is the world's own: the same plan cuts the same duties, the
// same roster staffs them under the same agreements, and passengers are
// owed what the law covering their flight says. Only the numbers the
// world invents (a meal, a hotel, how many passengers take a refund) are
// the world's.
package noc

import (
	"fmt"
	"math"
	"strings"
)

// Dist is an uncertain number as its quantile function: a draw u in
// [0, 1) maps to a value, so the same machinery serves plain Monte Carlo,
// Latin hypercube sampling and the fixed quantiles of a sensitivity run.
type Dist struct {
	// Kind is "fixed" (A), "uniform" (A to B), "triangular" (A, mode B,
	// C), "normal" (mean A, deviation B) or "lognormal" (median A, the
	// log's deviation B).
	Kind string  `json:"kind"`
	A    float64 `json:"a"`
	B    float64 `json:"b,omitempty"`
	C    float64 `json:"c,omitempty"`
}

// Fixed is a certain number.
func Fixed(v float64) Dist { return Dist{Kind: "fixed", A: v} }

// Tri is a triangular distribution.
func Tri(lo, mode, hi float64) Dist { return Dist{Kind: "triangular", A: lo, B: mode, C: hi} }

// Normal is a normal distribution.
func Normal(mean, sd float64) Dist { return Dist{Kind: "normal", A: mean, B: sd} }

// Quantile is the value at u.
func (d Dist) Quantile(u float64) float64 {
	u = math.Min(math.Max(u, 1e-9), 1-1e-9)
	switch d.Kind {
	case "uniform":
		return d.A + u*(d.B-d.A)
	case "triangular":
		lo, mode, hi := d.A, d.B, d.C
		if hi <= lo {
			return lo
		}
		f := (mode - lo) / (hi - lo)
		if u < f {
			return lo + math.Sqrt(u*(hi-lo)*(mode-lo))
		}
		return hi - math.Sqrt((1-u)*(hi-lo)*(hi-mode))
	case "normal":
		return d.A + d.B*math.Sqrt2*math.Erfinv(2*u-1)
	case "lognormal":
		return d.A * math.Exp(d.B*math.Sqrt2*math.Erfinv(2*u-1))
	}
	return d.A
}

// Uncertain says the distribution has spread.
func (d Dist) Uncertain() bool {
	switch d.Kind {
	case "uniform":
		return d.B != d.A
	case "triangular":
		return d.C != d.A
	case "normal", "lognormal":
		return d.B != 0
	}
	return false
}

func (d Dist) String() string {
	switch d.Kind {
	case "uniform":
		return fmt.Sprintf("U(%.4g, %.4g)", d.A, d.B)
	case "triangular":
		return fmt.Sprintf("Tri(%.4g, %.4g, %.4g)", d.A, d.B, d.C)
	case "normal":
		return fmt.Sprintf("N(%.4g, %.4g)", d.A, d.B)
	case "lognormal":
		return fmt.Sprintf("LogN(%.4g, %.4g)", d.A, d.B)
	}
	return fmt.Sprintf("%.4g", d.A)
}

// Event kinds.
const (
	// KindWeather is a weather system over an airport: its arrival rate
	// falls to Factor of normal from Onset for Duration.
	KindWeather = "weather"
	// KindGroundStop holds the departures to an airport.
	KindGroundStop = "groundstop"
	// KindClosure closes an airport both ways.
	KindClosure = "closure"
	// KindSickout is a share of the carrier's crews calling in sick: Rate.
	KindSickout = "sickout"
	// KindAOG is a number of the carrier's aircraft going technical in
	// the window: Rate is the count.
	KindAOG = "aog"
)

// Event is one uncertain disruption.
type Event struct {
	Kind    string `json:"kind"`
	Name    string `json:"name,omitempty"`
	Airport string `json:"airport,omitempty"`
	// Probability is the chance the event happens at all; 0 means 1.
	Probability float64 `json:"probability,omitempty"`
	// Onset and Duration are minutes into the day and minutes long.
	Onset    Dist `json:"onset"`
	Duration Dist `json:"duration"`
	// Factor is a weather event's arrival rate as a share of normal.
	Factor Dist `json:"factor,omitempty"`
	// Rate is a sick-out's share or an AOG count.
	Rate Dist `json:"rate,omitempty"`
	// RadiusKM is a weather event's reach; 0 means the airport alone.
	RadiusKM float64 `json:"radius_km,omitempty"`
}

// dims is how many uniform draws an event takes: occurrence, onset,
// duration, and its size.
const dims = 4

// Scenario is a set of events.
type Scenario struct {
	Name   string  `json:"name"`
	Events []Event `json:"events"`
}

// Dims is the scenario's uncertain dimensions, one more for the draws
// that pick which flights an event takes.
func (s Scenario) Dims() int { return len(s.Events)*dims + 1 }

// Describe is the scenario in a line.
func (s Scenario) Describe() string {
	var parts []string
	for _, e := range s.Events {
		p := ""
		if e.Probability > 0 && e.Probability < 1 {
			p = fmt.Sprintf(" p=%.2f", e.Probability)
		}
		switch e.Kind {
		case KindWeather:
			parts = append(parts, fmt.Sprintf("weather %s from %s for %s at ×%s%s", e.Airport, clock(e.Onset.Quantile(0.5)), mins(e.Duration.Quantile(0.5)), trim(e.Factor.Quantile(0.5)), p))
		case KindGroundStop, KindClosure:
			parts = append(parts, fmt.Sprintf("%s %s from %s for %s%s", e.Kind, e.Airport, clock(e.Onset.Quantile(0.5)), mins(e.Duration.Quantile(0.5)), p))
		case KindSickout:
			parts = append(parts, fmt.Sprintf("sick-out %.1f%%%s", 100*e.Rate.Quantile(0.5), p))
		case KindAOG:
			parts = append(parts, fmt.Sprintf("%s aircraft technical%s", trim(e.Rate.Quantile(0.5)), p))
		}
	}
	return strings.Join(parts, "; ")
}

// clock is a time of the day; past midnight it says which day.
func clock(min float64) string {
	m := int(math.Round(min))
	s := fmt.Sprintf("%02d%02d", ((m/60)%24+24)%24, (m%60+60)%60)
	if m >= 24*60 {
		s += " the next day"
	}
	return s
}

func mins(m float64) string {
	h := int(m) / 60
	if h == 0 {
		return fmt.Sprintf("%dm", int(m))
	}
	return fmt.Sprintf("%dh%02d", h, int(m)%60)
}

func trim(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
}
