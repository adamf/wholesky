package sim

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// Running a carrier, end to end through the API: the lobby lists the
// carriers with scores, a seat is taken, operations goes manual, the day's
// retime becomes a decision the seat holds, a cancellation pulled as a
// lever goes out on the wire and shows on the scorecard, and the fares
// move by the multiplier.
func TestSeatRunsACarrier(t *testing.T) {
	s := bootWorld(t, Options{DecisionWindow: 3 * time.Second})
	ctx := context.Background()
	mux := http.NewServeMux()
	srv := &airline.Server{Reg: s.Airline, World: seatWorld{s}}
	srv.Routes(mux)
	call := func(method, path, token string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			json.NewEncoder(&buf).Encode(body) //nolint:errcheck
		}
		req := httptest.NewRequest(method, path, &buf)
		if token != "" {
			req.Header.Set("X-Seat-Token", token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out) //nolint:errcheck
		return rec.Code, out
	}
	code, lobby := call("GET", "/carriers.json", "", nil)
	carriers, _ := lobby["carriers"].([]any)
	if code != 200 || len(carriers) == 0 {
		t.Fatalf("lobby %d %v", code, lobby)
	}
	cc := s.Manifest.Carriers[0].Designator
	f := s.Flights[cc][0]
	tn := s.Tenants[cc]

	code, taken := call("POST", "/carrier/"+cc+"/take", "", map[string]string{"holder": "Adam"})
	token, _ := taken["token"].(string)
	if code != 200 || token == "" {
		t.Fatalf("take %d %v", code, taken)
	}
	if code, _ := call("POST", "/carrier/"+cc+"/take", "", map[string]string{"holder": "someone"}); code != 409 {
		t.Errorf("a held seat answers 409, got %d", code)
	}
	if code, _ := call("POST", "/carrier/"+cc+"/departments", token, map[string]any{"department": "ops", "manual": true}); code != 200 {
		t.Fatalf("departments %d", code)
	}

	// The day's retime, gated: the seat holds it, so no TIM goes out.
	go func() {
		if !s.askRetime(ctx, f, 52, 47) {
			return
		}
		tn.Retime(ctx, f, s.BookingDate, 52, 47) //nolint:errcheck
	}()
	var inbox []airline.Decision
	deadline := time.Now().Add(2 * time.Second)
	for len(inbox) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		inbox = s.Airline.Inbox(cc)
	}
	if len(inbox) != 1 || inbox[0].Department != "ops" || !strings.Contains(inbox[0].Title, "52 minutes late") {
		t.Fatalf("inbox %+v", inbox)
	}
	if code, _ := call("POST", "/carrier/"+cc+"/decide", token, map[string]string{"id": inbox[0].ID, "option": "hold"}); code != 200 {
		t.Fatalf("decide %d", code)
	}
	time.Sleep(200 * time.Millisecond)
	if sum, ok := tn.Summarise(f.Carrier+f.Number, f.From); ok && sum.Retimed != "" {
		t.Errorf("held, yet retimed: %q", sum.Retimed)
	}
	seat, _ := s.Airline.Seat(cc)
	if seat.Answered != 1 {
		t.Errorf("seat %+v", seat)
	}

	// A lever: cancel another flight. It is announced, the plan holds it
	// cancelled, the board shows it, the scorecard counts it.
	// A flight still ahead of the clock, since the small world's day runs
	// on the wall clock and the test may run at any hour.
	pos, _ := seatWorld{s}.Clock()
	var g world.Flight
	found := false
	for _, cand := range s.Flights[cc] {
		if float64(cand.DepMin) > pos+cancelledBefore+10 && cand.Carrier+cand.Number != f.Carrier+f.Number {
			g, found = cand, true
			break
		}
	}
	if !found {
		t.Skip("no departure left in the day to cancel")
	}
	code, res := call("POST", "/carrier/"+cc+"/act", token, airline.Action{Kind: "cancel", Flight: g.Carrier + g.Number, Board: g.From, Reason: "ops decision"})
	if code != 200 {
		t.Fatalf("cancel %d %v", code, res)
	}
	if fate := s.fate.Of(g); !fate.Cancelled || fate.Reason != "ops decision" {
		t.Errorf("fate %+v", fate)
	}
	if _, done := s.announced.Load(dayplan.Key(g)); !done {
		t.Error("the cancellation was not announced")
	}
	code, st := call("GET", "/carrier/"+cc+"/state", "", nil)
	if code != 200 {
		t.Fatal(code)
	}
	score := st["score"].(map[string]any)
	if score["cancelled"].(float64) < 1 {
		t.Errorf("scorecard %v", score)
	}
	found = false
	for _, x := range st["flights"].([]any) {
		fs := x.(map[string]any)
		if fs["flight"] == g.Carrier+g.Number && fs["status"] == "cancelled" {
			found = true
		}
	}
	if !found {
		t.Error("the board does not show the cancellation")
	}
	// Without the token a lever is refused; with it the fares move.
	if code, _ := call("POST", "/carrier/"+cc+"/act", "", airline.Action{Kind: "fares", Multiplier: 1.2}); code != 403 {
		t.Errorf("unauthorised act %d", code)
	}
	if code, _ := call("POST", "/carrier/"+cc+"/act", token, airline.Action{Kind: "fares", Multiplier: 1.2}); code != 200 || s.tariff.Multiplier(cc) != 1.2 {
		t.Errorf("fares %d ×%.2f", code, s.tariff.Multiplier(cc))
	}
	if code, _ := call("POST", "/carrier/"+cc+"/release", token, nil); code != 200 {
		t.Errorf("release %d", code)
	}
	if _, held := s.Airline.Seat(cc); held {
		t.Error("released seat still held")
	}
}
