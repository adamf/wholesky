// Package fleet is the cluster view: every Jetway node in the world at once.
//
// The globe shows what the sky is doing; the fleet shows what the systems are
// doing. One row per node -- the switch, the GDS, and every carrier tenant --
// with live message counts fed by tapping each node's own event bus, so the
// overview costs no store scans at all. Drilling into a node reads that one
// node's store, and drilling into a message shows the bytes as they crossed
// the wire, which is the ground truth everything else is a view of.
package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/jetway/pkg/gateway"
	"github.com/adamf/jetway/pkg/store"
)

// NodeKind says what a node is for.
type NodeKind string

const (
	KindSwitch  NodeKind = "switch"
	KindGDS     NodeKind = "gds"
	KindCarrier NodeKind = "carrier"
)

// node is one Jetway system's live ledger.
type node struct {
	Code   string   `json:"code"`
	Name   string   `json:"name"`
	Kind   NodeKind `json:"kind"`
	Format string   `json:"format,omitempty"`
	// Transport names the circuit type when it is not the plain framed link:
	// "matip" for carriers dialling in over the airline transport.
	Transport string `json:"transport,omitempty"`
	Hub       string `json:"hub,omitempty"`
	Flights   int    `json:"flights,omitempty"`

	mu       sync.Mutex
	in, out  int64
	lastAt   time.Time
	lastKind string

	st store.Store
}

// Collector watches every node.
type Collector struct {
	mu    sync.Mutex
	nodes map[string]*node
	order []string

	// LivePeers reports which subscribers hold a session on the switch, so
	// the dashboard can mark a dead link the moment it dies.
	LivePeers func() []string

	// LinkControl, when set, severs or restores a carrier's switch circuit.
	// It is the fleet page's chaos hook; the world behind it decides what a
	// cut link means.
	LinkControl func(code, action string) error

	// Remotes, when set, names the other machines whose fleets this board
	// merges in: each serves the same /fleet endpoints for its own nodes.
	Remotes func() []Remote
	// Owner, when set, answers which remote's URL owns a node code, so
	// drill-downs proxy to the machine that holds the store.
	Owner func(code string) string
	// OnOwners, when set, receives the code-to-URL map each merge discovers.
	OnOwners func(map[string]string)

	remoteMu   sync.Mutex
	remoteRows []json.RawMessage
	remoteAt   time.Time
}

// Remote is one peer machine's fleet.
type Remote struct {
	Name string
	URL  string
}

// New builds an empty collector.
func New() *Collector {
	return &Collector{nodes: map[string]*node{}}
}

// Add registers a node and taps its bus.
//
// Everything the overview shows comes from these taps: a message event
// increments a counter and stamps the clock, and that is all. The store
// handle is kept only for the drill-down, which reads one node at a time.
func (c *Collector) Add(ctx context.Context, code, name string, kind NodeKind,
	format, transport, hub string, flights int, st store.Store, bus *gateway.Bus) {

	n := &node{Code: code, Name: name, Kind: kind, Format: format,
		Transport: transport, Hub: hub, Flights: flights, st: st}
	c.mu.Lock()
	c.nodes[code] = n
	c.order = append(c.order, code)
	c.mu.Unlock()

	if bus == nil {
		return
	}
	sub, unsub := bus.Subscribe()
	go func() {
		defer unsub()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-sub:
				if ev.Type != gateway.EvMessage {
					continue
				}
				p, ok := ev.Data.(map[string]any)
				if !ok {
					continue
				}
				dir, _ := p["direction"].(string)
				kindS, _ := p["kind"].(string)
				n.mu.Lock()
				if dir == "out" {
					n.out++
				} else {
					n.in++
				}
				n.lastAt = time.Now()
				if kindS != "" {
					n.lastKind = kindS
				}
				n.mu.Unlock()
			}
		}
	}()
}

// Count feeds one message event into a node registered without a bus.
func (c *Collector) Count(code string, payload map[string]any) {
	n := c.byCode(code)
	if n == nil {
		return
	}
	dir, _ := payload["direction"].(string)
	kindS, _ := payload["kind"].(string)
	n.mu.Lock()
	if dir == "out" {
		n.out++
	} else {
		n.in++
	}
	n.lastAt = time.Now()
	if kindS != "" {
		n.lastKind = kindS
	}
	n.mu.Unlock()
}

