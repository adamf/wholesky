// Package stats is the fleet's instrument panel: the whole cluster as time
// series.
//
// Everything here is fed by the same bus taps the Eye and the fleet use --
// counters incremented per event, snapshotted into ring buffers every two
// seconds -- so the charts cost nothing when nobody is looking and almost
// nothing when somebody is. Ten minutes of history at two-second resolution:
// enough to watch a morning bank rise, a cascade spike, a deploy settle.
package stats

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ringLen is 10 minutes at 2-second steps.
const ringLen = 300

type ring struct {
	v   [ringLen]float64
	idx int
	n   int
}

func (r *ring) push(x float64) {
	r.v[r.idx] = x
	r.idx = (r.idx + 1) % ringLen
	if r.n < ringLen {
		r.n++
	}
}

func (r *ring) slice() []float64 {
	out := make([]float64, r.n)
	start := (r.idx - r.n + ringLen) % ringLen
	for i := 0; i < r.n; i++ {
		out[i] = r.v[(start+i)%ringLen]
	}
	return out
}

// Collector accumulates the cluster's pulse.
type Collector struct {
	mu sync.Mutex

	// window counters, reset each tick
	total, typeb, edifact         int64
	kindAVS, kindMVT, kindRES     int64
	kindASM, kindOther            int64
	undeliverable, bookings, mvts int64
	queuePlaced                   int64

	// series, per second
	sTotal, sTypeb, sEdifact        ring
	sAVS, sMVT, sRES, sASM, sOther  ring
	sUndeliv, sBookings, sMovements ring
	sAirborne, sQueued              ring

	// running totals for the headline numbers
	tTotal, tBookings, tMovements, tUndeliv int64

	// Airborne and QueueDepths are polled at snapshot time rather than fed,
	// because they are states, not events.
	Airborne    func() int
	QueueDepths func() map[string]int
	LinksUp     func() int

	latestQueues map[string]int
	started      time.Time
}

// New builds a collector and starts its snapshot loop.
func New() *Collector {
	return &Collector{started: time.Now(), latestQueues: map[string]int{}}
}

// Run snapshots the windows into the rings until the context ends. Two
// seconds is the same cadence as the Eye's stats, so the pages agree.
func (c *Collector) Run(stop <-chan struct{}) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	n := 0
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}
		n++
		c.mu.Lock()
		per := func(x int64) float64 { return float64(x) / 2 }
		c.sTotal.push(per(c.total))
		c.sTypeb.push(per(c.typeb))
		c.sEdifact.push(per(c.edifact))
		c.sAVS.push(per(c.kindAVS))
		c.sMVT.push(per(c.kindMVT))
		c.sRES.push(per(c.kindRES))
		c.sASM.push(per(c.kindASM))
		c.sOther.push(per(c.kindOther))
		c.sUndeliv.push(per(c.undeliverable))
		c.sBookings.push(per(c.bookings))
		c.sMovements.push(per(c.mvts))
		c.total, c.typeb, c.edifact = 0, 0, 0
		c.kindAVS, c.kindMVT, c.kindRES, c.kindASM, c.kindOther = 0, 0, 0, 0, 0
		c.undeliverable, c.bookings, c.mvts, c.queuePlaced = 0, 0, 0, 0
		if c.Airborne != nil {
			c.sAirborne.push(float64(c.Airborne()))
		}
		// Queue depths are a store read on one node; every fifth tick is
		// plenty for a chart.
		if c.QueueDepths != nil && n%5 == 0 {
			c.latestQueues = c.QueueDepths()
		}
		q := 0
		for _, v := range c.latestQueues {
			q += v
		}
		c.sQueued.push(float64(q))
		c.mu.Unlock()
	}
}

// OnMessage feeds one switch-side message event: every message in the world
// crosses the switch, so this window is the world's.
func (c *Collector) OnMessage(payload map[string]any) {
	kind, _ := payload["kind"].(string)
	format, _ := payload["format"].(string)
	status, _ := payload["status"].(string)
	head := strings.SplitN(kind, "/", 2)[0]
	c.mu.Lock()
	c.total++
	c.tTotal++
	switch format {
	case "typeb":
		c.typeb++
	case "edifact":
		c.edifact++
	}
	switch head {
	case "AVS":
		c.kindAVS++
	case "MVT", "MVA", "DIV":
		c.kindMVT++
	case "AIRIMP", "PAOREQ", "PAORES", "relay":
		c.kindRES++
	case "ASM", "SSM":
		c.kindASM++
	default:
		c.kindOther++
	}
	if status == "undeliverable" {
		c.undeliverable++
		c.tUndeliv++
	}
	c.mu.Unlock()
}

// OnBooking counts a record event at the GDS.
func (c *Collector) OnBooking() {
	c.mu.Lock()
	c.bookings++
	c.tBookings++
	c.mu.Unlock()
}

// OnMovement counts a movement recognised at the watcher.
func (c *Collector) OnMovement() {
	c.mu.Lock()
	c.mvts++
	c.tMovements++
	c.mu.Unlock()
}

// Routes mounts the panel.
func (c *Collector) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /stats", c.page)
	mux.HandleFunc("GET /stats/data.json", c.data)
}

func (c *Collector) data(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	out := map[string]any{
		"step": 2,
		"series": map[string][]float64{
			"total": c.sTotal.slice(), "typeb": c.sTypeb.slice(), "edifact": c.sEdifact.slice(),
			"avs": c.sAVS.slice(), "mvt": c.sMVT.slice(), "res": c.sRES.slice(),
			"asm": c.sASM.slice(), "other": c.sOther.slice(),
			"undeliverable": c.sUndeliv.slice(),
			"bookings":      c.sBookings.slice(), "movements": c.sMovements.slice(),
			"airborne": c.sAirborne.slice(), "queued": c.sQueued.slice(),
		},
		"totals": map[string]int64{
			"messages": c.tTotal, "bookings": c.tBookings,
			"movements": c.tMovements, "undeliverable": c.tUndeliv,
		},
		"queues": c.latestQueues,
		"uptime": int(time.Since(c.started).Seconds()),
	}
	if c.LinksUp != nil {
		out["links"] = c.LinksUp()
	}
	c.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out) //nolint:errcheck
}
