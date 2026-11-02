package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/workforce"
)

// The carrier's people and its operations centre, through the API an
// agent uses: the roster and its agreements, a pilot's record, the desk's
// signals, a projection with its spread and its tornado, weather added to
// the day, standby crews brought in, a union answered, and the desk's
// decision on forecast weather turned into cancellations on the wire.
func TestCrewsAndOperationsCentreThroughTheAPI(t *testing.T) {
	s := bootWorld(t, Options{DecisionWindow: 3 * time.Second, NOCTrials: 16})
	// The day held at 06:00, so the test reads the same at any hour.
	s.clock.Sync(time.Now(), 360, 0)
	ctx := context.Background()
	mux := http.NewServeMux()
	srv := &airline.Server{Reg: s.Airline, World: s.lobby}
	srv.Routes(mux)
	mux.HandleFunc("POST /noc/inject", s.serveInject)
	call := func(method, path, token string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			json.NewEncoder(&buf).Encode(body) //nolint:errcheck
		}
		req := httptest.NewRequest(method, path, &buf)
		if token != "" {
			req.Header.Set("X-Seat-Token", token)
		}
		req.Header.Set(secretHeader, s.linkSecret)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out) //nolint:errcheck
		return rec.Code, out
	}
	// The carrier with the most flights, and the busiest arrival point of
	// its own after ten in the morning.
	cc := ""
	for code := range s.Tenants {
		if cc == "" || len(s.Flights[code]) > len(s.Flights[cc]) {
			cc = code
		}
	}
	hub := s.carriers[cc].Hub

	code, crew := call("GET", "/carrier/"+cc+"/crew", "", nil)
	if code != 200 || crew["members"].(float64) == 0 || len(crew["contracts"].([]any)) != 3 || crew["duties"].(float64) == 0 {
		t.Fatalf("crew %d %v", code, crew)
	}
	watch, _ := crew["watch"].([]any)
	if len(watch) == 0 {
		t.Fatal("no pilot on the watch list")
	}
	id := watch[0].(map[string]any)["id"].(string)
	code, member := call("GET", "/carrier/"+cc+"/crew/"+id, "", nil)
	if code != 200 || len(member["days"].([]any)) != 28 || len(member["limits"].([]any)) != 4 {
		t.Fatalf("member %d %v", code, member)
	}

	code, desk := call("GET", "/carrier/"+cc+"/noc", "", nil)
	if code != 200 || int(desk["day"].(map[string]any)["flights"].(float64)) != len(s.Flights[cc]) {
		t.Fatalf("noc %d %v", code, desk)
	}

	// A projection of a storm over the hub from 10:00.
	code, pr := call("POST", "/carrier/"+cc+"/noc/project", "", map[string]any{"trials": 16, "sensitivity": true,
		"scenario": map[string]any{"name": "storm", "events": []map[string]any{{"kind": "weather", "airport": hub,
			"onset": map[string]any{"kind": "triangular", "a": 570, "b": 600, "c": 660}, "duration": map[string]any{"kind": "fixed", "a": 300},
			"factor": map[string]any{"kind": "triangular", "a": 0.3, "b": 0.4, "c": 0.5}}}}})
	results, _ := pr["results"].([]any)
	if code != 200 || len(results) != 4 || pr["best"] == "" {
		t.Fatalf("project %d %v", code, pr)
	}
	if swings, _ := pr["swings"].([]any); len(swings) != 2 {
		t.Errorf("onset and factor are uncertain: %v swings", len(swings))
	}

	// Weather added to the day at the hub, from 10:00 for five hours.
	_, regs0, _ := s.fate.Picture()
	before := len(regs0)
	code, out := call("POST", "/noc/inject", "", map[string]any{"kind": "weather", "airport": hub, "start": 600, "duration": 300, "factor": 0.4})
	if code != 200 || !strings.Contains(fmt.Sprint(out["ok"]), "flights changed") {
		t.Fatalf("inject %d %v", code, out)
	}
	cells, regs, _ := s.fate.Picture()
	if len(regs) <= before || !strings.HasPrefix(cells[len(cells)-1].Name, hub+"X") {
		t.Errorf("the injected cell regulated nothing: %d regulations, cells %v", len(regs), cells)
	}

	// A seat brings in standby crews and answers a union.
	code, taken := call("POST", "/carrier/"+cc+"/take", "", map[string]string{"holder": "Adam"})
	token, _ := taken["token"].(string)
	if code != 200 {
		t.Fatalf("take %d", code)
	}
	code, act := call("POST", "/carrier/"+cc+"/act", token, airline.Action{Kind: "standby", Board: hub, Count: 2})
	if code != 200 {
		t.Fatalf("standby %d %v", code, act)
	}
	if o, _ := s.Workforce.Overview(cc, 360); o.Standby != 8 {
		t.Errorf("two standby crews are eight members: %d", o.Standby)
	}
	if code, _ := call("POST", "/carrier/"+cc+"/act", token, airline.Action{Kind: "standby", Board: "ZZZ", Count: 1}); code == 200 {
		t.Error("standby at a station that is no base")
	}
	for _, u := range []workforce.Unit{workforce.Pilots, workforce.FlightAttendants, workforce.Ground} {
		if c, _ := s.Workforce.Contract(cc, u); c.Proposal > 0 {
			if code, res := call("POST", "/carrier/"+cc+"/act", token, airline.Action{Kind: "respond", Unit: string(u), Choice: "accept"}); code != 200 || c.Status != workforce.InForce {
				t.Errorf("accepting the %s: %d %v, now %s", u, code, res, c.Status)
			}
		}
	}

	// The desk's decision: the seat runs it, the forecast is the injected
	// storm, and the seat thins the schedule by thirty per cent.
	if code, _ := call("POST", "/carrier/"+cc+"/departments", token, map[string]any{"department": "noc", "manual": true}); code != 200 {
		t.Fatalf("departments %d", code)
	}
	cells, _, _ = s.fate.Picture()
	storm := cells[len(cells)-1]
	if s.arrivalsUnder(cc, storm) == 0 {
		t.Skipf("%s has no arrivals at %s in the storm", cc, hub)
	}
	done := make(chan struct{})
	go func() { s.nocDecide(ctx, cc, storm); close(done) }()
	var d airline.Decision
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if in := s.Airline.Inbox(cc); len(in) > 0 {
			d = in[0]
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if d.Department != "noc" || len(d.Options) != 4 {
		t.Fatalf("decision %+v", d)
	}
	if code, res := call("POST", "/carrier/"+cc+"/decide", token, map[string]string{"id": d.ID, "option": "thin 30%"}); code != 200 {
		t.Fatalf("decide %d %v", code, res)
	}
	<-done
	thinned := 0
	for _, f := range s.Flights[cc] {
		if fate := s.fate.Of(f); fate.Cancelled && strings.HasPrefix(fate.Reason, "noc") {
			thinned++
			if _, sent := s.announced.Load(dayplan.Key(f)); !sent {
				t.Errorf("%s%s thinned but never announced", f.Carrier, f.Number)
			}
		}
	}
	if thinned == 0 {
		t.Error("thinning cancelled nothing")
	}
	sc := seatWorld{s}.score(cc)
	if wc, ok := s.Workforce.Costs(cc, 1440); !ok || wc.Total <= 0 || wc.Reserves <= 0 {
		t.Errorf("crew costs %+v", wc)
	}
	_ = sc
}

// A new day of the same schedule: the records move on, the crews are
// staffed afresh, and yesterday's injected weather is gone.
func TestNewDayStaffsAfresh(t *testing.T) {
	s := bootWorld(t, Options{})
	s.clock.Sync(time.Now(), 360, 0)
	cc := s.Manifest.Carriers[0].Designator
	if _, err := s.applyInject(context.Background(), nocEvent{Kind: "weather", Airport: s.carriers[cc].Hub, Start: 600, Duration: 240, Factor: 0.4}); err != nil {
		t.Fatal(err)
	}
	cells, _, _ := s.fate.Picture()
	_, day0 := s.Workforce.Day()
	s.newDay()
	after, _, sum := s.fate.Picture()
	if len(after) != len(cells)-1 {
		t.Errorf("cells %d then %d", len(cells), len(after))
	}
	if _, n := s.Workforce.Day(); n != day0+1 {
		t.Errorf("day %d then %d", day0, n)
	}
	if sum.Duties == 0 {
		t.Error("the new day staffed nothing")
	}
}
