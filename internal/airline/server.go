package airline

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
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
	// this world.
	External bool `json:"external,omitempty"`
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

// Rank is the composite the leaderboard sorts by.
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
	s.Score = 100*s.Margin + 50*s.OTP - 2*float64(s.Cancelled)
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
	Reason     string  `json:"reason,omitempty"`
}

// Actions a seat can take, with what each needs.
var Actions = []struct {
	Kind, Needs, About string
}{
	{"cancel", "flight, board, reason", "cancel the departure: ASM CNL to distribution, the airport told, the flight plan withdrawn; IROPS reprotects the passengers"},
	{"retime", "flight, board, minutes", "announce a delay to distribution as an ASM TIM and move the bookings to the new times"},
	{"substitute", "flight, board", "swap the aircraft for a smaller type: the cabin is re-seated, distribution hears the EQT"},
	{"class", "flight, board, class, status", "force a booking class on the departure closed (C) or back to the ladder (empty status)"},
	{"fares", "multiplier", "move every fare the carrier files by the factor: 0.9 is a sale, 1.2 a premium, 0 the filing"},
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
	Reg   *Registry
	World World
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

func (s *Server) carriers(w http.ResponseWriter, r *http.Request) {
	list := s.view().Carriers()
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
	known := false
	code := strings.ToUpper(r.PathValue("carrier"))
	for _, c := range s.view().Carriers() {
		if c.Code == code {
			known = true
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

func (s *Server) lobbyPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(lobbyHTML)) //nolint:errcheck
}

func (s *Server) opsPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(strings.ReplaceAll(opsHTML, "{{CARRIER}}", strings.ToUpper(r.PathValue("carrier"))))) //nolint:errcheck
}