// Routes mounts the fleet on a mux.
func (c *Collector) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /fleet", c.page)
	mux.HandleFunc("GET /fleet/nodes.json", c.nodesJSON)
	mux.HandleFunc("GET /fleet/node/{code}/messages.json", c.messagesJSON)
	mux.HandleFunc("GET /fleet/node/{code}/message/{id}", c.raw)
	mux.HandleFunc("GET /fleet/node/{code}/detail.json", c.detailJSON)
	mux.HandleFunc("POST /fleet/node/{code}/link", c.linkControl)
}

// linkControl severs or restores one carrier's circuit.
func (c *Collector) linkControl(w http.ResponseWriter, r *http.Request) {
	if code := r.PathValue("code"); code != "" && c.byCode(code) == nil && c.proxyToOwner(w, r, code) {
		return
	}
	if c.LinkControl == nil {
		http.Error(w, "this fleet has no link control", http.StatusNotImplemented)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req); err != nil {
		http.Error(w, "malformed request", http.StatusBadRequest)
		return
	}
	if err := c.LinkControl(r.PathValue("code"), req.Action); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
}

func (c *Collector) nodesJSON(w http.ResponseWriter, r *http.Request) {
	live := map[string]bool{}
	if c.LivePeers != nil {
		for _, p := range c.LivePeers() {
			live[p] = true
		}
	}
	type row struct {
		Code      string   `json:"code"`
		Name      string   `json:"name"`
		Kind      NodeKind `json:"kind"`
		Format    string   `json:"format,omitempty"`
		Transport string   `json:"transport,omitempty"`
		Hub       string   `json:"hub,omitempty"`
		Flights   int      `json:"flights,omitempty"`
		In        int64    `json:"in"`
		Out       int64    `json:"out"`
		LastAt    string   `json:"last_at,omitempty"`
		LastKind  string   `json:"last_kind,omitempty"`
		Link      bool     `json:"link"`
	}
	c.mu.Lock()
	rows := make([]row, 0, len(c.order))
	for _, code := range c.order {
		n := c.nodes[code]
		n.mu.Lock()
		rw := row{Code: n.Code, Name: n.Name, Kind: n.Kind, Format: n.Format,
			Transport: n.Transport, Hub: n.Hub, Flights: n.Flights, In: n.in, Out: n.out,
			LastKind: n.lastKind,
			// The switch and GDS hold the sessions rather than appearing in
			// the peer list, so they read as up by construction.
			Link: n.Kind != KindCarrier || live[n.Code]}
		if !n.lastAt.IsZero() {
			rw.LastAt = n.lastAt.UTC().Format(time.RFC3339)
		}
		n.mu.Unlock()
		rows = append(rows, rw)
	}
	c.mu.Unlock()
	remote := c.remoteNodeRows()
	sort.SliceStable(rows, func(i, j int) bool {
		ki, kj := kindRank(rows[i].Kind), kindRank(rows[j].Kind)
		if ki != kj {
			return ki < kj
		}
		return rows[i].In+rows[i].Out > rows[j].In+rows[j].Out
	})
	w.Header().Set("Content-Type", "application/json")
	if len(remote) == 0 {
		json.NewEncoder(w).Encode(rows) //nolint:errcheck
		return
	}
	// Merge without re-decoding: local rows first, then every remote row.
	var b bytes.Buffer
	b.WriteByte('[')
	enc := json.NewEncoder(&b)
	for i, rw := range rows {
		if i > 0 {
			b.WriteByte(',')
		}
		enc.Encode(rw)          //nolint:errcheck
		b.Truncate(b.Len() - 1) // drop Encode's newline
	}
	for _, raw := range remote {
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		b.Write(raw)
	}
	b.WriteByte(']')
	w.Write(b.Bytes()) //nolint:errcheck
}

// remoteNodeRows fetches and caches every peer's rows. Two seconds of cache
// keeps twenty open boards from multiplying into a poll storm.
func (c *Collector) remoteNodeRows() []json.RawMessage {
	if c.Remotes == nil {
		return nil
	}
	c.remoteMu.Lock()
	defer c.remoteMu.Unlock()
	if time.Since(c.remoteAt) < 2*time.Second {
		return c.remoteRows
	}
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	var rows []json.RawMessage
	owners := map[string]string{}
	for _, r := range c.Remotes() {
		resp, err := client.Get(r.URL + "/fleet/nodes.json")
		if err != nil {
			continue
		}
		var batch []json.RawMessage
		err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&batch)
		resp.Body.Close()
		if err != nil {
			continue
		}
		for _, raw := range batch {
			var probe struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(raw, &probe) == nil && probe.Code != "" {
				owners[probe.Code] = r.URL
			}
			rows = append(rows, raw)
		}
	}
	c.remoteRows, c.remoteAt = rows, time.Now()
	if c.OnOwners != nil {
		c.OnOwners(owners)
	}
	return rows
}

