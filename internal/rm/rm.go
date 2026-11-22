// Package rm is what each carrier's revenue management remembers of its
// own flights, and the class limits it sets from that.
//
// A flight leaves once in each of the world's days. For each departure
// the carrier keeps, by time frame of the booking window, which class
// was the lowest on sale, for how long, and what was sold meanwhile. The
// record and the forecast taken from it are jetway's
// (inventory.History). This package keeps one record for each flight's
// cabin, says which time frame a booking falls in, and writes the
// records to a file so a restart does not forget them.
//
// It also keeps, for each cabin, who came: of the bookings a departure
// held, how many passengers showed up (inventory.Shows). From that and
// from what a denied boarding and an empty seat cost, the carrier sets
// how many bookings the cabin may hold: its authorised capacity, at or
// over its seats.
//
// The carrier sees what an airline sees: its own bookings and its own
// controls. It does not see the travellers who looked and did not buy.
// The time a class was on sale within a frame is counted in the
// travellers who looked, because the booking window has no clock finer
// than a day. Only the shares are kept, never the counts.
package rm

import (
	"bufio"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/adamf/jetway/pkg/inventory"
)

// frameFrom is the fewest days before departure in each time frame of
// the booking window, earliest frame first. The frames are the 16 of the
// Passenger Origin Destination Simulator (63, 56, 49, 42, 35, 31, 28, 24,
// 21, 17, 14, 10, 7, 5, 3 and 1 days out), with two boundaries moved to
// 30 and 24 so that every advance purchase day of the tariff (30, 21,
// 14, 7 and 3) begins a frame. A class is then on sale for a whole frame
// or for none of it. The first frame is everything booked 63 days out or
// earlier, and the last is the day of departure.
var frameFrom = []int{63, 56, 49, 42, 35, 30, 24, 21, 17, 14, 10, 7, 5, 3, 1, 0}

// Frames is how many time frames the booking window has.
const Frames = 16

// FrameOf is the time frame a booking made lead days before departure
// falls in.
func FrameOf(lead int) int {
	for i, from := range frameFrom {
		if lead >= from {
			return i
		}
	}
	return len(frameFrom) - 1
}

// FrameFrom is the fewest days before departure in a frame of the booking
// window.
func FrameFrom(frame int) int {
	if frame < 0 || frame >= len(frameFrom) {
		return 0
	}
	return frameFrom[frame]
}

// Warm is how many departures a flight flies on the tariff's fixed limits
// before its own record sets them. A week of departures. An estimate.
const Warm = 7

// Spread is the variance of a flight's demand over its mean: a Poisson
// stream's and a half, because parties travel together and days differ.
// It is the square of the 1.25 the fixed forecast used.
const Spread = 1.5625

// Over is the most a cabin is overbooked, as a share of its seats. A
// record that says more is not believed that far.
const Over = 0.20

// DeniedCost is what a carrier counts a denied boarding to cost it when
// it sets a cabin's limit, in units of what an empty seat costs it, which
// is the cabin's average fare. It is 2, because the carrier offers a
// volunteer twice the fare, and that is also what the law's first tier
// gives a passenger turned away (14 CFR 250.5). See docs/business.md.
const DeniedCost = 2.0

// Overbooks reports whether a carrier overbooks. Most do. JetBlue has
// always said it does not, and Southwest stopped in May 2017. The
// Government Accountability Office found 2 such airlines among the 7
// whose documents it read (GAO-20-191).
func Overbooks(carrier string) bool {
	switch strings.ToUpper(carrier) {
	case "B6", "WN":
		return false
	}
	return true
}

// Cabin is one flight's cabin: its record, and a lock, because the day's
// selling asks while the next day's is being written.
type Cabin struct {
	mu sync.Mutex
	h  *inventory.History
	// shows is who came, and limit the bookings the cabin was authorised
	// when its booking window was last sold, for a cabin of limitSeats
	// seats. Zero is no limit set.
	shows      inventory.Shows
	limit      int
	limitSeats int
}

// Memory is every flight's record.
type Memory struct {
	// Method is how the records are read. The zero value is fare
	// adjustment.
	Method inventory.Method
	// Departures is how many departures a forecast remembers. Zero is 26.
	Departures float64

	mu     sync.RWMutex
	cabins map[string]*Cabin
	day    int64
	dated  bool
}

