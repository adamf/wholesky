package sim

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/transport"
	"gopkg.in/yaml.v3"
)

// Bring your own jetway: a world booted with a carrier external runs no
// tenant for it, hands out a start pack whose configuration dials the
// switch as the carrier with a token, and the switch admits a node with
// that token and refuses one without.
func TestExternalCarrierGetsAPackAndTheSwitchChecksItsToken(t *testing.T) {
	code := smallWorld(t).Carriers[0].Designator
	s := bootWorld(t, Options{External: []string{code}, LinkSecret: "dev"})
	if _, ok := s.Tenants[code]; ok {
		t.Fatalf("%s was booted as a tenant", code)
	}
	other := s.Manifest.Carriers[1].Designator
	if _, err := s.Pack(other); err == nil {
		t.Errorf("a carrier the world runs has no pack to give")
	}
	pack, err := s.Pack(code)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Token == "" || pack.Switch == "" || pack.Flights == 0 || !strings.Contains(pack.SSIM, "3 ") {
		t.Fatalf("pack %+v", pack)
	}
	var cfg struct {
		Identity struct {
			Designator string `yaml:"designator"`
		} `yaml:"identity"`
		Peers []struct {
			Name   string `yaml:"name"`
			Token  string `yaml:"token"`
			Egress struct {
				Type string `yaml:"type"`
				Addr string `yaml:"addr"`
				Via  string `yaml:"via"`
			} `yaml:"egress"`
		} `yaml:"peers"`
	}
	if err := yaml.Unmarshal([]byte(pack.ConfigYAML), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Identity.Designator != code || len(cfg.Peers) < 2 || cfg.Peers[0].Egress.Type != "link_dial" || cfg.Peers[0].Token != pack.Token || cfg.Peers[0].Egress.Addr != pack.Switch || cfg.Peers[1].Egress.Via != cfg.Peers[0].Name {
		t.Fatalf("config %+v", cfg)
	}
	// The token is the same one the switch expects: deterministic under the secret.
	if pack.Token != linkToken("dev", code) {
		t.Error("the pack's token is not the switch's")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dial := func(token string) chan struct{} {
		up := make(chan struct{}, 1)
		cl := &transport.Client{Addr: pack.Switch, Framer: transport.DefaultFramer(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
			Hello:     transport.Hello{Peer: code, Role: "carrier", Format: "typeb", Token: token},
			OnMessage: func(context.Context, string, []byte) error { return nil },
			OnUp: func() {
				select {
				case up <- struct{}{}:
				default:
				}
			}}
		go cl.Run(ctx)
		return up
	}
	// Without the token the link is refused after the hello: the switch
	// never counts the carrier live, however often the node reconnects.
	dial("")
	time.Sleep(700 * time.Millisecond)
	if strings.Contains(strings.Join(s.Switch.LivePeers(), ","), code) {
		t.Fatal("the switch admitted a node without the token")
	}
	select {
	case <-dial(pack.Token):
	case <-time.After(5 * time.Second):
		t.Fatal("the switch never admitted the node with the token")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(strings.Join(s.Switch.LivePeers(), ","), code) {
		if time.Now().After(deadline) {
			t.Fatalf("%s is not live on the switch: %v", code, s.Switch.LivePeers())
		}
		time.Sleep(20 * time.Millisecond)
	}
}
