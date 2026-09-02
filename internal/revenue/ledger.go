// Package revenue keeps what the day's tickets were sold for, by the leg
// they fly: the money in the air is the sum over the legs airborne now,
// the money taken is the sum over everything sold. The ledger is fed where
// the price is known -- the distribution system that priced the sale, or
// the filler that wrote the pre-sold day -- and read by the globe.
package revenue

import (
	"strings"
	"sync"

	"github.com/adamf/jetway/pkg/pnr"
)

// Ledger is a per-leg total in minor units.
type Ledger struct {
	mu    sync.Mutex
	legs  map[string]int64
	total int64
}

// New returns an empty ledger.
func New() *Ledger { return &Ledger{legs: map[string]int64{}} }

// Key names a leg: the operating carrier, the flight number without
// leading zeros, and the boarding point, which is how the globe names an
// aircraft.
func Key(carrier, number, board string) string {
	return strings.ToUpper(carrier) + strings.TrimLeft(number, "0") + "/" + strings.ToUpper(board)
}

// Add credits a leg.
func (l *Ledger) Add(key string, cents int64) {
	if cents == 0 {
		return
	}
	l.mu.Lock()
	l.legs[key] += cents
	l.total += cents
	l.mu.Unlock()
}

// Record credits every air segment of a priced record with its share: each
// passenger's base for that segment plus an even share of their taxes.
// Segments sold under a marketing code are credited to the leg that flies
// them. A record with no price adds nothing.
func (l *Ledger) Record(r *pnr.PNR) {
	if r == nil || r.Pricing == nil {
		return
	}
	air := 0
	for _, s := range r.Segments {
		if s.Type == pnr.SegmentAir {
			air++
		}
	}
	if air == 0 {
		return
	}
	idx := 0
	for _, s := range r.Segments {
		if s.Type != pnr.SegmentAir {
			continue
		}
		carrier := s.Carrier
		if s.OperatingCarrier != "" {
			carrier = s.OperatingCarrier
		}
		var cents int64
		for _, pp := range r.Pricing.Passengers {
			if idx < len(pp.Segments) {
				cents += pp.Segments[idx] + pp.Taxes/int64(air)
			}
		}
		l.Add(Key(carrier, s.FlightNum, s.Board), cents)
		idx++
	}
}

// Sum is the total over the legs named.
func (l *Ledger) Sum(keys []string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	var n int64
	for _, k := range keys {
		n += l.legs[k]
	}
	return n
}

// Total is everything credited.
func (l *Ledger) Total() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}
