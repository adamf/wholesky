// Package fuel is the world's jet fuel market: what a gallon costs at a
// station on a day, what a flight burns, and what a hedge is worth.
//
// The price is real for as long as the record runs. The reference is the
// US Gulf Coast spot price the Energy Information Administration has
// published for every trading day since April 1990 (internal/refdata),
// which follows IATA's world average within about three dollars a barrel.
// A world whose calendar starts inside the record lives the record: start
// it in January 2008 and fuel goes from 2.75 dollars a gallon to 4.04 by
// July, 4.81 in September, and 1.26 by Christmas.
// Past the record's end the price is continued by a process fitted to
// the record: the logarithm of the price reverts to a long-run level and
// takes a random step and, some days, a jump. The fit is in this
// package's tests, which estimate the parameters again from the embedded
// series. The continuation is drawn from the world's seed and the date,
// so every machine of a world sees the same price.
//
// A station does not pay the reference. It pays a regional differential
// (Europe above the Gulf Coast, Asia and the Middle East below), an
// into-plane charge that falls with the station's size, and an index that
// is averaged over about two months, which is why an airline's fuel bill
// lags a spike. The differentials outside the United States rest on three
// of IATA's weekly snapshots and are estimates.
package fuel

import (
	"hash/fnv"
	"math"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/refdata"
)

// KgPerGallon is the mass of a US gallon of jet fuel.
const KgPerGallon = 3.04

// The continuation, fitted to the Gulf Coast series from 2005 to 2026:
// d ln P = Reversion (ln LongRun - ln P) dt + Volatility dW + jumps.
// The half-life of a deviation is ln 2 / Reversion, about ten months.
const (
	// Reversion is the speed of mean reversion per year. The fit gives
	// 0.81 (2005-2026) and 0.86 (2005-2019).
	Reversion = 0.81
	// LongRun is the level the price reverts to, in dollars per gallon.
	// The fit gives 2.27.
	LongRun = 2.27
	// Volatility is the annual volatility of the daily step without its
	// jumps: 36 per cent once the days more than three standard
	// deviations out are set aside.
	Volatility = 0.36
	// JumpsPerYear and JumpSize are those days: about four a year, up as
	// often as down, large enough that steps and jumps together give the
	// record's whole volatility of 45 per cent.
	JumpsPerYear = 4.0
	JumpSize     = 0.135
	tradingDays  = 252.0
)

// Market is a world's fuel market.
type Market struct {
	// Start is the calendar date of the world's day 0.
	Start time.Time
	// Seed makes the continuation the world's own.
	Seed int64

	mu     sync.Mutex
	path   []float64 // continuation, one value per calendar day after the record
	last   time.Time // last day of the record
	shocks []Shock
}

// Shock is an event added to the market: the price steps by Step (in log
// terms: 0.4 is about fifty per cent up) on Day, and the step fades with
// the half-life given, in days. A refinery fire, a closed strait.
type Shock struct {
	Day      int     `json:"day"`
	Step     float64 `json:"step"`
	HalfLife float64 `json:"half_life"`
	Name     string  `json:"name,omitempty"`
}

// New is a market whose day 0 is start.
func New(start time.Time, seed int64) *Market {
	s := refdata.JetFuel()
	return &Market{Start: dateOnly(start), Seed: seed, last: s[len(s)-1].Date}
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Date is the calendar date of a world day.
func (m *Market) Date(day int) time.Time { return m.Start.AddDate(0, 0, day) }

// Recorded reports whether a world day's price is the record's and not
// the continuation's.
func (m *Market) Recorded(day int) bool { return !m.Date(day).After(m.last) }

// AddShock adds an event to the market.
func (m *Market) AddShock(s Shock) {
	m.mu.Lock()
	m.shocks = append(m.shocks, s)
	m.mu.Unlock()
}

// Shocks is the events added so far.
func (m *Market) Shocks() []Shock {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Shock(nil), m.shocks...)
}

// Reference is the Gulf Coast spot price on a world day, in dollars per
// gallon: the record's while it lasts, the continuation's after, with
// any shocks on top.
func (m *Market) Reference(day int) float64 {
	d := m.Date(day)
	var p float64
	if v, ok := refdata.JetFuelOn(d); ok && !d.After(m.last) {
		p = v
	} else if d.Before(m.last) {
		// Before the record begins: its first price.
		p = refdata.JetFuel()[0].Value
	} else {
		p = m.continued(int(d.Sub(m.last).Hours() / 24))
	}
	m.mu.Lock()
	for _, s := range m.shocks {
		if day < s.Day {
			continue
		}
		fade := 1.0
		if s.HalfLife > 0 {
			fade = math.Pow(0.5, float64(day-s.Day)/s.HalfLife)
		}
		p *= math.Exp(s.Step * fade)
	}
	m.mu.Unlock()
	return p
}

// continued is the price n calendar days after the record ends. Each
// weekday takes one step of the process; a weekend holds Friday's price.
func (m *Market) continued(n int) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.path) == 0 {
		s := refdata.JetFuel()
		m.path = []float64{s[len(s)-1].Value}
	}
	dt := 1 / tradingDays
	for len(m.path) <= n {
		k := len(m.path)
		prev := m.path[k-1]
		date := m.last.AddDate(0, 0, k)
		if wd := date.Weekday(); wd == time.Saturday || wd == time.Sunday {
			m.path = append(m.path, prev)
			continue
		}
		x := math.Log(prev)
		x += Reversion*(math.Log(LongRun)-x)*dt + Volatility*math.Sqrt(dt)*m.normal(k, "step")
		if m.uniform(k, "jump?") < JumpsPerYear*dt {
			x += JumpSize * m.normal(k, "jump")
		}
		m.path = append(m.path, math.Exp(x))
	}
	return m.path[n]
}