// WarmRemotes refreshes the merged remote rows -- and with them the
// ownership map -- outside the nodes.json request path, so a drill-down or
// console link that arrives before anyone has loaded the board still finds
// its owner.
func (c *Collector) WarmRemotes() {
	c.remoteNodeRows()
}

// proxyToOwner forwards a drill-down to the machine that owns the node.
// It reports whether it handled the request.
func (c *Collector) proxyToOwner(w http.ResponseWriter, r *http.Request, code string) bool {
	if c.Owner == nil {
		return false
	}
	owner := c.Owner(code)
	if owner == "" {
		c.WarmRemotes()
		owner = c.Owner(code)
	}
	if owner == "" {
		return false
	}
	client := &http.Client{Timeout: 3 * time.Second}
	var resp *http.Response
	var err error
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		resp, err = client.Post(owner+r.URL.Path, r.Header.Get("Content-Type"), bytes.NewReader(body))
	} else {
		resp, err = client.Get(owner + r.URL.Path)
	}
	if err != nil {
		http.Error(w, "the machine holding "+code+" did not answer: "+err.Error(),
			http.StatusBadGateway)
		return true
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 8<<20)) //nolint:errcheck
	return true
}

func kindRank(k NodeKind) int {
	switch k {
	case KindSwitch:
		return 0
	case KindGDS:
		return 1
	}
	return 2
}

func (c *Collector) byCode(code string) *node {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nodes[strings.ToUpper(code)]
}

func (c *Collector) messagesJSON(w http.ResponseWriter, r *http.Request) {
	if code := r.PathValue("code"); code != "" && c.byCode(code) == nil && c.proxyToOwner(w, r, code) {
		return
	}
	n := c.byCode(r.PathValue("code"))
	if n == nil || n.st == nil {
		http.Error(w, "no such node", http.StatusNotFound)
		return
	}
	msgs, err := n.st.ListMessages(r.Context(), store.MessageFilter{Limit: 80})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type msg struct {
		ID     string `json:"id"`
		At     string `json:"at"`
		Dir    string `json:"dir"`
		Peer   string `json:"peer"`
		Format string `json:"format"`
		Kind   string `json:"kind"`
		Status string `json:"status"`
		Size   int    `json:"size"`
		Error  string `json:"error,omitempty"`
	}
	out := make([]msg, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, msg{
			ID: m.ID, At: m.At.UTC().Format("15:04:05"),
			Dir: string(m.Direction), Peer: m.Peer, Format: string(m.Format),
			Kind: m.Kind, Status: string(m.Status), Size: m.Size, Error: m.Error,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out) //nolint:errcheck
}

// detailJSON is the one place the fleet reads records: a single node, on
// demand, when somebody is actually looking at it.
func (c *Collector) detailJSON(w http.ResponseWriter, r *http.Request) {
	if code := r.PathValue("code"); code != "" && c.byCode(code) == nil && c.proxyToOwner(w, r, code) {
		return
	}
	n := c.byCode(r.PathValue("code"))
	if n == nil || n.st == nil {
		http.Error(w, "no such node", http.StatusNotFound)
		return
	}
	recs, err := n.st.ListPNRs(r.Context(), 10000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	queues := map[string]int{}
	if items, err := n.st.ListQueue(r.Context(), store.QueueFilter{}); err == nil {
		for _, it := range items {
			queues[it.Queue]++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
		"records": len(recs), "queues": queues,
	})
}

// raw serves one message's bytes as they crossed the wire. The stored raw is
// the evidence; showing anything reconstructed here would defeat the point.
func (c *Collector) raw(w http.ResponseWriter, r *http.Request) {
	if code := r.PathValue("code"); code != "" && c.byCode(code) == nil && c.proxyToOwner(w, r, code) {
		return
	}
	n := c.byCode(r.PathValue("code"))
	if n == nil || n.st == nil {
		http.Error(w, "no such node", http.StatusNotFound)
		return
	}
	m, err := n.st.GetMessage(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "no such message", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "# %s %s %s %s %s %d bytes\n\n", //nolint:errcheck
		m.ID, m.Direction, m.Peer, m.Format, m.Kind, m.Size)
	w.Write(m.Raw) //nolint:errcheck
}
