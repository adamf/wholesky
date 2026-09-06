package airline

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The operations page reflects the carrier's code into its markup, so the
// code is a designator or nothing: markup in the path is a 404, not a
// script in the page, and the page arrives with a content security policy.
func TestOpsPageReflectsOnlyADesignator(t *testing.T) {
	s := &Server{}
	mux := http.NewServeMux()
	s.Routes(mux)
	for _, p := range []string{"/ops/%3Cimg%20src=x%20onerror=alert(1)%3E", "/ops/BA%22%3E%3Cscript%3E", "/ops/ABCD", "/ops/A"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", p, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<img") || strings.Contains(rec.Body.String(), "<script>") {
			t.Errorf("GET %s reflected markup", p)
		}
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ops/ba", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `const CODE="BA"`) {
		t.Fatalf("GET /ops/ba: %d; body lacks the code", rec.Code)
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("no content security policy on the page: %q", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("pages are served without nosniff")
	}
}

// Tokens compare in constant time and an empty token never matches.
func TestTokenMatch(t *testing.T) {
	if !tokenMatch("abc", "abc") || tokenMatch("abc", "abd") || tokenMatch("", "") || tokenMatch("abc", "") {
		t.Fatal("tokenMatch is wrong")
	}
}

// A subscription's key is the caller's to choose; cancelling one leaves
// nothing behind under it.
func TestSubscribeLeavesNoEmptyTables(t *testing.T) {
	r := New(0)
	for i := 0; i < 1000; i++ {
		_, cancel := r.Subscribe("ZZ" + string(rune('A'+i%26)) + string(rune('A'+i/26%26)))
		cancel()
	}
	r.mu.Lock()
	n := len(r.subs)
	r.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d empty subscription tables left behind", n)
	}
}
