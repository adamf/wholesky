package eye

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adamf/wholesky/internal/world"
)

func testEye() (*Eye, *http.ServeMux) {
	m := &world.Manifest{
		Airports: []world.Airport{{IATA: "LHR", Lat: 51.47, Lon: -0.45}},
		Carriers: []world.Carrier{{Designator: "BA", Hub: "LHR"}},
	}
	e := New(m, 60)
	mux := http.NewServeMux()
	e.Routes(mux)
	return e, mux
}

// The time control drives the hook and refuses nonsense; a world without the
// hook says so rather than pretending.
func TestTimeControl(t *testing.T) {
	e, mux := testEye()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/eye/time", strings.NewReader(`{"warp":300}`)))
	if rec.Code != http.StatusNotImplemented {
		t.Errorf("without a hook: %d, want 501", rec.Code)
	}

	var got int
	e.SetWarp = func(w int) error { got = w; return nil }
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/eye/time", strings.NewReader(`{"warp":300}`)))
	if rec.Code != 200 || got != 300 {
		t.Errorf("code=%d hook got %d, want 200/300: %s", rec.Code, got, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/eye/time", strings.NewReader(`not json`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("garbage body: %d, want 400", rec.Code)
	}
}

// The world payload names the distribution systems, so the logical web knows
// its hubs -- with three GDSes running, treating only 1G as one drew the
// other two as carriers.
func TestWorldPayloadCarriesHubs(t *testing.T) {
	e, mux := testEye()
	e.Hubs = []string{"1G", "1S", "1A"}
	e.WarpNow = func() int { return 300 }

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/eye/world.json", nil))
	if rec.Code != 200 {
		t.Fatalf("world.json: %d", rec.Code)
	}
	var out struct {
		Hubs []string `json:"hubs"`
		Warp int      `json:"warp"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Hubs) != 3 || out.Hubs[1] != "1S" {
		t.Errorf("hubs = %v", out.Hubs)
	}
	if out.Warp != 300 {
		t.Errorf("warp = %d, want the live rate", out.Warp)
	}
}
