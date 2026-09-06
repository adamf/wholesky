package airline

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A run is recorded from take to release: every tape line in between with
// the clock on it, the scorecard as sampled, the seat's own notes; the
// finished run is kept and handed on, and a stranger's note is refused.
func TestARunIsRecordedTakeToRelease(t *testing.T) {
	r := New(0)
	pos := 360.0
	r.Pos = func() float64 { return pos }
	var handed *Recording
	done := make(chan struct{}, 1)
	r.OnRecording = func(rec *Recording) { handed = rec; done <- struct{}{} }

	seat, token, err := r.Take("BA", "Claude")
	if err != nil {
		t.Fatal(err)
	}
	if !ValidRecordingID(seat.Recording) {
		t.Fatalf("seat carries no recording id: %q", seat.Recording)
	}
	pos = 400
	r.Emit("BA", "incident", "fog at LHR", nil)
	r.Sample("BA", Scorecard{Score: 12.5, Flown: 3})
	pos = 420
	if err := r.Note("BA", token, "holding BA0117: the fog lifts by 0800, a swap costs more than the delay"); err != nil {
		t.Fatal(err)
	}
	if err := r.Note("BA", "not-the-token", "evil"); err == nil {
		t.Fatal("a stranger's note was taken")
	}
	if err := r.Note("BA", token, "   "); err == nil {
		t.Fatal("an empty note was taken")
	}
	live, ok := r.LiveRecording("BA")
	if !ok || !live.Live || len(live.Events) != 3 || live.Events[2].Kind != "agent" || live.Events[2].Pos != 420 || live.Events[1].Pos != 400 {
		t.Fatalf("live recording wrong: ok=%v %+v", ok, live)
	}
	if len(r.Recordings()) != 1 || !r.Recordings()[0].Live {
		t.Fatalf("index should list the live run: %+v", r.Recordings())
	}
	pos = 500
	if err := r.Release("BA", token); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the finished run was not handed on")
	}
	if handed.ID != seat.Recording || handed.Live || handed.EndPos != 500 || handed.StartPos != 360 || len(handed.Events) != 4 || len(handed.Scores) != 1 {
		t.Fatalf("finished run wrong: %+v", handed)
	}
	if _, ok := r.LiveRecording("BA"); ok {
		t.Fatal("a released seat still has a live run")
	}
	got, ok := r.Recording(seat.Recording)
	if !ok || got.Holder != "Claude" || len(got.Events) != 4 {
		t.Fatalf("finished run not kept: %v %+v", ok, got)
	}
	// The kept copy does not share the run's slices with a later caller.
	got.Events[0].Text = "tampered"
	again, _ := r.Recording(seat.Recording)
	if again.Events[0].Text == "tampered" {
		t.Fatal("recordings are served by reference")
	}
}

// The pages and endpoints accept a recording id or a carrier code and
// nothing else, and the note endpoint needs the seat's token.
func TestReplayRoutes(t *testing.T) {
	r := New(0)
	s := &Server{Reg: r}
	mux := http.NewServeMux()
	s.Routes(mux)
	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec
	}
	for _, p := range []string{"/replay/%3Cscript%3E", "/replay/zz-zz", "/recording/%3Cx%3E.json", "/recording/abc.json"} {
		if code := get(p).Code; code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, code)
		}
	}
	if rec := get("/replay/ba"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `const ID="BA"`) || rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("GET /replay/ba: %d", rec.Code)
	}
	_, token, _ := r.Take("BA", "Claude")
	id := r.Recordings()[0].ID
	if rec := get("/replay/" + id); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `const ID="`+id+`"`) {
		t.Fatalf("GET /replay/%s: %d", id, rec.Code)
	}
	if rec := get("/carrier/BA/recording.json"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"live":true`) {
		t.Fatalf("live recording: %d %s", rec.Code, rec.Body.String())
	}
	post := func(p, tok, body string) int {
		req := httptest.NewRequest(http.MethodPost, p, strings.NewReader(body))
		if tok != "" {
			req.Header.Set("X-Seat-Token", tok)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post("/carrier/BA/note", "", `{"text":"hi"}`); code != http.StatusForbidden {
		t.Fatalf("note without token: %d", code)
	}
	if code := post("/carrier/BA/note", token, `{"text":"the fog is lifting"}`); code != http.StatusOK {
		t.Fatalf("note with token: %d", code)
	}
	if rec := get("/recording/" + id + ".json"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "the fog is lifting") {
		t.Fatalf("recording by id: %d", rec.Code)
	}
	if rec := get("/recordings.json"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), id) {
		t.Fatalf("index: %d %s", rec.Code, rec.Body.String())
	}
}
