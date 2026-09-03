package host

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/inventory"
	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/world"
)

// The network programme behind this carrier's revenue management. The
// leg ladders value a connecting passenger leg by leg; the deterministic
// programme (jetway's inventory.NetworkBidPrices) values the network:
// over each connecting point, the legs departing in the next few hours,
// their local demand by class from the pickup forecaster, and the
// connecting itineraries the book has actually sold through that point,
// with the demand still to come read off the booking curve. The dual of
// each leg's capacity is its bid price, which the controller reads in
// place of the ladder's displacement cost. The programme is re-solved on
// a timer; between solves the controller uses the last answer, and a leg
// the programme did not reach falls back to its ladder.

// networkWindow is how far ahead of the clock the programme looks, in
// minutes: the legs whose last seats are being decided now.
const networkWindow = 240.0

// networkMaxLegs bounds one connecting point's problem so a hub carrier's
// bank solves in seconds on a dense simplex; the itineraries kept are
// the most booked.
const networkMaxLegs = 120

// networkRM holds the programme's answers between solves.
type networkRM struct {
	mu     sync.Mutex
	bids   map[string]float64
	status NetworkStatus
}

// NetworkStatus is what the panel shows of the programme.
type NetworkStatus struct {
	Solved   time.Time `json:"solved"`
	Hubs     int       `json:"hubs"`
	Legs     int       `json:"legs"`
	Products int       `json:"products"`
	Priced   int       `json:"priced"`
	Took     string    `json:"took"`
}

// Network is the state of the network programme.
func (t *Tenant) Network() NetworkStatus {
	if t.net == nil {
		return NetworkStatus{}
	}
	t.net.mu.Lock()
	defer t.net.mu.Unlock()
	return t.net.status
}

// BidPrice is the controller's hook: the leg's dual from the last solve,
// or false when the programme has not priced it.
func (t *Tenant) BidPrice(carrier, flightNum, wireDate, board, compartment string) (float64, bool) {
	if t.net == nil {
		return 0, false
	}
	t.net.mu.Lock()
	defer t.net.mu.Unlock()
	b, ok := t.net.bids[carrier+"/"+strings.TrimLeft(flightNum, "0")+"/"+wireDate+"/"+board+"/"+compartment]
	return b, ok
}

// runNetwork re-solves the programme while the tenant lives.
func (t *Tenant) runNetwork(ctx context.Context, every time.Duration) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if err := t.solveNetwork(ctx); err != nil {
			t.log.Debug("network programme not solved", "err", err)
		}
		timer.Reset(every)
	}
}

// pathStat is one connecting itinerary the book has sold: its legs, the
// point it connects at, how many passengers and what they paid in all.
type pathStat struct {
	legs []string
	hub  string
	n    int
	fare float64
}

