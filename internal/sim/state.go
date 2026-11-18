package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/distribution"
	"github.com/adamf/wholesky/internal/fuel"
	"github.com/adamf/wholesky/internal/growth"
)

// What a restart must not forget. A world's books start each day clean by
// design, but the people in it do not: who holds which carrier, which
// carriers have been handed to their own nodes and where those nodes are,
// and which worlds this one has joined. The state is one JSON file,
// written whole after every change (debounced) and read back at boot;
// each machine keeps its own, since seats live where the tenant does.

// worldState is the file.
type worldState struct {
	SavedAt time.Time `json:"saved_at"`
	// Peers are the peer machines' own states, kept here for them.
	Peers    map[string]json.RawMessage `json:"peers,omitempty"`
	Seats    []airline.SeatState        `json:"seats"`
	External []string                   `json:"external"`
	Nodes    map[string]string          `json:"nodes"`
	// Worlds are the joined worlds' hellos, with the side this world took.
	Worlds []savedWorld `json:"worlds"`
	// Economy is the calendar and what was done to the market: the day the
	// world has reached, the shocks added, and the hedges carriers hold.
	Economy *savedEconomy `json:"economy,omitempty"`
	// Fares is every fare multiplier filed: by carrier, and by carrier,
	// origin and destination.
	Fares *savedFares `json:"fares,omitempty"`
	// Policies is how each carrier sells, where somebody has changed it.
	Policies map[string]distribution.Policy `json:"policies,omitempty"`
}

type savedFares struct {
	Carriers map[string]float64 `json:"carriers,omitempty"`
	Markets  map[string]float64 `json:"markets,omitempty"`
}

// faresNow is the fares filed, for the state file and for a machine that
// joins late.
func (s *Sim) faresNow() *savedFares {
	if s.tariff == nil {
		return nil
	}
	c, m := s.tariff.Multipliers()
	if len(c) == 0 && len(m) == 0 {
		return nil
	}
	return &savedFares{Carriers: c, Markets: m}
}

// loadFares puts saved or received fares on this machine's tariff.
func (s *Sim) loadFares(f *savedFares) {
	if f == nil || s.tariff == nil {
		return
	}
	for code, m := range f.Carriers {
		s.tariff.SetMultiplier(code, m)
	}
	for k, m := range f.Markets {
		if p := strings.Split(k, "/"); len(p) == 3 {
			s.tariff.SetMarketMultiplier(p[0], p[1], p[2], m)
		}
	}
}

type savedEconomy struct {
	Day    int64                   `json:"day"`
	Shocks []fuel.Shock            `json:"shocks,omitempty"`
	Hedges map[string][]fuel.Hedge `json:"hedges,omitempty"`
	// Books is each carrier's money across days and its orders.
	Books map[string]*growth.Book `json:"books,omitempty"`
}

type savedWorld struct {
	Hello     worldHello `json:"hello"`
	Accepting bool       `json:"accepting"`
}

// stateKeeper debounces writes and serialises them. A keeper with a path
// writes a file; one with a core writes its state to the core's file under
// its own name (a region has no volume of its own); a core keeps its peers'
// blobs in its file alongside its own state.
type stateKeeper struct {
	path    string
	coreURL string
	name    string
	mu      sync.Mutex
	timer   *time.Timer
	peers   map[string]json.RawMessage
}

