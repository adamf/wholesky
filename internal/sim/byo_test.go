package sim

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/config"
	"github.com/adamf/jetway/pkg/node"
	"github.com/adamf/jetway/pkg/store"
	"gopkg.in/yaml.v3"
)

// Bring your own jetway, all the way: a jetway node built from nothing but
// the start pack's configuration dials the world's switch as the external
// carrier, comes up as a live peer, and a booking sold by the world's
// distribution system for one of the carrier's flights lands in the
// node's own book of record over the wire.
func TestAJetwayNodeFromThePackFliesTheCarrier(t *testing.T) {
	code := smallWorld(t).Carriers[0].Designator
	s := bootWorld(t, Options{External: []string{code}, LinkSecret: "byo"})
	pack, err := s.Pack(code)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	if err := yaml.Unmarshal([]byte(pack.ConfigYAML), cfg); err != nil {
		t.Fatal(err)
	}
	cfg.HTTP.Addr = "127.0.0.1:0" // the pack says :8080; a test cannot
	// The schedule the pack carries, saved beside the configuration as the
	// notes say.
	schedulePath := filepath.Join(t.TempDir(), cfg.Ops.Schedule)
	if err := os.WriteFile(schedulePath, []byte(pack.SSIM), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.Ops.Schedule = schedulePath
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the pack's configuration does not validate: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	n, err := node.Build(ctx, cfg, log, node.Options{LocatorSecret: []byte("byo"), SkipConsole: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(strings.Join(s.Switch.LivePeers(), ","), code) {
		if time.Now().After(deadline) {
			t.Fatalf("the node never came up on the switch as %s: %v", code, s.Switch.LivePeers())
		}
		time.Sleep(20 * time.Millisecond)
	}

	// The world sells a seat on the carrier's flight; the sell crosses the
	// switch to the node, whose gateway books it into its own store.
	f := s.Flights[code][0]
	res, err := s.Book(ctx, f, "Y", 0, "BYOJET")
	if err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(20 * time.Second)
	for {
		recs, _ := n.Store.ListPNRs(ctx, 100)
		found := false
		for _, r := range recs {
			for _, p := range r.Passengers {
				if strings.EqualFold(p.Surname, "BYOJET") {
					found = true
				}
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			msgs, _ := n.Store.ListMessages(ctx, store.MessageFilter{Limit: 20})
			t.Fatalf("the booking %s never reached the node's book; %d messages seen", res.PNR.RecordLocator, len(msgs))
		}
		time.Sleep(50 * time.Millisecond)
	}

	// The world flies the aircraft: the datalink provider reports the
	// departure to the carrier -- the node -- whose operations desk turns
	// it into the MVT the distribution system reads. The globe draws that.
	if n.Ops == nil || len(n.Ops.Legs()) != len(s.Flights[code]) {
		t.Fatalf("the node's desk holds %d legs, the world %d", len(n.Ops.Legs()), len(s.Flights[code]))
	}
	s.departure(ctx, nil, f, s.BookingDate, "N1BYO", 7)
	deadline = time.Now().Add(20 * time.Second)
	for {
		msgs, _ := s.GDSStore.ListMessages(ctx, store.MessageFilter{Limit: 500})
		found := false
		for _, m := range msgs {
			if m.Direction == store.Inbound && strings.HasPrefix(m.Kind, "MVT") && strings.Contains(string(m.Raw), code+strings.TrimLeft(f.Number, "0")) && strings.Contains(string(m.Raw), "N1BYO") {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no MVT from the node reached the distribution system; the desk filed %d movements", n.Ops.Movements())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
