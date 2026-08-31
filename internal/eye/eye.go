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

	mu     sync.Mutex
	planes map[string]*Plane
	subs   map[chan []byte]struct{}

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
	e.mu.Unlock()
	kind, _ := payload["kind"].(string)
	e.broadcast(map[string]any{
		"t":    "pulse",
		"peer": payload["peer"], "dir": payload["direction"],
		"kind": strings.SplitN(kind, "/", 2)[0], "fmt": payload["format"],
	})
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
			b, _ := json.Marshal(map[string]any{
				"t": "stats", "messages": e.messages, "movements": e.movements,
				"bookings": e.bookings, "airborne": len(e.planes),
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
