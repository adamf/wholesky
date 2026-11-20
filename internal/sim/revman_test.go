package sim

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/inventory"

	"github.com/adamf/wholesky/internal/host"
	"github.com/adamf/wholesky/internal/rm"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// A world with no record sells some days of booking windows before its
// first, writing no booking, and its carriers begin with limits their own
// flights set. The records are kept in a file beside the state file. The
// day's forecaster then reads the booking curve on what a flight's record
// says it draws, and not on the tariff's fixed mix.
func TestCarriersBeginWithWhatTheirFlightsTaughtThem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	state := filepath.Join(t.TempDir(), "state.json")
	opts := Options{GDSCount: 1, Fill: 0.8, FillSeed: 3, BurnIn: 9, StateFile: state, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s, err := Boot(ctx, oneCarrier(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Stop()
	if s.rm == nil || s.rm.Len() != 0 {
		t.Fatal("a new world should have an empty memory")
	}
	s.burnIn(ctx)
	cabins := s.rm.Len()
	if cabins == 0 {
		t.Fatal("nothing was burned in")
	}
	if day, ok := s.rm.Day(); !ok || day != s.today()-1 {
		t.Errorf("the records hold day %d (%v), the world is on day %d", day, ok, s.today())
	}
	// A second burn-in does nothing: the carriers have a record.
	s.burnIn(ctx)
	s.fillDay(ctx)
	s.rebuildInventories(ctx)
	if _, err := os.Stat(state + ".rm"); err != nil {
		t.Errorf("the records were not written beside the state file: %v", err)
	}

	// A flight with a record, leaving late enough that the whole day's
	// booking curve is still to come.
	s.clock.Sync(time.Now(), 0, s.clock.Warp())
	wire := strings.ToUpper(s.BookingDate.Format("02Jan"))
	checked := false
	for code, ten := range s.Tenants {
		for _, f := range s.Flights[code] {
			if checked || f.DepMin < 12*60 || f.Marketing != "" {
				continue
			}
			cab := s.rm.Peek(rm.Key(f.Carrier, f.Number, f.From, "Y"))
			base := s.learned(f.Carrier, f.Number, f.From, f.To, "Y")
			if cab == nil || base == nil {
				continue
			}
			// 9 days burned in and the day itself.
			if got := cab.Departures(); got != 10 {
				t.Errorf("%s%s has %d departures on record, want 10", f.Carrier, f.Number, got)
			}
			var toCome float64
			for _, d := range base {
				toCome += d.Mean
			}
			seats := host.Cabins(f, s.capacity)["Y"]
			if toCome < 0.2*float64(seats) || toCome > 3*float64(seats) {
				t.Errorf("%s%s draws %.0f over its booking window for %d seats", f.Carrier, f.Number, toCome, seats)
			}
			// The ladder the carrier's system sells under is EMSRb on the
			// flight's record, and is not the fixed mix's ladder.
			sold := ten.Inventory.SoldByClass(f.Carrier, f.Number, wire, f.From, "Y")
			full := tariff.FullFare(s.tariff, f.Carrier, f.From, f.To)
			price := func(class string) float64 { return tariff.Share(class) * float64(full) }
			got := ten.Inventory.Levels(f.Carrier, f.Number, wire, f.From, "Y")
			want := inventory.EMSRb(seats, tariff.PickupFrom(base, sold, 1, price))
			fixed := inventory.EMSRb(seats, tariff.PickupPriced("Y", seats, sold, 1, full))
			level := func(ls []inventory.Level) map[string]int {
				out := map[string]int{}
				for _, l := range ls {
					out[l.Class] = l.Authorized
				}
				return out
			}
			g, w, fx := level(got), level(want), level(fixed)
			differs := false
			for class, n := range w {
				if d := g[class] - n; d < -1 || d > 1 {
					t.Errorf("%s%s class %s is authorised %d, the flight's record gives %d", f.Carrier, f.Number, class, g[class], n)
				}
				if d := g[class] - fx[class]; d < -2 || d > 2 {
					differs = true
				}
			}
			if !differs {
				t.Errorf("%s%s sells under the fixed mix's ladder %v", f.Carrier, f.Number, fx)
			}
			checked = true

			// The same day filled again adds nothing to the record. The
			// next day adds a departure. The book is purged between, as
			// it is when a day ends: a world in memory once kept the late
			// bookers through the purge, and the next fill then met its
			// own locators and wrote nothing.
			s.purgeTenants(ctx, time.Now())
			s.fillDay(ctx)
			if got := cab.Departures(); got != 10 {
				t.Errorf("a day filled twice is on record twice: %d departures", got)
			}
			s.newDay()
			s.purgeTenants(ctx, time.Now())
			s.fillDay(ctx)
			if got := cab.Departures(); got != 11 {
				t.Errorf("after the next day %d departures, want 11", got)
			}
			if booked, _ := (seatWorld{s}).booked(f); booked == 0 {
				t.Errorf("%s%s has nobody on it on the second day", f.Carrier, f.Number)
			}
		}
	}
	if !checked {
		t.Fatal("no flight had a record")
	}

	// A world that starts again reads the records and burns nothing in.
	again := &Sim{log: opts.Log}
	again.initRevenueMemory(opts)
	if again.rm == nil || again.rm.Len() != cabins {
		t.Errorf("a restart read %d cabins, the world had %d", again.rm.Len(), cabins)
	}
	// A world told to keep no record has none.
	none := &Sim{log: opts.Log}
	opts.NoRevenueMemory = true
	none.initRevenueMemory(opts)
	if none.rm != nil {
		t.Error("a world told to keep no record kept one")
	}
}

// oneCarrier is the small world with one of its carriers' flights: a day
// of it is filled in a moment.
func oneCarrier(t *testing.T) *world.Manifest {
	t.Helper()
	m := smallWorld(t)
	count := map[string]int{}
	for _, f := range m.Flights {
		count[f.Carrier]++
	}
	keep := ""
	for code, n := range count {
		if n >= 150 && (keep == "" || n < count[keep] || (n == count[keep] && code < keep)) {
			keep = code
		}
	}
	out := *m
	out.Flights = nil
	for _, f := range m.Flights {
		if f.Carrier == keep {
			out.Flights = append(out.Flights, f)
		}
	}
	if len(out.Flights) == 0 {
		t.Fatal("no carrier with 150 flights")
	}
	return &out
}
