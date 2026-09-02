package sim

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"github.com/adamf/wholesky/internal/host"
)

// Oversold is a cabin holding more confirmed seats than it has.
type Oversold struct {
	Carrier, Flight, Date, Board, Compartment string
	Seats, Sold                               int
}

// Invariants is what a running world reports about the laws it must obey,
// so a deployment can be checked from outside while it flies. Cabins and
// Sold are the population the check ran over; Oversold is empty when the
// law holds. The in-process suite in invariants_test.go covers the laws
// that need the wire stopped (message conservation, interline
// convergence); this is the one that can be asked of a live sky.
type Invariants struct {
	Shards   int        `json:"shards"`
	Cabins   int        `json:"cabins"`
	Sold     int        `json:"sold"`
	Oversold []Oversold `json:"oversold"`
}

// OK is whether every law held.
func (v Invariants) OK() bool { return len(v.Oversold) == 0 }

// Merge folds another shard's report into this one.
func (v *Invariants) Merge(o Invariants) {
	v.Shards += o.Shards
	v.Cabins += o.Cabins
	v.Sold += o.Sold
	v.Oversold = append(v.Oversold, o.Oversold...)
}

// checkInventories reads every tenant's inventory. It is the live half of
// TestInvariantNoOversell: the same law, asked of the seats the carriers
// are actually holding rather than a booted fixture.
func checkInventories(tenants map[string]*host.Tenant) Invariants {
	v := Invariants{Shards: 1, Oversold: []Oversold{}}
	codes := make([]string, 0, len(tenants))
	for code := range tenants {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		t := tenants[code]
		if t == nil || t.Inventory == nil {
			continue
		}
		for _, p := range t.Inventory.Snapshot() {
			v.Cabins++
			v.Sold += p.Sold
			if p.Sold > p.Seats {
				v.Oversold = append(v.Oversold, Oversold{Carrier: code, Flight: p.Flight, Date: p.Date, Board: p.Board,
					Compartment: p.Compartment, Seats: p.Seats, Sold: p.Sold})
			}
		}
	}
	return v
}

// Invariants reports on this process's carriers.
func (s *Sim) Invariants() Invariants { return checkInventories(s.Tenants) }

func (s *Sim) serveInvariants(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.Invariants()) //nolint:errcheck
}

// federatedInvariants asks every live shard and adds the core's own
// carriers, if it has any. A shard that does not answer is a failed check,
// not a silent pass: the gate must not go green because a machine was
// down.
func (c *Core) federatedInvariants(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 10 * 1e9}
	v := c.Sim.Invariants()
	var unreachable []string
	for _, p := range c.livePeers() {
		resp, err := client.Get(p.URL + "/shard/invariants.json")
		if err != nil {
			unreachable = append(unreachable, p.Name)
			continue
		}
		var o Invariants
		err = json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&o)
		resp.Body.Close()
		if err != nil {
			unreachable = append(unreachable, p.Name)
			continue
		}
		v.Merge(o)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct { //nolint:errcheck
		Invariants
		Unreachable []string `json:"unreachable,omitempty"`
	}{v, unreachable})
}
