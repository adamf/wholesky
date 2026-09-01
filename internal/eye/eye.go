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
	_ "embed"
	"sort"

	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/world"
)

// Plane is one aircraft the Eye believes is in the air.
// FlightRecord is one booking as the drill-through lists it.
type FlightRecord struct {
	Locator string `json:"locator"`
	Surname string `json:"surname"`
	Party   int    `json:"party"`
	Status  string `json:"status"`
	GDS     string `json:"gds"`
}

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
	// Diverted marks an aircraft sent somewhere it was not scheduled to go.
	Diverted bool `json:"diverted,omitempty"`
}

// Eye is the observer.
type Eye struct {
	manifest *world.Manifest
	airports map[string]*world.Airport
	// byFlight resolves "U2"+"0123" to its scheduled leg.
	byFlight map[string]world.Flight
	warp     int

	// WarpNow, when set, reads the world's current time rate; SimPos reads
	// its position in the sim day, in minutes. SetWarp changes the rate --
	// the page's time control. Hubs names the distribution systems, so the
	// logical web knows which nodes anchor it.
	WarpNow func() int
	SimPos  func() float64
	SetWarp func(int) error
	Hubs    []string

	// FlightPNRs, when set, answers the drill-through from an aircraft to
	// the bookings riding on it: what the distribution world holds on a
	// flight, straight from the stores.
	FlightPNRs func(flight string) []FlightRecord

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
	// peerWindow counts messages per peer in the current stats window, so the
	// topology view can weight an edge by its true rate even though the
	// pulses riding it are sampled.
	peerWindow map[string]int64

	// The logical web. The switch is transparent plumbing; the graph worth
	// drawing is who converses with whom, end to end. Every relayed message
	// carries the id of the inbound message it forwards, so remembering
	// recent inbound origins lets an outbound relay be resolved to a
	// src->dst edge -- the conversation, with the switch removed.
	inboundSrc  map[string]inMeta
	inboundFIFO []string
	edgeWindow  map[string]int64 // "SRC>DST" -> count this window
	flowBudget  int

	// counters for the stats line
	messages  int64
	movements int64
	bookings  int64
}

// New builds an Eye over a compiled world.
// warpNow reads the live rate, falling back to the boot value. Movement
// arithmetic divides by it, and a paused world must not divide by zero --
// nothing departs while paused, but a message already decoded still lands.
func (e *Eye) warpNow() int {
	w := e.warp
	if e.WarpNow != nil {
		w = e.WarpNow()
	}
	if w < 1 {
		return 1
	}
	return w
}

func New(m *world.Manifest, warp int) *Eye {
	if warp < 1 {
		warp = 1
	}
	e := &Eye{
		manifest: m, warp: warp,
		airports:   map[string]*world.Airport{},
		byFlight:   map[string]world.Flight{},
		planes:     map[string]*Plane{},
		subs:       map[chan []byte]struct{}{},
		closed:     map[string]bool{},
		halos:      map[string]int64{},
		peerWindow: map[string]int64{},
		inboundSrc: map[string]inMeta{},
		edgeWindow: map[string]int64{},
	}
	for i := range m.Airports {
		e.airports[m.Airports[i].IATA] = &m.Airports[i]
	}
	for _, f := range m.Flights {
		e.byFlight[f.Carrier+f.Number] = f
	}
	return e
}

type inMeta struct {
	peer string
	kind string
}

// rememberCap bounds the inbound-origin memory. Relays happen within
// milliseconds of the inbound they forward, so a few thousand entries is
// hours of margin.
const rememberCap = 4096

