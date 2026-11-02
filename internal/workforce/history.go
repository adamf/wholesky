package workforce

import (
	"time"

	"github.com/adamf/jetway/pkg/crew"
)

// ringDays is how many past days a member's record keeps by the day: the
// 672 hours of the longest look-back but one, which is 28 days.
const ringDays = 28

// period kinds in the ring.
const (
	kindNone uint8 = iota
	kindFlight
	kindReserve
)

// dayRec is one past day of a member's record: the duty's report and its
// length in minutes from that day's midnight, the block flown, and the
// kind. A day holds at most one duty, which is how a line is built.
type dayRec struct {
	report, length, flight uint16
	kind                   uint8
}

// history is a member's record in a fixed size: the last 28 days by day,
// and the block time of the 336 days before them carried as one number.
// The sky has a few hundred thousand crew members, and a list of every
// duty for a year each would not fit beside the rest of the world.
type history struct {
	days [ringDays]dayRec // days[i] is i+1 days before the ops day
	// year is the block minutes flown more than 28 days ago and inside 365
	// calendar days. The ring cannot say when they were flown, so a day
	// the window leaves takes away one 336th of the number: an
	// approximation that errs neither way on average.
	year uint32
}

// view is a member's history seen from an ops day: the crew.History
// jetway's legality checks read.
type view struct {
	h   *history
	day time.Time // the ops day's midnight
}

func (v view) span(i int) (from, to time.Time, rec dayRec) {
	rec = v.h.days[i]
	midnight := v.day.AddDate(0, 0, -(i + 1))
	from = midnight.Add(time.Duration(rec.report) * time.Minute)
	return from, from.Add(time.Duration(rec.length) * time.Minute), rec
}

func overlapMin(a, b, from, to time.Time) time.Duration {
	if a.Before(from) {
		a = from
	}
	if b.After(to) {
		b = to
	}
	if !b.After(a) {
		return 0
	}
	return b.Sub(a)
}

// FlightTime implements crew.History. A day's block is counted in full
// when its duty overlaps the window at all: the ring does not keep the
// legs' times, and a check that counts too much is the safe one.
func (v view) FlightTime(from, to time.Time) time.Duration {
	var sum time.Duration
	for i := range v.h.days {
		a, b, rec := v.span(i)
		if rec.kind == kindNone || rec.flight == 0 {
			continue
		}
		if overlapMin(a, b, from, to) > 0 {
			sum += time.Duration(rec.flight) * time.Minute
		}
	}
	// The carried year counts when the window reaches back past the ring.
	if v.h.year > 0 && from.Before(v.day.AddDate(0, 0, -ringDays)) {
		sum += time.Duration(v.h.year) * time.Minute
	}
	return sum
}

// DutyTime implements crew.History: flight duty periods only.
func (v view) DutyTime(from, to time.Time) time.Duration {
	var sum time.Duration
	for i := range v.h.days {
		a, b, rec := v.span(i)
		if rec.kind == kindFlight {
			sum += overlapMin(a, b, from, to)
		}
	}
	return sum
}

// LastDuty implements crew.History.
func (v view) LastDuty(before time.Time) (report, release time.Time, ok bool) {
	for i := range v.h.days {
		a, b, rec := v.span(i)
		if rec.kind == kindNone || b.After(before) {
			continue
		}
		if !ok || b.After(release) {
			report, release, ok = a, b, true
		}
	}
	return report, release, ok
}

// LongestFree implements crew.History.
func (v view) LongestFree(from, to time.Time) time.Duration {
	var ps []crew.Period
	for i := range v.h.days {
		a, b, rec := v.span(i)
		if rec.kind != kindNone {
			ps = append(ps, crew.Period{Report: a, Release: b})
		}
	}
	return crew.LongestFree(ps, from, to)
}

// roll files the ops day's duty and moves the record a day on.
func (h *history) roll(today dayRec) {
	old := h.days[ringDays-1]
	if old.kind == kindFlight {
		h.year += uint32(old.flight)
	}
	h.year -= h.year / 336
	copy(h.days[1:], h.days[:ringDays-1])
	h.days[0] = today
}

// pattern is a member's line: runs of days on and days off. Every line
// is four on and three off, and the members of a pool start the cycle on
// different days, spread evenly, so each day of the week has the same
// share of the pool working: lines are bid to cover the trips, and a
// pool whose coverage swung by the dice would cancel flights for no
// reason any airline would have.
type pattern struct {
	on, off, phase uint8
}

// linePattern is the line of the n-th member of a pool.
func linePattern(n int) pattern { return pattern{on: 4, off: 3, phase: uint8(n % 7)} }

// On says the member works the day, day n of the run.
func (p pattern) On(n int) bool {
	l := int(p.on + p.off)
	i := ((n+int(p.phase))%l + l) % l
	return i < int(p.on)
}

// synthesise fills a member's past: each on day of the line in the last
// 28 a duty drawn from the seed, the year before it carried. Intensity
// sets how hard the member has flown, so some are near their limits.
func synthesise(seed string, p pattern, dayN int, reserve bool) history {
	var h history
	intensity := 0.7 + 0.6*draw(seed, "intensity")
	var month uint32
	for i := 0; i < ringDays; i++ {
		n := dayN - (i + 1)
		if !p.On(n) {
			continue
		}
		d := itoaN(n)
		report := 300 + int(draw(seed, d, "report")*540) // 05:00 to 14:00
		length := 360 + int(draw(seed, d, "length")*300) // 6 to 11 hours
		frac := 0.45 + 0.2*draw(seed, d, "flight")       // share of the duty in the air
		if i == 0 && report+length > 21*60 {
			// The line's last day ends by 21:00, so the line is workable
			// the next morning; a late finish is the day's own business.
			length = 21*60 - report
		}
		rec := dayRec{report: uint16(report), length: uint16(length), kind: kindFlight}
		rec.flight = uint16(float64(length) * frac * intensity)
		if reserve && draw(seed, d, "called") < 0.7 {
			// Most reserve days pass on call without a trip.
			rec = dayRec{report: 300, length: 840, kind: kindReserve}
		}
		h.days[i] = rec
		if rec.kind == kindFlight {
			month += uint32(rec.flight)
		}
	}
	year := uint32((550 + 300*draw(seed, "year")) * 60)
	if limit := uint32(990 * 60); year+month > limit {
		year = limit - month
	}
	h.year = year
	return h
}

func itoaN(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
