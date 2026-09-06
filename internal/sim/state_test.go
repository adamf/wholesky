package sim

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A restart does not forget the people: the seat, its token and its manual
// departments, the carrier handed to a node and where the node is, all come
// back from the state file, and the handed carrier is severed again with
// its token demanded.
func TestStateSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	s, err := Boot(ctx, m, Options{Log: log, AllowPrivatePeers: true, StateFile: path, LinkSecret: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	code := m.Carriers[0].Designator
	_, token, err := s.Airline.Take(code, "Adam")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Airline.SetManual(code, token, "ops", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(code); err != nil {
		t.Fatal(err)
	}
	s.SetNodeURL(code, "http://node.example:8080")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 && time.Since(time.Now()) < time.Second {
			if string(b) != "" && containsAll(string(b), code, "Adam", "node.example") {
				break
			}
		}
		if time.Now().After(deadline) {
			b, _ := os.ReadFile(path)
			t.Fatalf("state never written: %s", b)
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.Stop()

	s2, err := Boot(ctx, m, Options{Log: log, AllowPrivatePeers: true, StateFile: path, LinkSecret: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Stop()
	seat, held := s2.Airline.Seat(code)
	if !held || seat.Holder != "Adam" || !seat.Manual["ops"] || !s2.Airline.Authorised(code, token) {
		t.Fatalf("seat after restart: %+v held %v authorised %v", seat, held, s2.Airline.Authorised(code, token))
	}
	if !s2.External(code) || !s2.Tenants[code].Severed() {
		t.Errorf("the handed carrier is not handed after restart: external %v severed %v", s2.External(code), s2.Tenants[code].Severed())
	}
	if s2.NodeURL(code) != "http://node.example:8080" {
		t.Errorf("node url %q", s2.NodeURL(code))
	}
	if pack, err := s2.Pack(code); err != nil || pack.Token != linkToken("keep", code) {
		t.Errorf("pack after restart: %+v %v", pack, err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
