package sim

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/transport"
)

// Claiming across machines: the region runs the tenant, the core runs the
// switch. The region's claim severs its tenant and asks the core to demand
// the token; the pack names the core's switch (the internal address, since
// this world publishes none); a node with the token comes up on the core.
func TestClaimAcrossMachinesSetsTheTokenOnTheCore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m := smallWorld(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	coreAddr := freeAddr(t)
	core, err := BootCore(ctx, m, Options{Console: coreAddr, Log: log, LinkSecret: "fed"}, "127.0.0.1")
	if err != nil {
		t.Fatalf("core: %v", err)
	}
	defer core.Sim.Stop()
	regAddr := freeAddr(t)
	r, err := BootRegion(ctx, m, Options{Log: log, LinkSecret: "fed"}, "http://"+coreAddr, "http://"+regAddr, 0, 1)
	if err != nil {
		t.Fatalf("region: %v", err)
	}
	defer r.Sim.Stop()
	serveMux(t, ctx, regAddr, r.Mux)

	code := m.Carriers[0].Designator
	live := func() bool { return strings.Contains(strings.Join(core.Sim.Switch.LivePeers(), ","), code) }
	deadline := time.Now().Add(30 * time.Second)
	for !live() {
		if time.Now().After(deadline) {
			t.Fatal("the region's tenant never dialled the core")
		}
		time.Sleep(50 * time.Millisecond)
	}
	pack, err := r.Sim.Claim(code)
	if err != nil {
		t.Fatal(err)
	}
	if pack.Switch == "" || pack.Token != linkToken("fed", code) {
		t.Fatalf("pack from the region: switch %q token %q", pack.Switch, pack.Token)
	}
	deadline = time.Now().Add(10 * time.Second)
	for live() {
		if time.Now().After(deadline) {
			t.Fatal("the core still counts the severed tenant live")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !core.Sim.External(code) {
		t.Error("the core does not know the carrier is external")
	}
	up := make(chan struct{}, 1)
	nctx, ncancel := context.WithCancel(ctx)
	defer ncancel()
	cl := &transport.Client{Addr: pack.Switch, Framer: transport.DefaultFramer(), Log: log,
		Hello:     transport.Hello{Peer: code, Role: "carrier", Format: "typeb", Token: pack.Token},
		OnMessage: func(context.Context, string, []byte) error { return nil },
		OnUp: func() {
			select {
			case up <- struct{}{}:
			default:
			}
		}}
	go cl.Run(nctx)
	select {
	case <-up:
	case <-time.After(10 * time.Second):
		t.Fatalf("the node never came up on the core at %s", pack.Switch)
	}
	deadline = time.Now().Add(10 * time.Second)
	for !live() {
		if time.Now().After(deadline) {
			t.Fatal("the node with the token is not live on the core")
		}
		time.Sleep(50 * time.Millisecond)
	}
	ncancel()
	if err := r.Sim.Unclaim(code); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for core.Sim.External(code) {
		if time.Now().After(deadline) {
			t.Fatal("the core still holds the carrier external after unclaim")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
