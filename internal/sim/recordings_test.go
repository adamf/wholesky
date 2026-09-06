package sim

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/airline"
)

// A core keeps finished runs beside its state file, with an index line
// each, serves them back by id, takes a region's over the private network
// only with the secret, and never writes a name it did not make.
func TestRecordingsLiveBesideTheStateFile(t *testing.T) {
	dir := t.TempDir()
	s := &Sim{state: &stateKeeper{path: filepath.Join(dir, "state.json")}, log: slog.New(slog.NewTextHandler(io.Discard, nil)), linkSecret: "s3"}
	rec := &airline.Recording{ID: "0123abcd4567", Carrier: "BA", Holder: "Claude", Started: time.Now(), Ended: time.Now(), StartPos: 360, EndPos: 900,
		Events: []airline.Event{{Kind: "agent", Text: "holding the fog", Pos: 400}}, Scores: []airline.ScoreSample{{Pos: 890, Score: airline.Scorecard{Score: 42}}}}
	s.storeRecording(rec)
	got, ok := s.Recording(rec.ID)
	if !ok || got.Holder != "Claude" || len(got.Events) != 1 {
		t.Fatalf("recording not read back: %v %+v", ok, got)
	}
	idx := s.Recordings()
	if len(idx) != 1 || idx[0].ID != rec.ID || idx[0].Final == nil || idx[0].Final.Score != 42 || idx[0].Events != 1 {
		t.Fatalf("index wrong: %+v", idx)
	}
	if _, ok := s.Recording("../state"); ok {
		t.Fatal("a path was read as a recording")
	}

	// A region's run arrives over the control plane, with the secret.
	body := `{"id":"feedfacefeed","carrier":"AF","holder":"Edith","started":"2026-09-06T10:00:00Z","events":[],"scores":[]}`
	h := s.requireSecret(s.servePutRecording)
	req := httptest.NewRequest(http.MethodPut, "/federation/recording/feedfacefeed", strings.NewReader(body))
	req.SetPathValue("id", "feedfacefeed")
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("without the secret: %d", rr.Code)
	}
	req = httptest.NewRequest(http.MethodPut, "/federation/recording/feedfacefeed", strings.NewReader(body))
	req.SetPathValue("id", "feedfacefeed")
	req.Header.Set(secretHeader, "s3")
	rr = httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("with the secret: %d %s", rr.Code, rr.Body.String())
	}
	if _, ok := s.Recording("feedfacefeed"); !ok {
		t.Fatal("the region's run was not kept")
	}
	// The body's id must be the path's.
	req = httptest.NewRequest(http.MethodPut, "/federation/recording/aaaabbbbcccc", strings.NewReader(body))
	req.SetPathValue("id", "aaaabbbbcccc")
	req.Header.Set(secretHeader, "s3")
	rr = httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("mismatched id: %d", rr.Code)
	}
}