// Day is the last day of the world whose booking windows the records
// hold; ok is false when no day has been named.
func (m *Memory) Day() (day int64, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.day, m.dated
}

// SetDay names the day whose booking windows the records now hold.
func (m *Memory) SetDay(day int64) {
	m.mu.Lock()
	m.day, m.dated = day, true
	m.mu.Unlock()
}

// New makes an empty memory.
func New() *Memory { return &Memory{cabins: map[string]*Cabin{}} }

// Key names a flight's cabin: the carrier, the flight number, where it
// boards and the compartment.
func Key(carrier, number, board, compartment string) string {
	return carrier + "/" + strings.TrimLeft(number, "0") + "/" + board + "/" + compartment
}

// Cabin is the record of a flight's cabin, made empty if there is none.
// classes are the cabin's booking classes, highest fare first; a record
// kept for other classes is started again.
func (m *Memory) Cabin(key string, classes []string) *Cabin {
	m.mu.RLock()
	c := m.cabins[key]
	m.mu.RUnlock()
	if c != nil && same(c.h.Classes, classes) {
		return c
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	old := m.cabins[key]
	if old != nil && same(old.h.Classes, classes) {
		return old
	}
	c = &Cabin{h: inventory.NewHistory(classes, Frames)}
	if old != nil {
		// The classes changed and the booking record starts again. Who
		// came does not depend on the classes, and is kept.
		old.mu.Lock()
		c.shows, c.limit, c.limitSeats = old.shows, old.limit, old.limitSeats
		old.mu.Unlock()
	}
	c.shows.Memory = m.Departures
	m.dress(c.h, key)
	m.cabins[key] = c
	return c
}

// Peek is the record of a flight's cabin, or nil.
func (m *Memory) Peek(key string) *Cabin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cabins[key]
}

// Len is how many cabins have a record.
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cabins)
}

// dress sets how a record is read. Each flight forgets a strategy it no
// longer uses after its own number of departures, so the carrier's
// flights do not all try their closed classes again on one day.
func (m *Memory) dress(h *inventory.History, key string) {
	h.Method, h.Memory, h.Spread = m.Method, m.Departures, Spread
	salt := fnv.New64a()
	salt.Write([]byte(key))
	h.Salt = salt.Sum64()
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Departures is how many departures the cabin's record holds.
func (c *Cabin) Departures() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.Departures()
}

// Learned reports whether the cabin's own record sets its limits yet.
func (c *Cabin) Learned() bool { return c.Departures() >= Warm }

// Classes are the cabin's booking classes, highest fare first.
func (c *Cabin) Classes() []string { return c.h.Classes }

// Show adds a departure to the record of who came: the confirmed
// bookings the cabin held when the flight closed, and how many of them
// did not fly because the passenger did not come.
func (c *Cabin) Show(booked, noShows int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shows.Record(booked, noShows)
}

// Shown is how many departures the cabin's record of who came holds.
func (c *Cabin) Shown() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shows.Departures()
}

// ShowRate is the cabin's show-up rate: its mean, and its deviation from
// one departure to the next. The deviation is never under that of a cabin
// of seats independent passengers, sqrt(p*(1-p)/seats), which is the
// least a real cabin has. ok is false until the record holds Warm
// departures.
func (c *Cabin) ShowRate(seats int) (mean, deviation float64, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shows.Departures() < Warm {
		return 0, 0, false
	}
	mean, deviation, ok = c.shows.Rate()
	if !ok || seats <= 0 {
		return mean, deviation, ok
	}
	if least := math.Sqrt(mean * (1 - mean) / float64(seats)); deviation < least {
		deviation = least
	}
	return mean, deviation, true
}

// Authorize sets how many bookings the cabin may hold for the booking
// window now to be sold, and returns it. The limit is the one with the
// least expected cost of denied boardings and empty seats together
// (inventory.CostBased), given what one of each costs the carrier and
// the cabin's own show-up rate. A cabin with no record of who came, or
// with a cost of nothing for an empty seat, is authorised its seats.
// slack is seats the carrier counts on elsewhere for passengers this
// cabin cannot seat: the seats it expects empty in the cabin in front.
func (c *Cabin) Authorize(seats, slack int, deniedCost, spoiledCost float64) int {
	limit := seats
	if mean, deviation, ok := c.ShowRate(seats); ok && deniedCost > 0 && spoiledCost > 0 {
		limit = inventory.CostBased(seats+max(slack, 0), mean, deviation, deniedCost, spoiledCost) - max(slack, 0)
		limit = min(max(limit, seats), seats+int(float64(seats)*Over))
	}
	c.mu.Lock()
	c.limit, c.limitSeats = limit, seats
	c.mu.Unlock()
	return limit
}

