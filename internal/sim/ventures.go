package sim

import (
	"net/http"
	"sort"
	"time"

	"encoding/json"

	"github.com/adamf/wholesky/internal/partners"
)

// Joint ventures share what their routes earn. Each venture's parties
// put the revenue of the flights they fly on its routes into a pool, and
// each takes out its share of the seats flown there (internal/partners).
// A party that sold more than its share of the seats pays the others
// the difference, so none of them gains by taking a passenger from
// another. The scorecard shows what a carrier received or paid.

// ventureView is one venture's pool, for the instruments.
type ventureView struct {
	partners.JointVenture
	// Pool is what its routes earned, in cents, and Shares what each
	// party received (or, below nothing, paid).
	Pool   int64            `json:"pool"`
	Shares map[string]int64 `json:"shares"`
}

// pooled is what each carrier received from the joint ventures it is in,
// in cents, and each venture's pool. It is worked out again every ten
// seconds at most.
func (s *Sim) pooled() (map[string]int64, []ventureView) {
	if s.commerce == nil || s.commerce.partners == nil {
		return nil, nil
	}
	s.commerce.poolMu.Lock()
	defer s.commerce.poolMu.Unlock()
	if s.commerce.poolAt.After(time.Now().Add(-10*time.Second)) && s.commerce.pool != nil {
		return s.commerce.pool, s.commerce.pools
	}
	out := map[string]int64{}
	var views []ventureView
	for _, v := range s.commerce.partners.Ventures() {
		revenue := map[string]int64{}
		capacity := map[string]float64{}
		for _, code := range v.Parties {
			for _, f := range s.Flights[code] {
				from, to := s.airports[f.From], s.airports[f.To]
				var fromRegion, toRegion string
				if s.econ != nil {
					fromRegion, toRegion = s.econ.region[f.From], s.econ.region[f.To]
				}
				if !v.Covers(fromRegion, from.Country, toRegion, to.Country) {
					continue
				}
				revenue[code] += s.legRevenue(f)
				capacity[code] += float64(f.Seats) * float64(f.KM)
			}
		}
		shares := partners.Pool(revenue, capacity)
		if len(shares) == 0 {
			continue
		}
		view := ventureView{JointVenture: v, Shares: shares}
		for code, adj := range shares {
			out[code] += adj
			view.Pool += revenue[code]
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Pool > views[j].Pool })
	s.commerce.pool, s.commerce.pools, s.commerce.poolAt = out, views, time.Now()
	return out, views
}

// serveVentures is GET /ventures.json: each joint venture, its pool and
// what each party received from it.
func (s *Sim) serveVentures(w http.ResponseWriter, r *http.Request) {
	_, views := s.pooled()
	if views == nil {
		views = []ventureView{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"ventures": views, //nolint:errcheck
		"note": "Money is in cents. A venture's pool is what its parties' flights earned on its routes. A party's share is its part of the seats flown there, and the figure is that less what it earned itself."})
}
