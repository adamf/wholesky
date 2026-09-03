package sim

// Irregular operations in the world: the distribution systems run jetway's
// irops engine against the schedule-change queue, with the manifest as the
// schedule it looks alternatives up in. When Heathrow closes and every
// flight touching it is cancelled by ASM, the bookings those cancellations
// queued are moved to the next flights over the same city pairs -- real
// sells and real cancels on the wire -- and the halos on the globe fade as
// they go. What nothing can carry stays on the queue, which is where a
// person would find it.

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/irops"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"

	"github.com/adamf/wholesky/internal/world"
)

// alternatives implements irops.Schedule from the manifest: flights over the
// dead segment's city pair departing after it, the carrier's own first, the
// same day and then the next, nearest first.
func (s *Sim) alternatives(ctx context.Context, dead pnr.Segment) ([]irops.Candidate, error) {
	sameNum := func(a, b string) bool { return strings.TrimLeft(a, "0") == strings.TrimLeft(b, "0") }
	// The dead flight's own departure, whether the passenger held it under
	// the operator's code or a marketing carrier's.
	deadDep := -1
	for _, f := range s.Flights[dead.Carrier] {
		if sameNum(f.Number, dead.FlightNum) {
			deadDep = f.DepMin
			break
		}
	}
	if deadDep < 0 {
		for _, fs := range s.Flights {
			for _, f := range fs {
				if f.Marketing == dead.Carrier && sameNum(f.MarketingNumber, dead.FlightNum) {
					deadDep = f.DepMin
				}
			}
		}
	}
	type cand struct {
		f       world.Flight
		carrier string // the code the seat is sold under
		number  string
		day     int // 0 same day, 1 next
		sortBy  int
	}
	var out []cand
	for code, fs := range s.Flights {
		for _, f := range fs {
			if f.From != dead.Board || f.To != dead.Off {
				continue
			}
			if f.Actual != nil && f.Actual.Cancelled {
				continue // the record says this one did not fly either
			}
			isDead := (code == dead.Carrier && sameNum(f.Number, dead.FlightNum)) ||
				(f.Marketing == dead.Carrier && sameNum(f.MarketingNumber, dead.FlightNum))
			if isDead {
				continue
			}
			// Own metal first, however much later it leaves: a carrier
			// protects its stranded passengers on its own flights and
			// interlines them only when it has nothing, because another
			// carrier's seat is another carrier's revenue. Its own code on a
			// partner's aircraft -- a codeshare it markets -- comes next,
			// sold under its own number so the booking stays its own.
			// Within each tier, the nearest departure.
			carrier, number, tier := code, f.Number, 2
			switch {
			case code == dead.Carrier:
				tier = 0
			case f.Marketing == dead.Carrier && f.MarketingNumber != "":
				carrier, number, tier = f.Marketing, f.MarketingNumber, 1
			}
			if f.DepMin > deadDep {
				out = append(out, cand{f: f, carrier: carrier, number: number, day: 0, sortBy: tier*10000 + f.DepMin})
			} else {
				out = append(out, cand{f: f, carrier: carrier, number: number, day: 1, sortBy: tier*10000 + 1440 + f.DepMin})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].sortBy < out[j].sortBy })
	if len(out) > 12 {
		out = out[:12]
	}
	res := make([]irops.Candidate, 0, len(out))
	for _, c := range out {
		dep := dead.Depart.AddDate(0, 0, c.day)
		res = append(res, irops.Candidate{
			Carrier: c.carrier, FlightNum: c.number, Board: c.f.From, Off: c.f.To,
			Depart:     dep,
			DepartTime: hhmm(c.f.DepMin), ArriveTime: hhmm(c.f.ArrMin),
		})
	}
	return res, nil
}

func hhmm(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return time.Date(2000, 1, 1, min/60, min%60, 0, 0, time.UTC).Format("1504")
}

// startIROPS runs the engine on one distribution system.
func (s *Sim) startIROPS(ctx context.Context, g *GDSNode, every time.Duration, log *slog.Logger) *irops.Engine {
	eng := &irops.Engine{
		Gateway: g.GW, Store: g.Store, Queues: g.GW.Queues,
		Schedule: irops.ScheduleFunc(s.alternatives),
		By:       "irops-" + strings.ToLower(g.Designator),
		Log:      log,
		// Full flights are the normal case on a filled day: ask the carriers
		// for what the cache does not offer on free sale, and give each
		// answer the time a real link takes.
		AskCarriers:  true,
		ReplyTimeout: 15 * time.Second,
		OnRebooked: func(ctx context.Context, item *store.QueueItem, out irops.Outcome) {
			s.Rebooked.Add(1)
			s.Stats.OnRebooked()
			s.Eye.OnRebooked(item.Reason)
		},
	}
	if every <= 0 {
		every = 15 * time.Second
	}
	go eng.Run(ctx, every)
	return eng
}