// PeerState is what a peer machine keeps at the core.
func (s *Sim) peerState(name string) (json.RawMessage, bool) {
	if s.state == nil {
		return nil, false
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	b, ok := s.state.peers[name]
	return b, ok
}

// setPeerState stores a peer's blob and schedules a write.
func (s *Sim) setPeerState(name string, b json.RawMessage) {
	if s.state == nil {
		return
	}
	s.state.mu.Lock()
	if s.state.peers == nil {
		s.state.peers = map[string]json.RawMessage{}
	}
	if _, known := s.state.peers[name]; !known && len(s.state.peers) >= maxStatePeers {
		s.state.mu.Unlock()
		s.log.Warn("state for an unknown peer refused: the table is full", "peer", name)
		return
	}
	s.state.peers[name] = b
	s.state.mu.Unlock()
	s.saveState()
}

// maxStatePeers bounds the peers whose state a core keeps: a world has a
// handful of machines, not thousands.
const maxStatePeers = 64

// servePeerState is GET and PUT /federation/state/{peer} on a core.
func (s *Sim) servePeerState(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("peer")
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<20))
		if err != nil || !json.Valid(b) {
			http.Error(w, "a peer's state is a JSON document", http.StatusBadRequest)
			return
		}
		s.setPeerState(name, b)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	b, ok := s.peerState(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b) //nolint:errcheck
}

// saveState schedules a write of the state file, soon; nothing without a
// path.
func (s *Sim) saveState() {
	if s.state == nil {
		return
	}
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if s.state.timer != nil {
		return
	}
	s.state.timer = time.AfterFunc(500*time.Millisecond, func() {
		s.state.mu.Lock()
		s.state.timer = nil
		s.state.mu.Unlock()
		if err := s.writeState(); err != nil {
			s.log.Warn("state not saved", "path", s.state.path, "err", err)
		}
	})
}

// snapshot is the state now.
func (s *Sim) snapshot() worldState {
	st := worldState{SavedAt: time.Now(), Nodes: map[string]string{}}
	if s.Airline != nil {
		st.Seats = s.Airline.Snapshot()
	}
	s.externalMu.RLock()
	for code := range s.external {
		st.External = append(st.External, code)
	}
	for code, u := range s.nodeURL {
		st.Nodes[code] = u
	}
	s.externalMu.RUnlock()
	sort.Strings(st.External)
	s.foreignMu.RLock()
	for _, fw := range s.foreign {
		st.Worlds = append(st.Worlds, savedWorld{Hello: fw.Hello, Accepting: fw.accepting})
	}
	s.foreignMu.RUnlock()
	sort.Slice(st.Worlds, func(i, j int) bool { return st.Worlds[i].Hello.Code < st.Worlds[j].Hello.Code })
	if e := s.econ; e != nil {
		se := &savedEconomy{Day: e.day.Load(), Shocks: e.fuel.Shocks(), Hedges: map[string][]fuel.Hedge{}}
		e.mu.Lock()
		for code, hs := range e.hedges {
			// The hedges a carrier had before the world began are worked
			// out again at boot; only the ones taken since are kept.
			for _, h := range hs {
				if h.Bought >= 0 {
					se.Hedges[code] = append(se.Hedges[code], h)
				}
			}
		}
		if len(e.books) > 0 {
			se.Books = map[string]*growth.Book{}
			for code, b := range e.books {
				cp := *b
				cp.Days = append([]growth.Day(nil), b.Days...)
				cp.Orders = append([]growth.Order(nil), b.Orders...)
				se.Books[code] = &cp
			}
		}
		e.mu.Unlock()
		st.Economy = se
	}
	st.Fares = s.faresNow()
	if s.commerce != nil {
		s.commerce.mu.RLock()
		if len(s.commerce.policy) > 0 {
			st.Policies = map[string]distribution.Policy{}
			for code, p := range s.commerce.policy {
				st.Policies[code] = p
			}
		}
		s.commerce.mu.RUnlock()
	}
	return st
}

