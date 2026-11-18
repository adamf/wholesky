package airline

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
)

// Operations is the optional half of World that shows a carrier's people
// and its operations centre: the crews and their agreements, the day's
// signals, and projections of what a disruption will cost.
type Operations interface {
	Crew(carrier string) (any, bool)
	CrewMember(carrier, id string) (any, bool)
	NOC(carrier string) (any, bool)
	// Project plays a scenario the request describes (or the operations
	// centre's own forecast) under the policies and returns the spread.
	Project(ctx context.Context, carrier string, req json.RawMessage) (any, error)
}

// Economics is what a world with an economy tells about a carrier's
// money: its fuel and fleet, and what an aircraft does on the ground
// before a departure.
type Economics interface {
	Economy(carrier string) (any, bool)
	Turn(carrier, flight, board string) (any, bool)
}

func (s *Server) economy(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	code := strings.ToUpper(r.PathValue("carrier"))
	e, ok := s.view().(Economics)
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world keeps no economy"))
		return
	}
	v, ok := e.Economy(code)
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no economy for %s here", code))
		return
	}
	writeJSON(w, v)
}

func (s *Server) turn(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	code := strings.ToUpper(r.PathValue("carrier"))
	e, ok := s.view().(Economics)
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world keeps no economy"))
		return
	}
	v, ok := e.Turn(code, r.URL.Query().Get("flight"), r.URL.Query().Get("board"))
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no departure %s from %s", r.URL.Query().Get("flight"), r.URL.Query().Get("board")))
		return
	}
	writeJSON(w, v)
}

func (s *Server) operations() (Operations, bool) {
	o, ok := s.view().(Operations)
	return o, ok
}

func (s *Server) operationsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /carrier/{carrier}/crew", s.crew)
	mux.HandleFunc("GET /carrier/{carrier}/crew/{id}", s.crewMember)
	mux.HandleFunc("GET /carrier/{carrier}/noc", s.nocState)
	mux.HandleFunc("POST /carrier/{carrier}/noc/project", s.nocProject)
	mux.HandleFunc("GET /noc/{carrier}", s.nocPage)
	mux.HandleFunc("GET /carrier/{carrier}/economy", s.economy)
	mux.HandleFunc("GET /carrier/{carrier}/turn", s.turn)
}

func (s *Server) crew(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	o, ok := s.operations()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world keeps no rosters"))
		return
	}
	v, ok := o.Crew(strings.ToUpper(r.PathValue("carrier")))
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no roster for %s here", strings.ToUpper(r.PathValue("carrier"))))
		return
	}
	writeJSON(w, v)
}

func (s *Server) crewMember(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	o, ok := s.operations()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world keeps no rosters"))
		return
	}
	v, ok := o.CrewMember(strings.ToUpper(r.PathValue("carrier")), r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no crew member %s", r.PathValue("id")))
		return
	}
	writeJSON(w, v)
}

func (s *Server) nocState(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	o, ok := s.operations()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world has no operations centres"))
		return
	}
	v, ok := o.NOC(strings.ToUpper(r.PathValue("carrier")))
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("no operations centre for %s here", strings.ToUpper(r.PathValue("carrier"))))
		return
	}
	writeJSON(w, v)
}

func (s *Server) nocProject(w http.ResponseWriter, r *http.Request) {
	if s.forwarded(w, r) {
		return
	}
	o, ok := s.operations()
	if !ok {
		fail(w, http.StatusNotFound, fmt.Errorf("this world has no operations centres"))
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&raw); err != nil {
		raw = nil
	}
	v, err := o.Project(r.Context(), strings.ToUpper(r.PathValue("carrier")), raw)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, v)
}

func (s *Server) nocPage(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(r.PathValue("carrier"))
	if !codeRe.MatchString(code) {
		http.NotFound(w, r)
		return
	}
	pageHeaders(w)
	w.Write([]byte(strings.ReplaceAll(nocHTML, "{{CODE}}", html.EscapeString(code)))) //nolint:errcheck
}