// Limit is the bookings a cabin of seats seats was authorised when its
// booking window was last sold. A cabin with no limit set, or one set
// for another number of seats, has its seats: a smaller aircraft on the
// day is not sold past its own.
func (c *Cabin) Limit(seats int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.limit <= 0 || c.limitSeats != seats {
		return seats
	}
	return c.limit
}

// Begin starts the record of a departure, or of a part of one.
func (c *Cabin) Begin() *inventory.Log {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.Begin()
}

// Record adds a departure's booking window to the record.
func (c *Cabin) Record(l *inventory.Log) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.h.Record(l)
}

// Controls is how to sell each class from a time frame on, highest class
// first: what a sale is worth, the seats to hold back from it, and
// whether to sell it at all. share is the part of the frame still to
// come.
func (c *Cabin) Controls(frame int, share float64, fare func(class string) float64) []inventory.Control {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.Controls(frame, share, fare)
}

// ToCome is the demand still to come from a time frame on, by class.
func (c *Cabin) ToCome(frame int, share float64, fare func(class string) float64) []inventory.ClassDemand {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.h.ToCome(frame, share, fare)
}

// Save writes every record to a file, whole, beside its old copy until
// the new one is complete.
func (m *Memory) Save(path string) error {
	m.mu.RLock()
	keys := make([]string, 0, len(m.cabins))
	for k := range m.cabins {
		keys = append(keys, k)
	}
	m.mu.RUnlock()
	sort.Strings(keys)
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	bw := bufio.NewWriter(tmp)
	zw := gzip.NewWriter(bw)
	enc := gob.NewEncoder(zw)
	day, dated := m.Day()
	if err := enc.Encode(header{Cabins: len(keys), Day: day, Dated: dated}); err != nil {
		return err
	}
	for _, k := range keys {
		c := m.Peek(k)
		if c == nil {
			continue
		}
		c.mu.Lock()
		b, err := c.h.MarshalBinary()
		sh, _ := c.shows.MarshalBinary()
		limit, limitSeats := c.limit, c.limitSeats
		c.mu.Unlock()
		if err != nil {
			return err
		}
		if err := enc.Encode(saved{Key: k, Record: b, Shows: sh, Limit: limit, LimitSeats: limitSeats}); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

type header struct {
	Cabins int
	Day    int64
	Dated  bool
}

type saved struct {
	Key    string
	Record []byte
	// Shows is the record of who came, and Limit the authorised capacity
	// in force for a cabin of LimitSeats seats. A file written before
	// they were kept has none of them.
	Shows      []byte
	Limit      int
	LimitSeats int
}

// Load reads the records Save wrote. A file that is not there is an
// empty memory.
func (m *Memory) Load(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	zr, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return err
	}
	dec := gob.NewDecoder(zr)
	var head header
	if err := dec.Decode(&head); err != nil {
		return err
	}
	cabins := map[string]*Cabin{}
	for i := 0; i < head.Cabins; i++ {
		var s saved
		if err := dec.Decode(&s); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		h := &inventory.History{}
		if err := h.UnmarshalBinary(s.Record); err != nil {
			return fmt.Errorf("rm: %s: %w", s.Key, err)
		}
		if h.Frames != Frames {
			continue // a record cut into other frames is not this world's
		}
		m.dress(h, s.Key)
		c := &Cabin{h: h, limit: s.Limit, limitSeats: s.LimitSeats}
		c.shows.Memory = m.Departures
		if len(s.Shows) > 0 {
			if err := c.shows.UnmarshalBinary(s.Shows); err != nil {
				return fmt.Errorf("rm: %s: %w", s.Key, err)
			}
		}
		cabins[s.Key] = c
	}
	m.mu.Lock()
	m.cabins, m.day, m.dated = cabins, head.Day, head.Dated
	m.mu.Unlock()
	return nil
}
