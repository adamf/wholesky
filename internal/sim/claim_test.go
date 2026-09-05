package sim

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/transport"
)

// A carrier the world runs is claimed at runtime: the seat that holds it
// asks, the tenant is severed from the switch, the switch wants the token
// from then on, the lobby marks the carrier external, and a node with the
// pack's token takes the carrier's place. Unclaiming gives it back.
func TestClaimHandsARunningCarrierToANode(t *testing.T) {
	s := bootWorld(t, Options{LinkSecret: "claim"})
	code := s.Manifest.Carriers[0].Designator
	tn := s.Tenants[code]
	if _, err := s.Pack(code); err == nil {
		t.Fatal("an unclaimed carrier has no pack")
	}
	pack, err := s.Claim(code)
	if err != nil {
		t.Fatal(err)
	}
	if !s.External(code) || !tn.Severed() || pack.Token != linkToken("claim", code) {
		t.Fatalf("after the claim: external %v severed %v pack %+v", s.External(code), tn.Severed(), pack)
	}
	found := false
	for _, c := range (seatWorld{s}).Carriers() {
		if c.Code == code && c.External {
			found = true
		}
	}
	if !found {
		t.Error("the lobby does not mark the carrier external")
	}
	// The tenant's link is cut; the switch drops the carrier.
	deadline := time.Now().Add(5 * time.Second)
	for strings.Contains(strings.Join(s.Switch.LivePeers(), ","), code) {
		if time.Now().After(deadline) {
			t.Fatalf("%s is still live on the switch after the claim", code)
		}
		time.Sleep(50 * time.Millisecond)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	up := make(chan struct{}, 1)
	cl := &transport.Client{Addr: pack.Switch, Framer: transport.DefaultFramer(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Hello:     transport.Hello{Peer: code, Role: "carrier", Format: "typeb", Token: pack.Token},
		OnMessage: func(context.Context, string, []byte) error { return nil },
		OnUp: func() {
			select {
			case up <- struct{}{}:
			default:
			}
		}}
	go cl.Run(ctx)
	select {
	case <-up:
	case <-time.After(5 * time.Second):
		t.Fatal("the node with the token never came up")
	}
	deadline = time.Now().Add(5 * time.Second)
	for !strings.Contains(strings.Join(s.Switch.LivePeers(), ","), code) {
		if time.Now().After(deadline) {
			t.Fatalf("%s with the token is not live", code)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := s.Unclaim(code); err != nil {
		t.Fatal(err)
	}
	if s.External(code) || tn.Severed() {
		t.Errorf("after unclaim: external %v severed %v", s.External(code), tn.Severed())
	}
	if err := s.Unclaim("ZZ"); err == nil {
		t.Error("unclaiming a carrier with no tenant is refused")
	}
}

// A seat that releases a claimed carrier hands it back to the world rather
// than leaving it dark: the release goes through the API, and the world
// unclaims.
func TestReleasingASeatUnclaimsItsCarrier(t *testing.T) {
	s := bootWorld(t, Options{LinkSecret: "claim"})
	code := s.Manifest.Carriers[0].Designator
	_, token, err := s.Airline.Take(code, "leaver")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim(code); err != nil {
		t.Fatal(err)
	}
	if !s.External(code) || !s.Tenants[code].Severed() {
		t.Fatal("not claimed")
	}
	mux := http.NewServeMux()
	s.airlineSrv.Routes(mux)
	req := httptest.NewRequest("POST", "/carrier/"+code+"/release", nil)
	req.Header.Set("X-Seat-Token", token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("release %d %s", rec.Code, rec.Body.String())
	}
	if s.External(code) || s.Tenants[code].Severed() {
		t.Errorf("after release: external %v severed %v", s.External(code), s.Tenants[code].Severed())
	}
}