// writeState writes the state: to the file, atomically, and to the core
// where this machine keeps its state there.
func (s *Sim) writeState() error {
	st := s.snapshot()
	s.state.mu.Lock()
	if len(s.state.peers) > 0 {
		st.Peers = map[string]json.RawMessage{}
		for k, v := range s.state.peers {
			st.Peers[k] = v
		}
	}
	s.state.mu.Unlock()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if s.state.coreURL != "" && s.state.name != "" {
		req, err := http.NewRequest(http.MethodPut, strings.TrimRight(s.state.coreURL, "/")+"/federation/state/"+s.state.name, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(secretHeader, s.linkSecret)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	if s.state.path == "" {
		return nil
	}
	// The file holds every seat's token: the process's own to read.
	if err := os.MkdirAll(filepath.Dir(s.state.path), 0o700); err != nil {
		return err
	}
	tmp := s.state.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.state.path)
}

// readState reads the state back: the file, else the core's copy.
func (s *Sim) readState() ([]byte, error) {
	if s.state.path != "" {
		if b, err := os.ReadFile(s.state.path); err == nil {
			return b, nil
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if s.state.coreURL != "" && s.state.name != "" {
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest(http.MethodGet, strings.TrimRight(s.state.coreURL, "/")+"/federation/state/"+s.state.name, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set(secretHeader, s.linkSecret)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, os.ErrNotExist
		}
		return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	}
	return nil, os.ErrNotExist
}

// restoreState reads the file, if any, and puts the world back the way the
// people left it: seats first, then the carriers handed to nodes (their
// tenants severed and the tokens demanded again), the nodes' URLs, and
// the joined worlds -- the dialling side dials again with the token it
// used; the accepting side accepts it again.
func (s *Sim) restoreState(ctx context.Context) {
	if s.state == nil {
		return
	}
	b, err := s.readState()
	if err != nil {
		if !os.IsNotExist(err) {
			s.log.Warn("state not read", "path", s.state.path, "core", s.state.coreURL, "err", err)
		}
		return
	}
	var st worldState
	if err := json.Unmarshal(b, &st); err != nil {
		s.log.Warn("state unreadable; starting clean", "path", s.state.path, "err", err)
		return
	}
	if len(st.Peers) > 0 {
		s.state.mu.Lock()
		s.state.peers = st.Peers
		s.state.mu.Unlock()
	}
	if s.Airline != nil {
		s.Airline.Restore(st.Seats)
	}
	for _, code := range st.External {
		if _, ok := s.Tenants[code]; ok {
			if _, err := s.Claim(code); err != nil {
				s.log.Warn("claim not restored", "carrier", code, "err", err)
			}
		} else {
			s.externalMu.Lock()
			s.external[code] = true
			s.externalMu.Unlock()
		}
	}
	for code, u := range st.Nodes {
		s.SetNodeURL(code, u)
	}
	s.loadFares(st.Fares)
	if s.commerce != nil && len(st.Policies) > 0 {
		s.commerce.mu.Lock()
		for code, p := range st.Policies {
			s.commerce.policy[code] = p
		}
		s.commerce.mu.Unlock()
	}
	if e := s.econ; e != nil && st.Economy != nil {
		e.day.Store(st.Economy.Day)
		for _, sh := range st.Economy.Shocks {
			e.fuel.AddShock(sh)
		}
		e.mu.Lock()
		for code, hs := range st.Economy.Hedges {
			if _, ok := e.fleets[code]; ok {
				e.hedges[code] = append(e.hedges[code], hs...)
			}
		}
		for code, b := range st.Economy.Books {
			if _, ok := e.fleets[code]; ok && b != nil {
				// An order that has come due is in the schedule this
				// world was built from.
				for i := range b.Orders {
					b.Orders[i].Flying = int64(b.Orders[i].Due) <= st.Economy.Day
				}
				e.books[code] = b
			}
		}
		e.mu.Unlock()
	}
	for _, w := range st.Worlds {
		// A world that said hello again while this one was booting is
		// already joined, with a fresher token than the file holds; the
		// file must not put the old one back.
		s.foreignMu.RLock()
		_, already := s.foreign[w.Hello.Code]
		s.foreignMu.RUnlock()
		if already {
			continue
		}
		if err := s.joinWorld(ctx, w.Hello, w.Accepting); err != nil {
			s.log.Warn("joined world not restored", "world", w.Hello.Name, "err", err)
		}
	}
	s.log.Info("state restored", "path", s.state.path, "seats", len(st.Seats), "external", len(st.External), "nodes", len(st.Nodes), "worlds", len(st.Worlds), "saved_at", st.SavedAt)
}