func (m *Market) uniform(k int, what string) float64 {
	h := fnv.New64a()
	var b [16]byte
	for i := 0; i < 8; i++ {
		b[i] = byte(m.Seed >> (8 * i))
		b[8+i] = byte(k >> (8 * i))
	}
	h.Write(b[:])
	h.Write([]byte(what))
	v := h.Sum64()
	// A second round: FNV alone leaves the low bits of nearby keys close.
	v ^= v >> 33
	v *= 0xff51afd7ed558ccd
	v ^= v >> 33
	v *= 0xc4ceb9fe1a85ec53
	v ^= v >> 33
	return (float64(v>>11) + 0.5) / float64(1<<53)
}

func (m *Market) normal(k int, what string) float64 {
	return math.Sqrt2 * math.Erfinv(2*m.uniform(k, what)-1)
}

// basis is a region's differential to the Gulf Coast in cents per gallon,
// in calm and in crisis. North West Europe, Asia, the Middle East and
// Latin America are IATA's regional indices against the Gulf Coast in the
// weeks of 30 January, 21 August and 18 September 2026; North America is
// the New York and Los Angeles differentials weighted with the Gulf
// Coast's own zero; Africa and Oceania are estimates.
var basis = map[string][2]float64{
	refdata.NorthAmerica: {5, 9},
	refdata.Europe:       {10, 24},
	refdata.Asia:         {-12, -21},
	refdata.MiddleEast:   {-22, -31},
	refdata.LatinAmerica: {16, 19},
	refdata.Africa:       {8, 20},
	refdata.Oceania:      {-4, -8},
}

// Basis is a region's differential to the reference on a world day, in
// dollars per gallon. Differentials widen in a crisis: the further the
// reference is above its long-run level, the nearer the crisis value.
func (m *Market) Basis(day int, region string) float64 {
	b, ok := basis[region]
	if !ok {
		return 0
	}
	stress := (m.Reference(day)/LongRun - 1.3) / 0.7
	stress = math.Max(0, math.Min(1, stress))
	return (b[0] + (b[1]-b[0])*stress) / 100
}

// IntoPlane is what a station adds to its hub's price to put the fuel in
// the wing, in dollars per gallon: pipeline or truck, storage, the
// fueller's fee. It falls with the station's size, counted in departures
// a day. US carriers as a whole paid 12 to 22 cents over the Gulf Coast
// spot price in 2019 to 2025; the steps by size are estimates around
// that.
func IntoPlane(departuresPerDay int) float64 {
	switch {
	case departuresPerDay >= 400:
		return 0.08
	case departuresPerDay >= 150:
		return 0.12
	case departuresPerDay >= 40:
		return 0.20
	case departuresPerDay >= 10:
		return 0.33
	}
	return 0.60
}

// Paid is what an airline pays for a gallon at a station on a world day:
// the index averaged over the sixty days to that day, which is how a
// supply contract that takes the mean of this month and last behaves,
// with the region's differential and the station's into-plane charge.
// In March 2026 the spot price rose 63 per cent and the price US
// carriers paid rose 33.
func (m *Market) Paid(day int, region string, departuresPerDay int) float64 {
	return m.Index(day) + m.Basis(day, region) + IntoPlane(departuresPerDay)
}

// Index is the reference averaged over the sixty days to a world day.
func (m *Market) Index(day int) float64 {
	sum := 0.0
	for k := 0; k < 60; k += 3 {
		sum += m.Reference(day - k)
	}
	return sum / 20
}

// Forward is the price the market expects for a world day `ahead` days
// after `day`: the process's own expectation, which pulls the price
// toward the long-run level at the speed of reversion. After a spike the
// curve slopes down, so a swap looks cheap against spot and still locks
// in a high price.
func (m *Market) Forward(day, ahead int) float64 {
	t := float64(ahead) / 365
	x0 := math.Log(m.Reference(day))
	mu := math.Log(LongRun)
	mean := mu + (x0-mu)*math.Exp(-Reversion*t)
	return math.Exp(mean + variance(t)/2)
}

// variance is the variance of the log price t years ahead.
func variance(t float64) float64 {
	s2 := Volatility*Volatility + JumpsPerYear*JumpSize*JumpSize
	return s2 * (1 - math.Exp(-2*Reversion*t)) / (2 * Reversion)
}

// Burn is the fuel a flight of km great-circle kilometres burns, in
// kilograms: the landing and take-off cycle plus the type's line through
// EUROCONTROL's climb, cruise and descent figures, over a route five per
// cent longer than the great circle. An unknown type burns by its seats.
func Burn(equipment string, seats, km int) float64 {
	a, ok := refdata.AircraftType(equipment)
	if !ok {
		// 22 grams a seat-kilometre and 4 kg a seat for the cycle: the
		// middle of the table.
		return float64(seats)*4 + 0.022*float64(seats)*float64(km)*1.05
	}
	kg := a.LTOFuel + a.BurnFixed + a.BurnPerKM*float64(km)*1.05
	if seats > 0 && a.Seats > 0 && seats != a.Seats {
		// A denser cabin is heavier: a tenth of the difference in seats.
		kg *= 1 + 0.1*(float64(seats)/float64(a.Seats)-1)
	}
	return kg
}

// Gallons converts kilograms of fuel to US gallons.
func Gallons(kg float64) float64 { return kg / KgPerGallon }
