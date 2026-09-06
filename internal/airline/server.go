package airline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CarrierInfo is one carrier as the lobby lists it.
type CarrierInfo struct {
	Code    string    `json:"code"`
	Name    string    `json:"name"`
	Hub     string    `json:"hub"`
	Flights int       `json:"flights"`
	Seat    *Seat     `json:"seat,omitempty"`
	Score   Scorecard `json:"score"`
	// External says someone's own jetway node flies the carrier, not
	// this world. World names the joined world a carrier belongs to, when
	// it is not this one.
	External bool   `json:"external,omitempty"`
	World    string `json:"world,omitempty"`
}

// FlightState is one of the carrier's flights today as the seat sees it.
type FlightState struct {
	Flight string `json:"flight"`
	From   string `json:"from"`
	To     string `json:"to"`
	STD    string `json:"std"`
	ETD    string `json:"etd"`
	STA    string `json:"sta"`
	// DelayMin is the departure delay the day holds for the flight; Status
	// is where it is: scheduled, open (check-in), boarding, departed,
	// landed, cancelled.
	DelayMin int    `json:"delay_min"`
	Status   string `json:"status"`
	Tail     string `json:"tail,omitempty"`
	Type     string `json:"type,omitempty"`
	Seats    int    `json:"seats"`
	Booked   int    `json:"booked"`
	Boarded  int    `json:"boarded,omitempty"`
	Revenue  int64  `json:"revenue"`
	// The day's account of the flight, as the panel has it.
	Delay       string `json:"delay,omitempty"`
	Slot        string `json:"slot,omitempty"`
	Crew        string `json:"crew,omitempty"`
	Retimed     string `json:"retimed,omitempty"`
	Substituted string `json:"substituted,omitempty"`
	Rushed      string `json:"rushed,omitempty"`
	Cancelled   string `json:"cancelled,omitempty"`
}

// Scorecard is how a carrier's day is going, in money and punctuality.
// Costs are the world's own shape of an airline's cost base -- block hours
// by aircraft class, delay minutes, cancellations, reserve callouts, bags
// mishandled -- and are labelled as such; the point is the comparison
// between carriers run by people and carriers run by the autopilot.
type Scorecard struct {
	Carrier    string           `json:"carrier"`
	Flights    int              `json:"flights"`
	Flown      int              `json:"flown"`
	Cancelled  int              `json:"cancelled"`
	Remaining  int              `json:"remaining"`
	OnTime     int              `json:"on_time"`
	OTP        float64          `json:"otp"`
	DelayMin   int              `json:"delay_min"`
	Passengers int              `json:"passengers"`
	Seats      int              `json:"seats"`
	LoadFactor float64          `json:"load_factor"`
	Revenue    int64            `json:"revenue"`
	Cost       int64            `json:"cost"`
	Profit     int64            `json:"profit"`
	Margin     float64          `json:"margin"`
	Costs      map[string]int64 `json:"costs"`
	// Score is the composite the leaderboard ranks by: margin points plus
	// punctuality points less a penalty per cancellation.
	Score float64 `json:"score"`
	Bags  struct {
		Rushed, Mishandled int
	} `json:"bags"`
	Slots     int `json:"slots"`
	Reserves  int `json:"reserves"`
	Decisions int `json:"decisions"`
	Defaulted int `json:"defaulted"`
}

// Rank fills the derived figures. The composite is set by
// RankAgainstTheWorld once every carrier is in hand, because a day's
// revenue only means something against the day's: an unfilled world
// leaves every carrier in the red and a two-flight carrier with one
// lucky sale on top.
func (s *Scorecard) Rank() {
	s.Profit = s.Revenue - s.Cost
	if s.Revenue > 0 {
		s.Margin = float64(s.Profit) / float64(s.Revenue)
	}
	if s.Flown > 0 {
		s.OTP = float64(s.OnTime) / float64(s.Flown)
	}
	if s.Seats > 0 {
		s.LoadFactor = float64(s.Passengers) / float64(s.Seats)
	}
	s.Score = 50*s.OTP + 50*s.completion() - 2*float64(s.Cancelled)
}

