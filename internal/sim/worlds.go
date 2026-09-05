package sim

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/config"
	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"
	"github.com/adamf/wholesky/internal/world"
)

// Worlds joined to worlds. Two skyd instances -- two skies, each with its
// own carriers, distribution systems, switch and day -- become one network
// the way two real networks do: their switches hold a trunk, each routes
// the other's subscribers down it, and the distribution systems on either
// side sell the other side's flights. The join is a handshake over HTTP
// (POST /federation/world) that carries what each side needs to configure
// its switch and its sellers; the traffic itself is Type B and EDIFACT over
// the trunk, exactly as it is within one world. The carriers' designators
// must differ between the worlds -- a designator is an address -- and so
// must the distribution systems' cities (WorldCity) and the switches'
// codes (WorldCode).

// worldHello is what a world tells another about itself.
type worldHello struct {
	Name string `json:"name"`
	// Code and SwitchTTY identify the first switch; SwitchAddr is where a
	// peer's switch dials it; Token is what the dialling side presents.
	Code       string `json:"code"`
	SwitchTTY  string `json:"switch_tty"`
	SwitchAddr string `json:"switch_addr"`
	Token      string `json:"token,omitempty"`
	// Watcher is the address operational messages should be copied to,
	// so the peer's globe sees this world's aircraft too. URL is where the
	// peer fetches the manifest and answers back.
	Watcher  string          `json:"watcher"`
	URL      string          `json:"url"`
	Carriers []world.Carrier `json:"carriers"`
	GDS      []gdsInfo       `json:"gds"`
}

type gdsInfo struct {
	Designator string `json:"designator"`
	Address    string `json:"address"`
}

// foreignWorld is a world this one has joined, with what it flies: kept
// apart from this world's own schedule maps, which the day loop and the
// demand read without a lock, and read through the accessors below.
type foreignWorld struct {
	Hello    worldHello
	Flights  int
	JoinedAt time.Time
	carriers map[string]world.Carrier
	flights  map[string][]world.Flight
	byOrigin map[string][]world.Flight
}

// hello is this world as it introduces itself.
func (s *Sim) hello() worldHello {
	d, tty := switchIdentity(0, s.worldCode)
	h := worldHello{Name: s.worldName, Code: d, SwitchTTY: tty, Watcher: s.watcher(), URL: s.publicURL}
	if pub := strings.Split(s.publicSwitch, ","); pub[0] != "" {
		h.SwitchAddr = strings.TrimSpace(pub[0])
	} else if s.Switch != nil {
		h.SwitchAddr = s.Switch.Addr("link-net")
	}
	for _, c := range s.Manifest.Carriers {
		if !s.External(c.Designator) || true {
			h.Carriers = append(h.Carriers, world.Carrier{Designator: c.Designator, TTYAddress: c.TTYAddress, Format: c.Format, ICAO: c.ICAO, Name: c.Name, Hub: c.Hub})
		}
	}
	for _, g := range s.GDSes {
		h.GDS = append(h.GDS, gdsInfo{Designator: g.Designator, Address: g.Address})
	}
	return h
}

// serveManifest is GET /world/manifest.json: the compiled world, for a peer
// to sell its flights.
func (s *Sim) serveManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Manifest) //nolint:errcheck
}

// serveJoinWorld is POST /federation/world: another world asking to join.
// This side accepts the trunk the other will dial, routes the other's
// subscribers down it, tells its carriers to copy their movements to the
// other's watcher, learns the other's flights so its sellers can sell them,
// and answers with its own hello.
func (s *Sim) serveJoinWorld(w http.ResponseWriter, r *http.Request) {
	var h worldHello
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&h); err != nil || h.Code == "" || h.Token == "" {
		http.Error(w, "a world's hello names its switch and carries a token", http.StatusBadRequest)
		return
	}
	if err := s.joinWorld(r.Context(), h, true); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.hello()) //nolint:errcheck
}