// OnMessage folds one switch-bus message event into the traffic picture.
//
// The payload is the gateway's own message view; only the envelope matters
// here. Pulses are emitted raw and the browser thins them -- at demo rates
// that is fine, and at world rates the Eye aggregates before this point.
func (e *Eye) OnMessage(payload map[string]any) {
	id, _ := payload["id"].(string)
	peer, _ := payload["peer"].(string)
	dir, _ := payload["direction"].(string)
	kind, _ := payload["kind"].(string)
	corr, _ := payload["correlation_id"].(string)

	e.mu.Lock()
	e.messages++
	if peer != "" {
		e.peerWindow[peer]++
	}
	var flowSrc, flowDst, flowKind string
	switch dir {
	case "in":
		if id != "" {
			e.inboundSrc[id] = inMeta{peer: peer, kind: strings.SplitN(kind, "/", 2)[0]}
			e.inboundFIFO = append(e.inboundFIFO, id)
			if len(e.inboundFIFO) > rememberCap {
				delete(e.inboundSrc, e.inboundFIFO[0])
				e.inboundFIFO = e.inboundFIFO[1:]
			}
		}
	case "out":
		if corr != "" {
			if src, ok := e.inboundSrc[corr]; ok && src.peer != peer {
				e.edgeWindow[src.peer+">"+peer]++
				flowSrc, flowDst, flowKind = src.peer, peer, src.kind
			}
		}
	}
	now := time.Now()
	if now.After(e.pulseReset) {
		e.pulseBudget, e.flowBudget, e.pulseReset = 120, 80, now.Add(time.Second)
	}
	if e.pulseBudget <= 0 {
		e.mu.Unlock()
		return
	}
	e.pulseBudget--
	// The flow budget rides the pulse budget: sampled conversations for the
	// web, full counts for the weights.
	sendFlow := flowSrc != "" && e.flowBudget > 0
	if sendFlow {
		e.flowBudget--
	}
	e.mu.Unlock()
	e.broadcast(map[string]any{
		"t":    "pulse",
		"peer": payload["peer"], "dir": payload["direction"],
		"kind": strings.SplitN(kind, "/", 2)[0], "fmt": payload["format"],
	})
	if sendFlow {
		e.broadcast(map[string]any{"t": "flow", "src": flowSrc, "dst": flowDst, "kind": flowKind})
	}
}

// LogicalEdges snapshots the current window's conversation counts, for the
// tests: the web must be derivable, not decorative.
func (e *Eye) LogicalEdges() map[string]int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make(map[string]int64, len(e.edgeWindow))
	for k, v := range e.edgeWindow {
		out[k] = v
	}
	return out
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

// Airborne reports how many aircraft the Eye currently believes are flying.
func (e *Eye) Airborne() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.planes)
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
	kind, _ := payload["kind"].(string)

	if kind == "DIV" {
		ea, _ := payload["ea_airport"].(string)
		e.divert(flight, ea)
		return
	}

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
			Arriving: now.Add(time.Duration(f.BlockMin) * time.Minute / time.Duration(e.warpNow())),
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

// divert reroutes an airborne aircraft to the airport its DIV names.
//
// The aircraft continues from wherever it is now: its origin becomes its
// present position, its destination the diversion airport, and its new
// arrival falls out of the remaining great-circle distance at the same
// warped groundspeed everything else flies.
func (e *Eye) divert(flight, toApt string) {
	e.mu.Lock()
	p := e.planes[flight]
	div, ok := e.airports[toApt]
	if p == nil || !ok {
		e.mu.Unlock()
		return
	}
	now := time.Now()
	frac := float64(now.Sub(p.Departed)) / float64(p.Arriving.Sub(p.Departed))
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	curLat := p.FromLat + (p.ToLat-p.FromLat)*frac
	curLon := p.FromLon + (p.ToLon-p.FromLon)*frac
	km := haversineKM(curLat, curLon, div.Lat, div.Lon)
	p.FromLat, p.FromLon = curLat, curLon
	p.To, p.ToLat, p.ToLon = toApt, div.Lat, div.Lon
	p.Departed = now
	p.Arriving = now.Add(time.Duration(km/820*60) * time.Minute / time.Duration(e.warpNow()))
	p.Diverted = true
	cp := *p
	e.mu.Unlock()
	e.broadcast(map[string]any{"t": "dep", "plane": &cp})
}