// solveNetwork solves the programme once over every connecting point.
func (t *Tenant) solveNetwork(ctx context.Context) error {
	if t.net == nil || t.dayPos == nil || t.tariff == nil || t.capacity == nil {
		return nil
	}
	start := time.Now()
	pos := t.dayPos()
	wire := strings.ToUpper(t.bookingDate.Format("02Jan"))
	legKey := func(carrier, num, board string) string {
		return carrier + "/" + strings.TrimLeft(num, "0") + "/" + wire + "/" + board
	}
	type legInfo struct {
		f         world.Flight
		seats     int
		remaining float64
	}
	legs := map[string]legInfo{}
	for _, f := range t.flights {
		if float64(f.DepMin) <= pos || float64(f.DepMin) > pos+networkWindow {
			continue
		}
		comps, ok := t.capacity(f.Carrier, f.Number, wire, f.From)
		if !ok || comps["Y"] <= 0 {
			continue
		}
		legs[legKey(f.Carrier, f.Number, f.From)] = legInfo{f: f, seats: comps["Y"], remaining: (float64(f.DepMin) - pos) / float64(f.DepMin)}
	}
	paths := map[string]*pathStat{}
	var recs []*pnr.PNR
	if len(legs) > 0 {
		var err error
		if recs, err = t.Store.ListPNRs(ctx, 1_000_000); err != nil {
			return err
		}
	}
	for _, rec := range recs {
		var air []*pnr.Segment
		for i := range rec.Segments {
			s := &rec.Segments[i]
			if s.Type == pnr.SegmentAir && s.Carrier == t.Carrier.Designator && s.WireDate == wire && heldOrAsked(s.Status) {
				air = append(air, s)
			}
		}
		for i := 0; i+1 < len(air); i++ {
			a, b := air[i], air[i+1]
			if a.Off != b.Board {
				continue
			}
			ka, kb := legKey(a.Carrier, a.FlightNum, a.Board), legKey(b.Carrier, b.FlightNum, b.Board)
			if _, ok := legs[ka]; !ok {
				continue
			}
			if _, ok := legs[kb]; !ok {
				continue
			}
			fare, ok := farePerPassenger(rec, a, b)
			if !ok {
				continue
			}
			k := ka + ">" + kb
			p := paths[k]
			if p == nil {
				p = &pathStat{legs: []string{ka, kb}, hub: a.Off}
				paths[k] = p
			}
			seats := max(a.Seats, 1)
			p.n += seats
			p.fare += fare * float64(seats)
		}
	}
	byHub := map[string][]*pathStat{}
	for k, p := range paths {
		_ = k
		byHub[p.hub] = append(byHub[p.hub], p)
	}
	hubs := make([]string, 0, len(byHub))
	for h := range byHub {
		hubs = append(hubs, h)
	}
	sort.Strings(hubs)
	bids := map[string]float64{}
	status := NetworkStatus{Hubs: len(hubs)}
	for _, h := range hubs {
		hubPaths := byHub[h]
		sort.Slice(hubPaths, func(i, j int) bool {
			if hubPaths[i].n != hubPaths[j].n {
				return hubPaths[i].n > hubPaths[j].n
			}
			return hubPaths[i].legs[0]+hubPaths[i].legs[1] < hubPaths[j].legs[0]+hubPaths[j].legs[1]
		})
		used := map[string]bool{}
		var kept []*pathStat
		for _, p := range hubPaths {
			add := 0
			for _, l := range p.legs {
				if !used[l] {
					add++
				}
			}
			if len(used)+add > networkMaxLegs {
				break
			}
			for _, l := range p.legs {
				used[l] = true
			}
			kept = append(kept, p)
		}
		capacity := map[string]float64{}
		var its []inventory.Itinerary
		for l := range used {
			li := legs[l]
			sold := t.Inventory.SoldByClass(li.f.Carrier, li.f.Number, wire, li.f.From, "Y")
			total := 0
			for _, n := range sold {
				total += n
			}
			capacity[l] = float64(max(li.seats-total, 0))
			full := tariff.FullFare(t.tariff, li.f.Carrier, li.f.From, li.f.To)
			for _, d := range tariff.PickupPriced("Y", li.seats, sold, li.remaining, full) {
				if d.Mean > 0 && d.Fare > 0 {
					its = append(its, inventory.Itinerary{Legs: []string{l}, Fare: d.Fare, Demand: d.Mean})
				}
			}
		}
		for _, p := range kept {
			li := legs[p.legs[0]]
			// What is still to come is what has come, scaled by the share
			// of the booking curve left; a path with one booking late in
			// the day still counts for half a seat.
			toCome := float64(p.n) * li.remaining / math.Max(1-li.remaining, 0.25)
			its = append(its, inventory.Itinerary{Legs: p.legs, Fare: p.fare / float64(p.n), Demand: math.Max(toCome, 0.5)})
		}
		b, _, err := inventory.NetworkBidPrices(capacity, its)
		if err != nil {
			return err
		}
		for l, v := range b {
			// A leg through two connecting points takes the dearer valuation;
			// a leg the programme reached is priced even when at nothing.
			if old, ok := bids[l+"/Y"]; !ok || v > old {
				bids[l+"/Y"] = v
			}
		}
		status.Legs += len(used)
		status.Products += len(its)
	}
	for _, v := range bids {
		if v > 0 {
			status.Priced++
		}
	}
	status.Solved = time.Now()
	status.Took = time.Since(start).Round(time.Millisecond).String()
	t.net.mu.Lock()
	t.net.bids, t.net.status = bids, status
	t.net.mu.Unlock()
	return nil
}

// heldOrAsked is a segment status that occupies or is asking for a seat.
func heldOrAsked(status string) bool {
	switch status {
	case "HK", "KK", "TK", "HN", "NN", "KL", "HL":
		return true
	}
	return false
}

// farePerPassenger is what the record's pricing says one passenger pays
// for the two segments together, averaged over its passengers.
func farePerPassenger(rec *pnr.PNR, a, b *pnr.Segment) (float64, bool) {
	if rec.Pricing == nil || len(rec.Pricing.Passengers) == 0 {
		return 0, false
	}
	index := map[int]int{}
	for i := range rec.Segments {
		index[rec.Segments[i].Ref] = i
	}
	var total float64
	n := 0
	for _, pp := range rec.Pricing.Passengers {
		ia, okA := index[a.Ref]
		ib, okB := index[b.Ref]
		if !okA || !okB || ia >= len(pp.Segments) || ib >= len(pp.Segments) {
			return 0, false
		}
		total += float64(pp.Segments[ia] + pp.Segments[ib])
		n++
	}
	if n == 0 {
		return 0, false
	}
	return total / float64(n), true
}