// joinWorld wires another world in. accepting says this side accepts the
// trunk (the other dials); otherwise this side dials, with the token.
func (s *Sim) joinWorld(ctx context.Context, h worldHello, accepting bool) error {
	if s.Switch == nil {
		return fmt.Errorf("this machine runs no switch; join worlds from the core")
	}
	if h.Code == s.worldCode {
		return fmt.Errorf("both worlds call their switch %s; set -world-code on one", h.Code)
	}
	for _, c := range h.Carriers {
		if _, ok := s.carriers[c.Designator]; ok {
			if _, foreign := s.foreignCarrier(c.Designator); !foreign {
				return fmt.Errorf("both worlds fly %s; a designator is an address and must be one world's", c.Designator)
			}
		}
	}
	for _, g := range s.GDSes {
		for _, fg := range h.GDS {
			if fg.Address == g.Address {
				return fmt.Errorf("both worlds' distribution systems answer at %s; set -world-city on one", g.Address)
			}
		}
	}
	// The trunk peer is named for the other switch, because that is the
	// name its hello carries when it dials and the name its session is
	// held under; a via peer has to find that session.
	trunkName := h.Code
	trunk := config.Peer{Name: trunkName, Carrier: h.Code, Format: "typeb", TTYAddress: h.SwitchTTY, Trunk: true, Token: h.Token}
	if accepting {
		trunk.Egress = config.Egress{Type: "tcp_accept"}
	} else {
		trunk.Egress = config.Egress{Type: "link_dial", Addr: h.SwitchAddr, Role: "switch"}
	}
	peers := []config.Peer{trunk}
	for _, c := range h.Carriers {
		peers = append(peers, config.Peer{Name: c.Designator, Carrier: c.Designator, Format: c.Format, TTYAddress: c.TTYAddress, ICAO: c.ICAO,
			Egress: config.Egress{Type: "via", Via: trunkName}})
	}
	for _, g := range h.GDS {
		peers = append(peers, config.Peer{Name: "W:" + g.Designator, Format: "typeb", TTYAddress: g.Address, Egress: config.Egress{Type: "via", Via: trunkName}})
	}
	// The other world's carriers become sellable here first -- peers of
	// every distribution system, flights in the schedule the demand draws
	// from, markets in the tariff, legs the globe can place -- and the trunk
	// comes up last, so nothing is routable before it is sellable.
	var flights []world.Flight
	if h.URL != "" {
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Get(strings.TrimRight(h.URL, "/") + "/world/manifest.json")
		if err == nil {
			var m world.Manifest
			if err := json.NewDecoder(resp.Body).Decode(&m); err == nil {
				flights = m.Flights
				if s.Eye != nil {
					s.Eye.AddAirports(m.Airports)
				}
			}
			resp.Body.Close()
		} else {
			s.log.Warn("joined world's manifest not fetched", "world", h.Name, "err", err)
		}
	}
	s.addForeign(h, flights)
	for _, t := range s.Tenants {
		t.AddDistribution(h.Watcher)
	}
	if _, err := s.Switch.ReloadPeers(peers); err != nil {
		return fmt.Errorf("joining %s: %w", h.Name, err)
	}
	s.log.Info("world joined", "world", h.Name, "code", h.Code, "carriers", len(h.Carriers), "flights", len(flights), "accepting", accepting)
	if s.Airline != nil {
		s.Airline.Emit("", "world", fmt.Sprintf("world %s joined: %d carriers, %d flights now sellable here", h.Name, len(h.Carriers), len(flights)), nil)
	}
	return nil
}

// addForeign records a joined world and merges its carriers and flights
// into what this world sells and draws.
func (s *Sim) addForeign(h worldHello, flights []world.Flight) {
	fw := &foreignWorld{Hello: h, Flights: len(flights), JoinedAt: time.Now(),
		carriers: map[string]world.Carrier{}, flights: map[string][]world.Flight{}, byOrigin: map[string][]world.Flight{}}
	for _, c := range h.Carriers {
		fw.carriers[c.Designator] = c
	}
	for _, f := range flights {
		if _, ok := fw.carriers[f.Carrier]; ok {
			fw.flights[f.Carrier] = append(fw.flights[f.Carrier], f)
			fw.byOrigin[f.From] = append(fw.byOrigin[f.From], f)
		}
	}
	s.foreignMu.Lock()
	s.foreign[h.Code] = fw
	s.foreignMu.Unlock()
	if s.tariff != nil {
		s.tariff.AddFlights(flights)
	}
	if s.Eye != nil {
		s.Eye.AddFlights(flights)
	}
	for _, g := range s.GDSes {
		if g.GW == nil {
			continue
		}
		for _, c := range h.Carriers {
			format := store.FormatTypeB
			if c.Format == "edifact" {
				format = store.FormatEDIFACT
			}
			g.GW.AddPeer(&gateway.Peer{Name: c.Designator, Carrier: c.Designator, Format: format, TTYAddress: c.TTYAddress})
		}
	}
}

