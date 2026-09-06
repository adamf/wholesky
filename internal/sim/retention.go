package sim

import (
	"context"
	"strconv"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
)

// Retention: what a machine forgets while the day runs.
//
// A region runs a couple of hundred carriers' reservation systems in one
// process, and each keeps every record and its history until the day
// wraps. The day is long: the machines were running out of memory hours
// before the wrap. A journey that has flown is finished business for the
// simulator (the real systems archive it; nothing here reads it again), so
// every few minutes each book drops the records whose last segment left
// more than a few hours ago, and each distribution system drops the
// availability beliefs whose flights have gone.

// flownAfter is how long after its last departure a record is kept, so a
// late departure and its movements are still on the book.
const flownAfter = 3 * time.Hour

// simNow is the world's clock as an instant on the booking day, UTC: the
// world keeps every time UTC, so a segment's date and time compare to it
// directly.
func (s *Sim) simNow() time.Time {
	return s.BookingDate.UTC().Truncate(24 * time.Hour).Add(time.Duration(s.clock.Pos(time.Now()) * float64(time.Minute)))
}

// departureOf is a segment's departure instant, UTC, or zero when the
// segment does not carry one.
func departureOf(sg pnr.Segment) time.Time {
	if sg.Depart.IsZero() {
		return time.Time{}
	}
	d := sg.Depart.UTC()
	if hm := sg.DepartTime; len(hm) == 4 {
		h, herr := strconv.Atoi(hm[:2])
		m, merr := strconv.Atoi(hm[2:])
		if herr == nil && merr == nil {
			d = d.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
		}
	}
	return d
}

// keepUntilFlown is the retention policy: a record stays while any of its
// segments is still to depart, or departed within flownAfter. A record
// with no dated segment stays; nothing about it says it is finished.
func keepUntilFlown(cutoff time.Time) func(*pnr.PNR) bool {
	return func(p *pnr.PNR) bool {
		if len(p.Segments) == 0 {
			return true
		}
		var latest time.Time
		for _, sg := range p.Segments {
			d := departureOf(sg)
			if d.IsZero() {
				return true
			}
			if d.After(latest) {
				latest = d
			}
		}
		return latest.After(cutoff)
	}
}

// retire runs the retention policy over every book on this machine and
// the availability caches, and reports what went.
func (s *Sim) retire(ctx context.Context) (records, beliefs int) {
	cutoff := s.simNow().Add(-flownAfter)
	keep := keepUntilFlown(cutoff)
	for _, t := range s.Tenants {
		if ctx.Err() != nil {
			return
		}
		if p, ok := t.Store.(store.Pruner); ok {
			if got, err := p.PruneRecords(ctx, keep); err == nil {
				records += got.Records
			}
		}
		if t.Gateway != nil && t.Gateway.Avail != nil {
			beliefs += t.Gateway.Avail.Purge()
		}
	}
	for _, g := range s.GDSes {
		if p, ok := g.Store.(store.Pruner); ok {
			if got, err := p.PruneRecords(ctx, keep); err == nil {
				records += got.Records
			}
		}
		if g.GW != nil && g.GW.Avail != nil {
			beliefs += g.GW.Avail.Purge()
		}
	}
	return records, beliefs
}

// retentionLoop retires on a timer for the life of the world.
func (s *Sim) retentionLoop(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			records, beliefs := s.retire(ctx)
			if records > 0 || beliefs > 0 {
				s.log.Info("retired flown records", "records", records, "beliefs", beliefs)
			}
		}
	}
}
