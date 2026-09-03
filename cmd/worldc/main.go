// Command worldc compiles a world.
//
// It reads the vendored OpenFlights snapshot and writes a deterministic
// manifest: the airports, carriers and daily flights the simulation will run.
//
//	worldc -countries "United Kingdom,France,Germany" -o europe.json
//	worldc -scale 0.05 -o small-sky.json
//	worldc -o whole-sky.json
//	worldc -bts data/bts/2025-11-26.csv -date 2025-11-26 -o thanksgiving.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // a replay converts local schedules to UTC wherever it is built

	"github.com/adamf/wholesky/internal/world"
)

func main() {
	var (
		dataDir   = flag.String("data", "data", "directory holding the OpenFlights snapshot")
		out       = flag.String("o", "world.json", "manifest to write")
		seed      = flag.Int64("seed", 1, "world seed; same seed, same world")
		scale     = flag.Float64("scale", 1.0, "sigma: fraction of the sky to keep, majors first")
		countries = flag.String("countries", "", "comma-separated country filter, empty for the planet")
		carriers  = flag.Int("carriers", 0, "cap on carrier count, 0 for none")
		bts       = flag.String("bts", "", "replay: a BTS on-time performance CSV; compiles the recorded day instead of the synthetic one")
		date      = flag.String("date", "", "replay: the day to compile from the BTS file, YYYY-MM-DD")
		ssimOut   = flag.String("ssim", "", "also write the schedule as an SSIM chapter 7 file, one carrier after another")
	)
	flag.Parse()

	var cs []string
	if *countries != "" {
		for _, c := range strings.Split(*countries, ",") {
			cs = append(cs, strings.TrimSpace(c))
		}
	}
	var m *world.Manifest
	var err error
	if *bts != "" {
		m, err = world.CompileReplay(world.ReplayOptions{DataDir: *dataDir, BTS: *bts, Date: *date})
	} else {
		m, err = world.Compile(world.CompileOptions{
			DataDir: *dataDir, Seed: *seed, Scale: *scale,
			Countries: cs, MaxCarriers: *carriers,
		})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "worldc:", err)
		os.Exit(1)
	}
	m.GeneratedAt = time.Now().UTC()

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "worldc:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", " ")
	if err := enc.Encode(m); err != nil {
		fmt.Fprintln(os.Stderr, "worldc:", err)
		os.Exit(1)
	}
	f.Close()
	fmt.Printf("%s: %s\n", *out, m.Stats())
	if *ssimOut != "" {
		sf, err := os.Create(*ssimOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, "worldc:", err)
			os.Exit(1)
		}
		if err := world.WriteSSIM(sf, m, world.SellingDate(m)); err != nil {
			fmt.Fprintln(os.Stderr, "worldc:", err)
			os.Exit(1)
		}
		sf.Close()
		fmt.Printf("%s: %d flights as SSIM\n", *ssimOut, len(m.Flights))
	}
}
