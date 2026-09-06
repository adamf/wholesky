package sim

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The world's control plane -- registration, tokens, saved state, the
// feeds between machines -- answers only to a caller holding the world's
// secret; a stranger on the internet gets nothing, and gets it in
// constant time.
func TestControlPlaneNeedsTheSecret(t *testing.T) {
	s := bootWorld(t, Options{LinkSecret: "the-worlds-secret"})
	called := false
	h := s.requireSecret(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) })
	for _, got := range []string{"", "wrong", "the-worlds-secre"} {
		req := httptest.NewRequest(http.MethodPost, "/federation/token", nil)
		if got != "" {
			req.Header.Set(secretHeader, got)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != http.StatusForbidden || called {
			t.Fatalf("secret %q: %d, called=%v; want 403 and no call", got, rec.Code, called)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/federation/token", nil)
	req.Header.Set(secretHeader, "the-worlds-secret")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNoContent || !called {
		t.Fatalf("the secret was refused: %d", rec.Code)
	}

	// The revenue feed is written the same way.
	req = httptest.NewRequest(http.MethodPost, "/shard/revenue", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	s.serveRevenue(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an unsigned revenue feed was taken: %d", rec.Code)
	}
}

// A URL a stranger supplies is fetched only if the internet could reach
// it: loopback, private ranges, link-local, Fly's 6PN and .internal names
// are refused, unless the world was told peers live there.
func TestStrangersURLsMustBePublic(t *testing.T) {
	s := bootWorld(t, Options{})
	s.allowPrivate = false
	ctx := context.Background()
	for _, u := range []string{
		"http://127.0.0.1:8080", "http://localhost:6060/debug/pprof/", "http://10.1.2.3/", "http://192.168.1.1/",
		"http://[fdaa:0:1::2]:8080/", "http://[::1]/", "http://169.254.169.254/latest/meta-data/",
		"http://core.process.wholesky-demo.internal:8080", "http://100.64.0.1/", "ftp://example.com/", "http://user:pw@8.8.8.8/",
	} {
		if got, err := s.checkedURL(ctx, u); err == nil {
			t.Errorf("%s was accepted as %s", u, got)
		}
	}
	if got, err := s.checkedURL(ctx, "https://8.8.8.8:7000/x/"); err != nil || got != "https://8.8.8.8:7000/x" {
		t.Errorf("a public address was refused: %v %q", err, got)
	}
	if err := s.publicHostPort(ctx, "[fdaa:0:1::2]:7000"); err == nil {
		t.Error("a 6PN switch address was accepted")
	}
	if err := s.publicHostPort(ctx, "8.8.8.8:7000"); err != nil {
		t.Errorf("a public switch address was refused: %v", err)
	}
	s.allowPrivate = true
	if _, err := s.checkedURL(ctx, "http://127.0.0.1:8080"); err != nil {
		t.Errorf("with private peers allowed, loopback was refused: %v", err)
	}
}

// The node consoles are a window for everyone and a door for the seat: a
// stranger reads, and is told so in the JSON the console shows; the
// carrier's seat holder books, cancels and boards; nobody reaches the
// admin surface. The console page arrives carrying the seat's token.
func TestNodeConsolesAnswerToTheSeat(t *testing.T) {
	s := bootWorld(t, Options{})
	var code string
	for c := range s.Tenants {
		code = c
		break
	}
	if code == "" {
		t.Skip("no tenants in the small world")
	}
	do := func(method, path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		if token != "" {
			req.Header.Set("X-Seat-Token", token)
		}
		rec := httptest.NewRecorder()
		s.serveNodeConsole(rec, req)
		return rec
	}
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/node/" + code + "/api/book", http.StatusForbidden},
		{http.MethodPost, "/node/" + code + "/api/pnr/ABC123/cancel", http.StatusForbidden},
		{http.MethodPost, "/node/" + code + "/api/admin/retire", http.StatusForbidden},
		{http.MethodGet, "/node/" + code + "/api/admin/export", http.StatusForbidden},
		{http.MethodGet, "/node/" + code + "/api/status", http.StatusOK},
	} {
		rec := do(tc.method, tc.path, "")
		if rec.Code != tc.want {
			t.Errorf("%s %s: %d, want %d", tc.method, tc.path, rec.Code, tc.want)
		}
		if tc.want == http.StatusForbidden && !strings.Contains(rec.Header().Get("Content-Type"), "json") {
			t.Errorf("%s %s refused without JSON: %q", tc.method, tc.path, rec.Header().Get("Content-Type"))
		}
	}
	_, token, err := s.Airline.Take(code, "Edith")
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(http.MethodPost, "/node/"+code+"/api/pnr/ABC123/cancel", token); rec.Code == http.StatusForbidden {
		t.Fatalf("the seat holder was refused their own console: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPost, "/node/"+code+"/api/admin/retire", token); rec.Code != http.StatusForbidden {
		t.Fatalf("the seat holder reached the admin surface: %d", rec.Code)
	}
	page := do(http.MethodGet, "/node/"+code+"/", "")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `localStorage.getItem("seat:"+c)`) || !strings.Contains(page.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("console page without the seat script: %d %q", page.Code, page.Header().Get("Content-Type"))
	}
	if body := do(http.MethodGet, "/node/"+code+"/api/status", "").Body.String(); strings.Contains(body, "<script>") {
		t.Fatal("the seat script leaked into a JSON response")
	}
}

// A hello from a machine the internet cannot reach is refused before the
// world fetches anything from it, and a visitor may attempt one join at a
// time.
func TestStrangersHelloIsVettedAndPaced(t *testing.T) {
	s := bootWorld(t, Options{})
	s.allowPrivate = false
	req := httptest.NewRequest(http.MethodPost, "/federation/world",
		strings.NewReader(`{"code":"9Z","token":"t","name":"evil","url":"http://127.0.0.1:9999","switch_addr":"127.0.0.1:7000"}`))
	req.RemoteAddr = "203.0.113.7:1234"
	rec := httptest.NewRecorder()
	s.serveJoinWorld(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "refused") {
		t.Fatalf("a loopback hello was not refused: %d %s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/federation/world", strings.NewReader(`{"code":"9Y","token":"t","url":"http://8.8.8.8/"}`))
	req.RemoteAddr = "203.0.113.7:1234"
	rec = httptest.NewRecorder()
	s.serveJoinWorld(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("a second join from the same address was not paced: %d", rec.Code)
	}
}