// completion is the share of the carrier's day that flew or will.
func (s *Scorecard) completion() float64 {
	if s.Flights == 0 {
		return 0
	}
	return 1 - float64(s.Cancelled)/float64(s.Flights)
}

// RankAgainstTheWorld sets every carrier's composite against the whole
// table: punctuality and completion as before, plus how the carrier's
// revenue per seat flown stands against the world's, worth up to fifty
// points either way, and a small weight for having flown at all so a
// two-flight carrier cannot lead five hundred.
func RankAgainstTheWorld(rows []CarrierInfo) {
	var rev, seats int64
	for _, c := range rows {
		rev += c.Score.Revenue
		seats += int64(c.Score.Seats)
	}
	world := 0.0
	if seats > 0 {
		world = float64(rev) / float64(seats)
	}
	for i := range rows {
		sc := &rows[i].Score
		rel := 0.0
		if world > 0 && sc.Seats > 0 {
			rel = float64(sc.Revenue)/float64(sc.Seats)/world - 1
			rel = math.Max(-1, math.Min(1, rel))
		}
		size := math.Min(1, float64(sc.Flown)/20) // full weight from twenty flights flown
		sc.Score = size*(50*sc.OTP+50*sc.completion()+50*rel) - 2*float64(sc.Cancelled)
	}
}

