// Command skyd boots a world.
//
// It reads a compiled manifest and stands the topology up the way the real
// one is shaped: a message switch in the middle (a Jetway node in relay
// mode), carrier reservation systems as tenants each dialling one circuit,
// and a GDS reaching every carrier through its single switch link -- AIRIMP
// over Type B and PADIS over EDIFACT, routed either way. All of it over real
// TCP on loopback.
//
//	worldc -countries "United Kingdom,France" -o europe.json
//	skyd -world europe.json -carriers 12 -demand 60 -warp 240
//
// The flight day runs against the wall clock multiplied by -warp; -demand
// books continuously at roughly that many bookings per minute. The switch's
// console is Jetway's own, where every message can be opened field by field.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/adamf/wholesky/internal/sim"
	"github.com/adamf/wholesky/internal/world"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "skyd:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		worldPath = flag.String("world", "world.json", "compiled manifest to boot")
		carriers  = flag.Int("carriers", 12, "how many carriers to run, largest first; 0 for all")
		warp      = flag.Int("warp", 60, "sim minutes per real minute for the flight day")
		demand    = flag.Int("demand", 30, "bookings per minute; 0 disables demand")
		seed      = flag.Int64("seed", 1, "demand seed")
		console   = flag.String("console", "127.0.0.1:8090", "switch console address")
		verbose   = flag.Bool("v", false, "debug logging")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	b, err := os.ReadFile(*worldPath)
	if err != nil {
		return err
	}
	var m world.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("manifest %s: %w", *worldPath, err)
	}
	sort.Slice(m.Carriers, func(i, j int) bool { return m.Carriers[i].Routes > m.Carriers[j].Routes })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := sim.Boot(ctx, &m, sim.Options{
		Carriers: *carriers, Console: *console, Log: log,
	})
	if err != nil {
		return err
	}
	defer s.Stop()
	total := 0
	for _, fs := range s.Flights {
		total += len(fs)
	}
	log.Info("fabric up", "links", len(s.Switch.LivePeers()),
		"flights_per_day", total, "console", "http://"+*console)

	go s.FlyDay(ctx, *warp)
	go s.Demand(ctx, *demand, *seed)

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped")
			return nil
		case <-tick.C:
			log.Info("sky", "links", len(s.Switch.LivePeers()),
				"movements", s.Movements.Load())
		}
	}
}
