// Package eye watches the sky and draws it.
//
// The Eye subscribes to the buses the world already publishes on -- the
// switch's message stream, the GDS's movement and booking events -- and
// serves a map: airports from the manifest, aircraft flown by the MVT
// messages crossing the fabric, message traffic as pulses along the network
// star. Nothing here reaches into a store or a gateway; if it is not on a
// bus, the Eye does not know it, which keeps the Eye honest about what the
// network can actually see.
package eye

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

// Plane is one aircraft the Eye believes is in the air.
type Plane struct {
	Flight   string    `json:"flight"` // e.g. "U2123"
	Reg      string    `json:"reg"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	FromLat  float64   `json:"from_lat"`
	FromLon  float64   `json:"from_lon"`
	ToLat    float64   `json:"to_lat"`
	ToLon    float64   `json:"to_lon"`
	Departed time.Time `json:"departed"`
	Arriving time.Time `json:"arriving"`
}

// Eye is the observer.
type Eye struct {
	manifest *world.Manifest
	airports map[string]*world.Airport
	// byFlight resolves "U2"+"0123" to its scheduled leg.
	byFlight map[string]world.Flight
	warp     int

	// Chaos, when set, receives the map's control actions -- ("close","LHR"),
	// ("reopen","LHR"). The Eye owns the page and the halos; what closing an
	// airport means is the simulation's business, not the observer's.
	Chaos func(action, iata string) error

	mu     sync.Mutex
	planes map[string]*Plane
	subs   map[chan []byte]struct{}
	closed map[string]bool
	halos  map[string]int64
	// pulseBudget rate-limits pulse events to the browser: the world's fabric
	// runs hundreds of messages a second, and a view needs the rhythm, not
	// every beat.
	pulseBudget int
	pulseReset  time.Time

	// counters for the stats line
	messages  int64
	movements int64
	bookings  int64
}

// New builds an Eye over a compiled world.
func New(m *world.Manifest, warp int) *Eye {
	if warp < 1 {
		warp = 1
	}
	e := &Eye{
		manifest: m, warp: warp,
		airports: map[string]*world.Airport{},
		byFlight: map[string]world.Flight{},
		planes:   map[string]*Plane{},
		subs:     map[chan []byte]struct{}{},
		closed:   map[string]bool{},
		halos:    map[string]int64{},
	}
	for i := range m.Airports {
		e.airports[m.Airports[i].IATA] = &m.Airports[i]
	}
	for _, f := range m.Flights {
		e.byFlight[f.Carrier+f.Number] = f
	}
	return e
}

// OnMessage folds one switch-bus message event into the traffic picture.
//
// The payload is the gateway's own message view; only the envelope matters
// here. Pulses are emitted raw and the browser thins them -- at demo rates
// that is fine, and at world rates the Eye aggregates before this point.
func (e *Eye) OnMessage(payload map[string]any) {
	e.mu.Lock()
	e.messages++
	now := time.Now()
	if now.After(e.pulseReset) {
		e.pulseBudget, e.pulseReset = 120, now.Add(time.Second)
	}
	if e.pulseBudget <= 0 {
		e.mu.Unlock()
		return
	}
	e.pulseBudget--
	e.mu.Unlock()
	kind, _ := payload["kind"].(string)
	e.broadcast(map[string]any{
		"t":    "pulse",
		"peer": payload["peer"], "dir": payload["direction"],
		"kind": strings.SplitN(kind, "/", 2)[0], "fmt": payload["format"],
	})
}

// OnQueue folds a queue placement into the halos.
//
// A schedule-change item names its flight in the reason; the manifest turns
// that into airports, and a closed airport among them gets the credit. The
// halo is therefore a count of real queue items -- bookings a person now has
// to deal with -- not an animation invented for effect.
func (e *Eye) OnQueue(queue, reason string) {
	if queue != "schedule-change" {
		return
	}
	m := flightRe.FindStringSubmatch(reason)
	if m == nil {
		return
	}
	f, ok := e.lookup(m[1])
	if !ok {
		return
	}
	e.mu.Lock()
	var hit string
	for _, apt := range []string{f.From, f.To} {
		if e.closed[apt] {
			hit = apt
			e.halos[apt]++
		}
	}
	var n int64
	if hit != "" {
		n = e.halos[hit]
	}
	e.mu.Unlock()
	if hit != "" {
		e.broadcast(map[string]any{"t": "halo", "airport": hit, "count": n})
	}
}

// Airport reports whether the world holds an airport, for chaos validation.
func (e *Eye) Airport(iata string) (*world.Airport, bool) {
	a, ok := e.airports[iata]
	return a, ok
}

// ClearHalo forgets a reopened airport's halo.
func (e *Eye) ClearHalo(iata string) {
	e.mu.Lock()
	delete(e.closed, iata)
	delete(e.halos, iata)
	e.mu.Unlock()
	e.broadcast(map[string]any{"t": "reopen", "airport": iata})
}

// OnMovement folds a movement event into the aircraft picture.
//
// The event names the flight and the station it was sent from; the schedule
// says the rest. A movement at the flight's origin is a departure and spawns
// the aircraft; at its destination, an arrival and lands it. The Eye flies
// the plane between the two on the schedule's block time, warped -- the
// messages say when it left and landed, the manifest says where it was going.
func (e *Eye) OnMovement(payload map[string]any) {
	flight, _ := payload["flight"].(string)
	station, _ := payload["station"].(string)
	reg, _ := payload["registration"].(string)

	f, ok := e.lookup(flight)
	if !ok {
		return
	}
	e.mu.Lock()
	e.movements++
	switch station {
	case f.From:
		from, okF := e.airports[f.From]
		to, okT := e.airports[f.To]
		if !okF || !okT {
			e.mu.Unlock()
			return
		}
		now := time.Now()
		p := &Plane{
			Flight: flight, Reg: reg, From: f.From, To: f.To,
			FromLat: from.Lat, FromLon: from.Lon, ToLat: to.Lat, ToLon: to.Lon,
			Departed: now,
			Arriving: now.Add(time.Duration(f.BlockMin) * time.Minute / time.Duration(e.warp)),
		}
		e.planes[flight] = p
		e.mu.Unlock()
		e.broadcast(map[string]any{"t": "dep", "plane": p})
	case f.To:
		delete(e.planes, flight)
		e.mu.Unlock()
		e.broadcast(map[string]any{"t": "arr", "flight": flight})
	default:
		e.mu.Unlock()
	}
}

// OnBooking counts a settled or created booking for the stats line.
func (e *Eye) OnBooking() {
	e.mu.Lock()
	e.bookings++
	e.mu.Unlock()
}

// flightRe pulls the flight designator out of a queue reason, which leads
// with the segment description: "U2123 Y LGW-AMS ...".
var flightRe = regexp.MustCompile(`\b([A-Z][A-Z0-9]\d{1,4})\b`)

// lookup resolves a wire flight designator ("U2123") to its scheduled leg.
// The wire drops leading zeros; the manifest pads to four.
func (e *Eye) lookup(flight string) (world.Flight, bool) {
	if len(flight) < 3 {
		return world.Flight{}, false
	}
	carrier, num := flight[:2], flight[2:]
	for len(num) < 4 {
		num = "0" + num
	}
	f, ok := e.byFlight[carrier+num]
	return f, ok
}

// Routes mounts the Eye on a mux.
func (e *Eye) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /eye", e.page)
	mux.HandleFunc("GET /eye/world.json", e.world)
	mux.HandleFunc("GET /eye/planes.json", e.planesNow)
	mux.HandleFunc("GET /eye/stream", e.stream)
	mux.HandleFunc("POST /eye/chaos", e.chaos)
}

// chaos is the map's one control: close or reopen an airport.
func (e *Eye) chaos(w http.ResponseWriter, r *http.Request) {
	if e.Chaos == nil {
		http.Error(w, "this world has no chaos hook", http.StatusNotImplemented)
		return
	}
	var req struct {
		Action  string `json:"action"`
		Airport string `json:"airport"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}
	req.Airport = strings.ToUpper(strings.TrimSpace(req.Airport))
	if err := e.Chaos(req.Action, req.Airport); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	e.mu.Lock()
	if req.Action == "close" {
		e.closed[req.Airport] = true
	}
	closed := make([]string, 0, len(e.closed))
	for a := range e.closed {
		closed = append(closed, a)
	}
	e.mu.Unlock()
	if req.Action == "close" {
		e.broadcast(map[string]any{"t": "close", "airport": req.Airport})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"closed": closed}) //nolint:errcheck
}