// Action is a lever pulled.
type Action struct {
	Kind string `json:"kind"`
	// Flight and Board name the departure most actions act on.
	Flight string `json:"flight,omitempty"`
	Board  string `json:"board,omitempty"`
	// Minutes is a retime's delay; Class and Status a booking class and
	// the status to force it to (closed "C", open "" ); Multiplier the
	// fares' factor over the filing; Reason free text for the record.
	Minutes    int     `json:"minutes,omitempty"`
	Class      string  `json:"class,omitempty"`
	Status     string  `json:"status,omitempty"`
	Multiplier float64 `json:"multiplier,omitempty"`
	// From and To name a market for the fares action; empty means every
	// market the carrier files.
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Actions a seat can take, with what each needs.
var Actions = []struct {
	Kind, Needs, About string
}{
	{"cancel", "flight, board, reason", "cancel the departure: ASM CNL to distribution, the airport told, the flight plan withdrawn; IROPS reprotects the passengers"},
	{"retime", "flight, board, minutes", "announce a delay to distribution as an ASM TIM and move the bookings to the new times"},
	{"substitute", "flight, board", "swap the aircraft for a smaller type: the cabin is re-seated, distribution hears the EQT"},
	{"class", "flight, board, class, status", "force a booking class on the departure closed (C) or back to the ladder (empty status)"},
	{"fares", "multiplier, optionally from and to", "move the carrier's fares by the factor -- every market, or one when from and to name it: 0.9 is a sale, 1.2 a premium, 0 the filing. Travellers shop: fares above the filing lose buyers"},
	{"ready", "flight, board", "tell the Network Manager the flight is ready (REA) and ask for a slot improvement"},
	{"reserves", "flight, board", "call a reserve crew for a flight whose crew has timed out, instead of cancelling it"},
}

// World is what the simulation gives the seats to act on.
type World interface {
	Carriers() []CarrierInfo
	Flights(carrier string) []FlightState
	Score(carrier string) Scorecard
	Act(ctx context.Context, carrier string, a Action) (string, error)
	// Clock is the day's position in minutes and the warp.
	Clock() (pos float64, warp int)
}

// Server is the seats' HTTP surface.
type Server struct {
	// streams counts the open event streams, capped at maxStreams.
	streams atomic.Int32
	Reg     *Registry
	World   World
	// world, when set by SetWorld, replaces World: a federating core swaps
	// its view in after boot, while requests may already be arriving.
	worldMu sync.RWMutex
	world   World
	// Local says whether this machine runs the carrier; Proxy, when set,
	// forwards a request for one it does not to the machine that does. A
	// federated world's core runs no carriers and forwards everything;
	// its lobby merges the peers' lists.
	Local func(carrier string) bool
	Proxy func(w http.ResponseWriter, r *http.Request, carrier string) bool
	// OnRelease, when set, is told when a seat is released, so the world
	// can take back anything the seat had claimed.
	OnRelease func(carrier string)
	// Recordings, when set, holds finished runs beyond this process: the
	// world's disk, or the core's. The registry's own memory is asked first.
	Recordings RecordingStore
}

// RecordingStore keeps finished runs where a restart does not reach.
type RecordingStore interface {
	Recording(id string) (*Recording, bool)
	Recordings() []Summary
}

// SetWorld replaces the world the server answers from.
func (s *Server) SetWorld(w World) {
	s.worldMu.Lock()
	defer s.worldMu.Unlock()
	s.world = w
}

// view is the world in force.
func (s *Server) view() World {
	s.worldMu.RLock()
	defer s.worldMu.RUnlock()
	if s.world != nil {
		return s.world
	}
	return s.World
}

// forwarded handles a carrier-scoped request that belongs to another
// machine; true when it did.
func (s *Server) forwarded(w http.ResponseWriter, r *http.Request) bool {
	code := strings.ToUpper(r.PathValue("carrier"))
	if s.Local == nil || s.Proxy == nil || s.Local(code) {
		return false
	}
	return s.Proxy(w, r, code)
}

// Routes mounts the API and the pages.
func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ops/{$}", s.lobbyPage)
	mux.HandleFunc("GET /ops/{carrier}", s.opsPage)
	mux.HandleFunc("GET /carriers.json", s.carriers)
	mux.HandleFunc("GET /carrier/{carrier}/state", s.state)
	mux.HandleFunc("GET /carrier/{carrier}/inbox", s.inbox)
	mux.HandleFunc("GET /carrier/{carrier}/events", s.events)
	mux.HandleFunc("GET /carrier/{carrier}/tape", s.tape)
	mux.HandleFunc("POST /carrier/{carrier}/take", s.take)
	mux.HandleFunc("POST /carrier/{carrier}/release", s.release)
	mux.HandleFunc("POST /carrier/{carrier}/departments", s.departments)
	mux.HandleFunc("POST /carrier/{carrier}/decide", s.decide)
	mux.HandleFunc("POST /carrier/{carrier}/act", s.act)
	// The flight recorder: the seat's own words onto the tape, the run so
	// far, the runs kept, and the page that plays one back.
	mux.HandleFunc("POST /carrier/{carrier}/note", s.note)
	mux.HandleFunc("GET /carrier/{carrier}/recording.json", s.liveRecording)
	mux.HandleFunc("GET /recordings.json", s.recordings)
	mux.HandleFunc("GET /recording/{id}", s.recording)
	mux.HandleFunc("GET /replay/{id}", s.replayPage)
}

// RecordScores samples every held seat's scorecard into its recording,
// for the life of the world.
func (s *Server) RecordScores(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			for _, seat := range s.Reg.Seats() {
				if s.Local != nil && !s.Local(seat.Carrier) {
					continue
				}
				s.Reg.Sample(seat.Carrier, s.view().Score(seat.Carrier))
			}
		}
	}
}

func (s *Server) note(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("malformed request: %w", err))
		return
	}
	if err := s.Reg.Note(r.PathValue("carrier"), s.token(r), req.Text); err != nil {
		code := http.StatusForbidden
		if !errors.Is(err, ErrNotHeld) {
			code = http.StatusBadRequest
		}
		fail(w, code, err)
		return
	}
	writeJSON(w, map[string]string{"ok": "noted"})
}

func (s *Server) liveRecording(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	rec, ok := s.Reg.LiveRecording(r.PathValue("carrier"))
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("nobody holds %s; a run is recorded while a seat is held", strings.ToUpper(r.PathValue("carrier"))))
		return
	}
	writeJSON(w, rec)
}