// PlanesTo lists the aircraft currently flying toward an airport, for the
// simulation to divert when it closes.
func (e *Eye) PlanesTo(iata string) []Plane {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []Plane
	for _, p := range e.planes {
		if p.To == iata {
			out = append(out, *p)
		}
	}
	return out
}

func haversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * r * math.Asin(math.Sqrt(a))
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

// coastlineJSON is the Natural Earth 1:110m coastline (public domain),
// vendored the same way the OpenFlights data is: polylines of rounded
// lon/lat pairs, 76KB of planet. The traffic draws the cities; this draws
// the shore they sit on.
//
//go:embed coastline.json
var coastlineJSON []byte

// Routes mounts the Eye on a mux.
func (e *Eye) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /eye", e.page)
	mux.HandleFunc("GET /eye/land.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Write(coastlineJSON) //nolint:errcheck
	})
	mux.HandleFunc("GET /eye/world.json", e.world)
	mux.HandleFunc("GET /eye/planes.json", e.planesNow)
	mux.HandleFunc("GET /eye/stream", e.stream)
	mux.HandleFunc("POST /eye/chaos", e.chaos)
	mux.HandleFunc("GET /eye/flight/{flight}", e.flightRecords)
	mux.HandleFunc("POST /eye/time", e.time)
}

// time is the day's speed control: pause it, run it, race it.
func (e *Eye) time(w http.ResponseWriter, r *http.Request) {
	if e.SetWarp == nil {
		http.Error(w, "this world has no time control", http.StatusNotImplemented)
		return
	}
	var req struct {
		Warp int `json:"warp"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&req); err != nil {
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}
	if err := e.SetWarp(req.Warp); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"warp":%d}`, req.Warp) //nolint:errcheck
}

// flightRecords answers the aircraft drill-through.
func (e *Eye) flightRecords(w http.ResponseWriter, r *http.Request) {
	if e.FlightPNRs == nil {
		http.Error(w, "this world has no record hook", http.StatusNotImplemented)
		return
	}
	recs := e.FlightPNRs(strings.ToUpper(strings.TrimSpace(r.PathValue("flight"))))
	if recs == nil {
		recs = []FlightRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(recs) //nolint:errcheck
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
		"warp":     e.warpNow(),
		"hubs":     e.Hubs,
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
			for k, p := range e.planes {
				if time.Since(p.Arriving) > 2*time.Minute {
					delete(e.planes, k)
				}
			}
			closed := make(map[string]int64, len(e.closed))
			for a := range e.closed {
				closed[a] = e.halos[a]
			}
			rates := e.peerWindow
			e.peerWindow = map[string]int64{}
			edges := e.edgeWindow
			e.edgeWindow = map[string]int64{}
			if len(edges) > 3000 {
				// A safety valve, not a curator: the whole world's logical
				// graph is ~2,600 edges and the client wants all of it --
				// culling here made the carrier web flicker, because hub and
				// partner relay counts tie and the cut fell arbitrarily.
				type kv struct {
					k string
					v int64
				}
				all := make([]kv, 0, len(edges))
				for k, v := range edges {
					all = append(all, kv{k, v})
				}
				sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
				edges = map[string]int64{}
				for _, e2 := range all[:3000] {
					edges[e2.k] = e2.v
				}
			}
			ev := map[string]any{
				"t": "stats", "messages": e.messages, "movements": e.movements,
				"bookings": e.bookings, "airborne": len(e.planes), "closed": closed,
				"rates": rates, "edges": edges,
			}
			if e.WarpNow != nil {
				ev["warp"] = e.WarpNow()
			}
			if e.SimPos != nil {
				m := int(e.SimPos())
				ev["sim"] = fmt.Sprintf("%02d:%02d", m/60, m%60)
			}
			b, _ := json.Marshal(ev)
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