// world serves the static geography: what the map is drawn from.
func (e *Eye) world(w http.ResponseWriter, r *http.Request) {
	type apt struct {
		IATA string  `json:"iata"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
		N    int     `json:"n"` // flights touching, for dot size
	}
	touch := map[string]int{}
	for _, f := range e.manifest.Flights {
		touch[f.From]++
		touch[f.To]++
	}
	var apts []apt
	for _, a := range e.manifest.Airports {
		apts = append(apts, apt{IATA: a.IATA, Lat: a.Lat, Lon: a.Lon, N: touch[a.IATA]})
	}
	type hub struct {
		Code string  `json:"code"`
		Lat  float64 `json:"lat"`
		Lon  float64 `json:"lon"`
	}
	var hubs []hub
	for _, c := range e.manifest.Carriers {
		if a, ok := e.airports[c.Hub]; ok {
			hubs = append(hubs, hub{Code: c.Designator, Lat: a.Lat, Lon: a.Lon})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"airports": apts,
		"carriers": hubs,
		"warp":     e.warp,
	})
}

func (e *Eye) planesNow(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	out := make([]*Plane, 0, len(e.planes))
	for _, p := range e.planes {
		out = append(out, p)
	}
	e.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out) //nolint:errcheck
}

// stream is the live feed: one SSE event per pulse, departure, arrival, and a
// stats line every two seconds.
func (e *Eye) stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")

	ch := make(chan []byte, 256)
	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.subs, ch)
		e.mu.Unlock()
	}()

	stats := time.NewTicker(2 * time.Second)
	defer stats.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
			fl.Flush()
		case <-stats.C:
			e.mu.Lock()
			closed := make(map[string]int64, len(e.closed))
			for a := range e.closed {
				closed[a] = e.halos[a]
			}
			b, _ := json.Marshal(map[string]any{
				"t": "stats", "messages": e.messages, "movements": e.movements,
				"bookings": e.bookings, "airborne": len(e.planes), "closed": closed,
			})
			e.mu.Unlock()
			fmt.Fprintf(w, "data: %s\n\n", b) //nolint:errcheck
			fl.Flush()
		}
	}
}

// broadcast fans an event to every stream. A slow client's events are
// dropped, not queued: the feed is a view, and a view that lags is worse
// than one that skips.
func (e *Eye) broadcast(v map[string]any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	e.mu.Lock()
	for ch := range e.subs {
		select {
		case ch <- b:
		default:
		}
	}
	e.mu.Unlock()
}