// foreignFlights is a joined world's carrier's schedule as this world holds it.
func (s *Sim) foreignFlights(code string) []world.Flight {
	s.foreignMu.RLock()
	defer s.foreignMu.RUnlock()
	for _, fw := range s.foreign {
		if fs, ok := fw.flights[code]; ok {
			return fs
		}
	}
	return nil
}

// foreignCarrier says whether a carrier belongs to a joined world.
func (s *Sim) foreignCarrier(code string) (string, bool) {
	s.foreignMu.RLock()
	defer s.foreignMu.RUnlock()
	for _, fw := range s.foreign {
		if _, ok := fw.carriers[code]; ok {
			return fw.Hello.Name, true
		}
	}
	return "", false
}

// scheduleOf is a carrier's flights, this world's or a joined one's.
func (s *Sim) scheduleOf(code string) []world.Flight {
	if fs := s.Flights[code]; len(fs) > 0 {
		return fs
	}
	return s.foreignFlights(code)
}

// sellableCarriers is the carriers the demand may buy: this world's and
// every joined world's.
func (s *Sim) sellableCarriers(local []string) []string {
	s.foreignMu.RLock()
	defer s.foreignMu.RUnlock()
	if len(s.foreign) == 0 {
		return local
	}
	out := append([]string(nil), local...)
	for _, fw := range s.foreign {
		for code, fs := range fw.flights {
			if len(fs) > 0 {
				out = append(out, code)
			}
		}
	}
	sort.Strings(out)
	return out
}

// onwardFrom is every flight leaving an airport, this world's and the
// joined worlds', for a connection.
func (s *Sim) onwardFrom(iata string) []world.Flight {
	local := s.flightsByOrigin[iata]
	s.foreignMu.RLock()
	defer s.foreignMu.RUnlock()
	if len(s.foreign) == 0 {
		return local
	}
	out := append([]world.Flight(nil), local...)
	for _, fw := range s.foreign {
		out = append(out, fw.byOrigin[iata]...)
	}
	return out
}

// Worlds is the joined worlds, for the instruments.
func (s *Sim) Worlds() []map[string]any {
	s.foreignMu.RLock()
	defer s.foreignMu.RUnlock()
	var out []map[string]any
	for _, fw := range s.foreign {
		out = append(out, map[string]any{"name": fw.Hello.Name, "code": fw.Hello.Code, "carriers": len(fw.Hello.Carriers), "flights": fw.Flights, "joined": fw.JoinedAt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["code"].(string) < out[j]["code"].(string) })
	return out
}

// joinPeerWorlds is the initiator: for each world named at boot, keep
// asking to join until it answers, then dial its switch.
func (s *Sim) joinPeerWorlds(ctx context.Context, urls []string) {
	for _, u := range urls {
		u := strings.TrimRight(strings.TrimSpace(u), "/")
		if u == "" {
			continue
		}
		go func() {
			b := make([]byte, 12)
			rand.Read(b) //nolint:errcheck
			token := hex.EncodeToString(b)
			client := &http.Client{Timeout: 60 * time.Second}
			wait := 2 * time.Second
			for ctx.Err() == nil {
				h := s.hello()
				h.Token = token
				body, _ := json.Marshal(h)
				resp, err := client.Post(u+"/federation/world", "application/json", bytes.NewReader(body))
				if err == nil {
					var reply worldHello
					derr := json.NewDecoder(resp.Body).Decode(&reply)
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK && derr == nil {
						reply.Token = token
						if err := s.joinWorld(ctx, reply, false); err != nil {
							s.log.Error("could not join world", "url", u, "err", err)
							return
						}
						return
					}
					if resp.StatusCode == http.StatusConflict {
						s.log.Error("world refused the join", "url", u)
						return
					}
				}
				s.log.Info("waiting for the world to join", "url", u, "err", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
				if wait < 30*time.Second {
					wait *= 2
				}
			}
		}()
	}
}
