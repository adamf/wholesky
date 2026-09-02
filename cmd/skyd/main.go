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
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
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
		maxMsgs   = flag.Int("max-messages", 20000, "message-log cap per node; 0 is unbounded")
		maxRecs   = flag.Int("max-records", 20000, "record cap per node; 0 is unbounded")
		tMaxMsgs  = flag.Int("tenant-max-messages", 0, "message cap per carrier tenant; 0 inherits -max-messages")
		tMaxRecs  = flag.Int("tenant-max-records", 0, "record cap per carrier tenant; 0 inherits -max-records")
		avsEvery  = flag.Duration("avs-interval", 0, "availability rebroadcast interval; 0 uses the default")
		gdsCount  = flag.Int("gds", 0, "how many distribution systems to run; 0 runs all five")
		statsSnap = flag.String("stats-snapshot", "", "persist the stats rings here across restarts; empty disables")
		pprofAddr = flag.String("pprof", "", "serve net/http/pprof here (e.g. 127.0.0.1:6060); empty disables")
		verbose   = flag.Bool("v", false, "debug logging")

		role      = flag.String("role", "all", "all | core | gds | region: which slice of the world this machine runs")
		coreURL   = flag.String("core-url", "", "gds/region: the core machine's HTTP base, e.g. http://core.process.app.internal:8080")
		selfURL   = flag.String("self-url", "", "gds/region: this machine's HTTP base as the core can reach it")
		advertise = flag.String("advertise", "", "core: the hostname peers dial for switch links")
		gdsDesig  = flag.String("gds-designator", "", "gds: which distribution system this machine runs, e.g. 1G")
		shard     = flag.Int("shard", 0, "region: this machine's shard index")
		shards    = flag.Int("shards", 1, "region: how many region shards exist")
		linkBind  = flag.String("link-bind", "", "core: host the switch listeners bind; empty binds loopback")
		gdsList   = flag.String("gds-list", "", "region: comma-separated designators of the GDSes this deployment runs; empty means all five")
		memLimit  = flag.String("memlimit", "", "soft memory limit for the Go runtime, e.g. 3200MiB; empty leaves GOMEMLIMIT alone")
		tenantDSN = flag.String("tenant-dsn", "", "all/region: Postgres DSN backing every carrier tenant's records (one database, one node per carrier, purged each sim day); $NAME reads the environment")
	)
	flag.Parse()
	if *pprofAddr != "" {
		go func() {
			// The default mux carries the pprof handlers via the import.
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				slog.Error("pprof listener ended", "err", err)
			}
		}()
	}

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

	if *memLimit != "" {
		if n, err := parseByteSize(*memLimit); err == nil {
			debug.SetMemoryLimit(n)
		} else {
			return fmt.Errorf("bad -memlimit: %w", err)
		}
	}
	opts := sim.Options{
		Carriers: *carriers, Console: *console, Warp: *warp, Log: log,
		MaxMessages: *maxMsgs, MaxRecords: *maxRecs, AVSInterval: *avsEvery,
		TenantMaxMessages: *tMaxMsgs, TenantMaxRecords: *tMaxRecs,
		GDSCount:      *gdsCount,
		StatsSnapshot: *statsSnap,
		LinkBind:      *linkBind,
		TenantDSN:     *tenantDSN,
	}
	if *gdsList != "" {
		for _, d := range strings.Split(*gdsList, ",") {
			if d = strings.TrimSpace(strings.ToUpper(d)); d != "" {
				opts.GDSList = append(opts.GDSList, d)
			}
		}
	}
	self := *selfURL
	if self == "" {
		if id := os.Getenv("FLY_MACHINE_ID"); id != "" && os.Getenv("FLY_APP_NAME") != "" {
			self = "http://" + id + ".vm." + os.Getenv("FLY_APP_NAME") + ".internal:8080"
		}
	}

	switch *role {
	case "all":
		s, err := sim.Boot(ctx, &m, opts)
		if err != nil {
			return err
		}
		defer s.Stop()
		total := 0
		for _, fs := range s.Flights {
			total += len(fs)
		}
		log.Info("fabric up", "links", len(s.Switch.LivePeers()),
			"flights_per_day", total,
			"console", "http://"+*console, "eye", "http://"+*console+"/eye")
		go s.FlyDay(ctx)
		go s.Demand(ctx, *demand, *seed)
		return watch(ctx, log, func() (int, int64) {
			return len(s.Switch.LivePeers()), s.Movements.Load()
		})

	case "core":
		c, err := sim.BootCore(ctx, &m, opts, *advertise)
		if err != nil {
			return err
		}
		defer c.Sim.Stop()
		log.Info("core up", "console", "http://"+*console,
			"advertise", *advertise)
		return watch(ctx, log, func() (int, int64) {
			return len(c.Sim.Switch.LivePeers()), c.Sim.Movements.Load()
		})

	case "gds":
		if *gdsDesig == "" || *coreURL == "" {
			return fmt.Errorf("-role gds needs -gds-designator and -core-url")
		}
		g, err := sim.BootGDS(ctx, &m, opts, *coreURL, self, *gdsDesig)
		if err != nil {
			return err
		}
		defer g.Sim.Stop()
		go g.Sim.Demand(ctx, *demand, *seed)
		log.Info("gds up", "designator", *gdsDesig, "core", *coreURL, "self", self)
		return serveAndWatch(ctx, log, *console, g.Mux, func() (int, int64) {
			return 1, g.Sim.DemBooked.Load()
		})

	case "region":
		if *coreURL == "" {
			return fmt.Errorf("-role region needs -core-url")
		}
		r, err := sim.BootRegion(ctx, &m, opts, *coreURL, self, *shard, *shards)
		if err != nil {
			return err
		}
		defer r.Sim.Stop()
		go r.Sim.FlyDay(ctx)
		log.Info("region up", "shard", *shard, "core", *coreURL, "self", self)
		return serveAndWatch(ctx, log, *console, r.Mux, func() (int, int64) {
			return len(r.Sim.Tenants), r.Sim.Movements.Load()
		})
	}
	return fmt.Errorf("unknown role %q", *role)
}

// parseByteSize reads "3200MiB" style limits.
func parseByteSize(s string) (int64, error) {
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "GiB"):
		mult, s = 1<<30, strings.TrimSuffix(s, "GiB")
	case strings.HasSuffix(s, "MiB"):
		mult, s = 1<<20, strings.TrimSuffix(s, "MiB")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return n * mult, nil
}

// watch is the run loop: a heartbeat log until the context ends.
func watch(ctx context.Context, log *slog.Logger, snap func() (int, int64)) error {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("stopped")
			return nil
		case <-tick.C:
			links, moves := snap()
			log.Info("sky", "links", links, "movements", moves)
		}
	}
}

// serveAndWatch serves a machine-local mux and heartbeats.
func serveAndWatch(ctx context.Context, log *slog.Logger, addr string,
	mux *http.ServeMux, snap func() (int, int64)) error {
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		shctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shctx) //nolint:errcheck
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("shard http stopped", "err", err)
		}
	}()
	return watch(ctx, log, snap)
}