func (s *Server) recordings(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	var out []Summary
	for _, sum := range s.Reg.Recordings() {
		seen[sum.ID] = true
		out = append(out, sum)
	}
	if s.Recordings != nil {
		for _, sum := range s.Recordings.Recordings() {
			if !seen[sum.ID] {
				out = append(out, sum)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	if len(out) > 200 {
		out = out[:200]
	}
	writeJSON(w, map[string]any{"recordings": out})
}

func (s *Server) recording(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".json")
	if !ValidRecordingID(id) {
		http.NotFound(w, r)
		return
	}
	if rec, ok := s.Reg.Recording(id); ok {
		writeJSON(w, rec)
		return
	}
	if s.Recordings != nil {
		if rec, ok := s.Recordings.Recording(id); ok {
			writeJSON(w, rec)
			return
		}
	}
	http.NotFound(w, r)
}

// replayPage plays a run back: by recording id, or a held seat's live by
// carrier code. Anything else is not a page.
func (s *Server) replayPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	title := id
	switch {
	case ValidRecordingID(id):
	case codeRe.MatchString(strings.ToUpper(id)):
		id = strings.ToUpper(id)
		title = id
	default:
		http.NotFound(w, r)
		return
	}
	pageHeaders(w)
	page := strings.ReplaceAll(replayHTML, "{{ID}}", html.EscapeString(id))
	page = strings.ReplaceAll(page, "{{TITLE}}", html.EscapeString(title))
	w.Write([]byte(page)) //nolint:errcheck
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func fail(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
}

func (s *Server) token(r *http.Request) string {
	if t := r.Header.Get("X-Seat-Token"); t != "" {
		return t
	}
	return r.URL.Query().Get("token")
}

// HasCarrier is the optional half of World that answers whether a carrier
// exists without building the whole lobby, which a take must not wait for.
type HasCarrier interface {
	Has(code string) bool
}

// OwnCarriers is the optional half of World a federating lobby needs: this
// world's carriers alone, without the joined worlds' rows, so two worlds
// asking each other for their lobbies do not ask forever.
type OwnCarriers interface {
	OwnCarriers() []CarrierInfo
}

func (s *Server) carriers(w http.ResponseWriter, r *http.Request) {
	var list []CarrierInfo
	if own, ok := s.view().(OwnCarriers); ok && r.URL.Query().Get("own") == "1" {
		list = own.OwnCarriers()
	} else {
		list = s.view().Carriers()
	}
	for i := range list {
		if seat, ok := s.Reg.Seat(list[i].Code); ok {
			list[i].Seat = &seat
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Score.Score != list[j].Score.Score {
			return list[i].Score.Score > list[j].Score.Score
		}
		return list[i].Code < list[j].Code
	})
	pos, warp := s.view().Clock()
	writeJSON(w, map[string]any{"carriers": list, "pos": pos, "warp": warp, "departments": Departments, "actions": Actions})
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	code := strings.ToUpper(r.PathValue("carrier"))
	var seatp *Seat
	if seat, ok := s.Reg.Seat(code); ok {
		seatp = &seat
	}
	pos, warp := s.view().Clock()
	writeJSON(w, map[string]any{
		"carrier": code, "seat": seatp, "pos": pos, "warp": warp,
		"score": s.view().Score(code), "flights": s.view().Flights(code),
		"inbox": s.Reg.Inbox(code), "departments": Departments,
	})
}

func (s *Server) inbox(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	writeJSON(w, s.Reg.Inbox(r.PathValue("carrier")))
}

func (s *Server) tape(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	writeJSON(w, s.Reg.Tape(r.PathValue("carrier")))
}

func (s *Server) take(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	var req struct {
		Holder string `json:"holder"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("malformed request: %w", err))
		return
	}
	code := strings.ToUpper(r.PathValue("carrier"))
	known := false
	if h, ok := s.view().(HasCarrier); ok {
		known = h.Has(code)
	} else {
		for _, c := range s.view().Carriers() {
			if c.Code == code {
				known = true
			}
		}
	}
	if !known {
		fail(w, http.StatusNotFound, fmt.Errorf("no carrier %s in this world", code))
		return
	}
	seat, token, err := s.Reg.Take(code, req.Holder)
	if err != nil {
		fail(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, map[string]any{"seat": seat, "token": token})
}

func (s *Server) release(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	if err := s.Reg.Release(r.PathValue("carrier"), s.token(r)); err != nil {
		fail(w, http.StatusForbidden, err)
		return
	}
	if s.OnRelease != nil {
		s.OnRelease(strings.ToUpper(r.PathValue("carrier")))
	}
	writeJSON(w, map[string]string{"ok": "released"})
}

func (s *Server) departments(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	var req struct {
		Department string `json:"department"`
		Manual     bool   `json:"manual"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("malformed request: %w", err))
		return
	}
	if err := s.Reg.SetManual(r.PathValue("carrier"), s.token(r), req.Department, req.Manual); err != nil {
		code := http.StatusBadRequest
		if err == ErrNotHeld {
			code = http.StatusForbidden
		}
		fail(w, code, err)
		return
	}
	seat, _ := s.Reg.Seat(r.PathValue("carrier"))
	writeJSON(w, seat)
}

func (s *Server) decide(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	var req struct {
		ID     string `json:"id"`
		Option string `json:"option"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("malformed request: %w", err))
		return
	}
	if err := s.Reg.Answer(r.PathValue("carrier"), s.token(r), req.ID, req.Option); err != nil {
		code := http.StatusBadRequest
		if err == ErrNotHeld {
			code = http.StatusForbidden
		}
		fail(w, code, err)
		return
	}
	writeJSON(w, map[string]string{"ok": "decided", "id": req.ID, "option": req.Option})
}

func (s *Server) act(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	code := strings.ToUpper(r.PathValue("carrier"))
	if !s.Reg.Authorised(code, s.token(r)) {
		fail(w, http.StatusForbidden, ErrNotHeld)
		return
	}
	var a Action
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&a); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("malformed request: %w", err))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := s.view().Act(ctx, code, a)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	s.Reg.Emit(code, "action", a.Kind+" "+a.Flight+": "+result, a)
	writeJSON(w, map[string]string{"ok": "done", "result": result})
}

// events streams a carrier's tape as server-sent events.
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if s.streams.Add(1) > maxStreams {
		s.streams.Add(-1)
		http.Error(w, "too many open streams", http.StatusServiceUnavailable)
		return
	}
	defer s.streams.Add(-1)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch, cancel := s.Reg.Subscribe(r.PathValue("carrier"))
	defer cancel()
	for _, e := range s.Reg.Tape(r.PathValue("carrier")) {
		b, _ := json.Marshal(e)
		fmt.Fprintf(w, "data: %s\n\n", b)
	}
	fl.Flush()
	keep := time.NewTicker(15 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			fl.Flush()
		case <-keep.C:
			fmt.Fprint(w, ": keep\n\n")
			fl.Flush()
		}
	}
}

// pageHeaders are what every page is served with: the scripts are the
// page's own, the fetches go to this origin, nobody frames it, and a
// mistyped response is never sniffed into something else.
func pageHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "same-origin")
}

// codeRe is the shape of a designator the page may reflect: two or three
// letters or digits. Anything else is not a carrier and never reaches the
// markup.
// maxStreams caps the event streams held open at once: a page reconnects,
// a script opening thousands does not get to.
const maxStreams = 256

var codeRe = regexp.MustCompile(`^[A-Z0-9]{2,3}$`)

func (s *Server) lobbyPage(w http.ResponseWriter, r *http.Request) {
	pageHeaders(w)
	w.Write([]byte(lobbyHTML)) //nolint:errcheck
}

func (s *Server) opsPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("carrier"))
	if !codeRe.MatchString(code) {
		http.NotFound(w, r)
		return
	}
	pageHeaders(w)
	w.Write([]byte(strings.ReplaceAll(opsHTML, "{{CARRIER}}", html.EscapeString(code)))) //nolint:errcheck
}
