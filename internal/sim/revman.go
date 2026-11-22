package sim

import (
	"context"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/fill"
	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/world"
)

// The carriers' revenue management memory.
//
// Each carrier keeps a record of every flight's booking window, departure
// by departure (internal/rm). The fill writes a day's windows to it and
// sells the next day's flights under the limits it gives. The day's own
// forecaster reads the booking curve on what a flight's record says the
// flight draws, in place of the tariff's fixed mix. The records are in a
// file beside the state file, written after each day is filled, so a
// world that starts again knows what its flights have taught it.
//
// Each record also holds who came. When a flight's door closes, departure
// control counts each cabin: the bookings it held and the passengers who
// stayed away. From that the carrier sets how many bookings the main
// cabin may hold the next time it is sold, at or over its seats. The
// day's own selling holds the cabin to the limit its fill set.

// initRevenueMemory makes the memory and reads what an earlier run kept.
func (s *Sim) initRevenueMemory(opts Options) {
	s.rmBase.Store(&sync.Map{})
	if opts.Fill <= 0 || opts.NoRevenueMemory {
		return
	}
	s.rm, s.burn = rm.New(), opts.BurnIn
	if opts.StateFile == "" {
		return
	}
	s.rmPath = opts.StateFile + ".rm"
	if err := s.rm.Load(s.rmPath); err != nil {
		s.log.Warn("the carriers' revenue management records were not read; they start again", "file", s.rmPath, "err", err)
		s.rm = rm.New()
		return
	}
	if n := s.rm.Len(); n > 0 {
		s.log.Info("revenue management records read", "cabins", n, "file", s.rmPath)
	}
}

// saveRevenueMemory writes the records to their file.
func (s *Sim) saveRevenueMemory() {
	if s.rm == nil || s.rmPath == "" {
		return
	}
	started := time.Now()
	if err := s.rm.Save(s.rmPath); err != nil {
		s.log.Warn("the carriers' revenue management records were not saved", "file", s.rmPath, "err", err)
		return
	}
	s.log.Debug("revenue management records saved", "cabins", s.rm.Len(), "took", time.Since(started).Round(time.Millisecond).String())
}

// remembered reports whether the carriers' records already hold a day's
// booking windows. A day filled a second time, after a restart say, is
// sold under the records and adds nothing to them.
func (s *Sim) remembered(day int64) bool {
	if s.rm == nil {
		return false
	}
	held, ok := s.rm.Day()
	return ok && held >= day
}

// today is the world's day number, zero on its first day.
func (s *Sim) today() int64 {
	if s.econ == nil {
		return 0
	}
	return s.econ.day.Load()
}

// fillOptions is how a day's booking windows are sold. The travellers of
// each day are their own draw, so a flight's record is of days that
// differ, as a real flight's is.
func (s *Sim) fillOptions(day int64) fill.Options {
	return fill.Options{LoadFactor: s.fill, Seed: s.fillSeed + day*7919, Day: s.BookingDate, Tariff: s.tariff,
		Cabins: func(f world.Flight) map[string]int { return host.Cabins(f, s.capacity) },
		// The seats sold before the day are sold to the same travellers
		// the day's own are: the market's, at the carrier's fares, in the
		// channels it sells in.
		Base:    s.baseFare,
		Mix:     s.mixFor,
		Offered: func(carrier, channel string) bool { return s.policyOf(carrier).Offered(channel) },
		Share:   s.shareOf,
		Seller:  s.seller,
		Channels: func() []string {
			var out []string
			for _, g := range gdsSlots {
				out = append(out, g.Designator)
			}
			return out
		}(),
		// A connection sold in advance makes the station's minimum
		// connecting time, as one sold on the day does.
		Connect: s.connectTime,
		RM:      s.rm,
		// A ticket sold before the day is on the same stock as one sold
		// during it, so that settlement and interline billing find its
		// carrier.
		Accounting: s.accountingCode,
	}
}

// burnIn sells the booking windows of the days before the world's first,
// when the carriers have no record: the travellers shop and the carriers'
// records fill, and no booking is written. A simulation of revenue
// management is run this way, because a forecaster that has seen nothing
// forecasts nothing.
func (s *Sim) burnIn(ctx context.Context) {
	if s.rm == nil || s.burn <= 0 || s.rm.Len() > 0 || len(s.Tenants) == 0 {
		return
	}
	sub := &world.Manifest{}
	for code, fs := range s.Flights {
		if _, ok := s.Tenants[code]; ok {
			sub.Flights = append(sub.Flights, fs...)
		}
	}
	started := time.Now()
	day := s.today()
	for back := s.burn; back >= 1; back-- {
		if ctx.Err() != nil {
			return
		}
		opts := s.fillOptions(day - int64(back))
		opts.Dry = true
		if _, err := fill.Day(ctx, sub, opts, nil); err != nil {
			s.log.Warn("burn-in stopped", "err", err)
			return
		}
	}
	s.rm.SetDay(day - 1)
	s.saveRevenueMemory()
	s.log.Info("revenue management burned in", "days", s.burn, "flights", len(sub.Flights), "cabins", s.rm.Len(),
		"took", time.Since(started).Round(time.Millisecond).String())
}

// authorized is the bookings a flight's cabin may hold today: the limit
// the carrier set when the day was filled, or the seats when it set none
// or when another aircraft has the flight.
func (s *Sim) authorized(carrier, flightNum, board, compartment string, seats int) int {
	if s.rm == nil {
		return seats
	}
	if cab := s.rm.Peek(rm.Key(carrier, flightNum, board, compartment)); cab != nil {
		return cab.Limit(seats)
	}
	return seats
}

// shown adds a closed flight's cabin to the carrier's record of who
// came: the bookings it held, and the passengers who stayed away.
func (s *Sim) shown(carrier, flightNum, board, compartment string, booked, away int) {
	if s.rm == nil {
		return
	}
	if cab := s.rm.Peek(rm.Key(carrier, flightNum, board, compartment)); cab != nil {
		cab.Show(booked, away)
	}
}

// learned is what a flight's cabin draws over its booking window, by
// class, as the carrier's record has it, or nil while the flight has not
// flown long enough to have one. The answer is the day's: it is worked
// out once for a flight and kept until the next day is filled.
func (s *Sim) learned(carrier, flightNum, board, off, compartment string) []inventory.ClassDemand {
	if s.rm == nil || s.tariff == nil {
		return nil
	}
	key := rm.Key(carrier, flightNum, board, compartment)
	kept := s.rmBase.Load()
	if v, ok := kept.Load(key); ok {
		return v.([]inventory.ClassDemand)
	}
	var base []inventory.ClassDemand
	if cab := s.rm.Peek(key); cab != nil && cab.Learned() {
		// A class's fare is the one anyone may buy one way. The record is
		// in shares of the cabin's highest fare, and the forecast is in
		// today's money.
		fares := map[string]float64{}
		for _, f := range s.tariff.Fares(carrier, board, off) {
			if f.RoundTrip || f.Rule.SaturdayNight || len(f.Rule.Sale.Accounts) > 0 {
				continue
			}
			if a := float64(f.OneWay.Amount); a > fares[f.Class] {
				fares[f.Class] = a
			}
		}
		if top := fares[cab.Classes()[0]]; top > 0 {
			base = cab.ToCome(0, 1, func(class string) float64 { return fares[class] / top })
			for i := range base {
				base[i].Fare *= top
			}
		}
	}
	kept.Store(key, base)
	return base
}
