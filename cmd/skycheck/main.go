// skycheck asks a running world whether its laws hold, and exits non-zero
// if they do not. It is the release gate: deploy to staging, let the day
// fly, run skycheck against it.
//
//	go run ./cmd/skycheck https://wholesky-demo.fly.dev
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/adamf/wholesky/internal/sim"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: skycheck <base-url>")
		os.Exit(2)
	}
	base := strings.TrimRight(os.Args[1], "/")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(base + "/invariants.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "skycheck:", err)
		os.Exit(2)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "skycheck: %s\n", resp.Status)
		os.Exit(2)
	}
	var v struct {
		sim.Invariants
		Unreachable []string `json:"unreachable"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&v); err != nil {
		fmt.Fprintln(os.Stderr, "skycheck:", err)
		os.Exit(2)
	}
	fmt.Printf("%d shards, %d cabins, %d seats sold, %d oversold\n", v.Shards, v.Cabins, v.Sold, len(v.Oversold))
	for _, o := range v.Oversold {
		fmt.Printf("OVERSOLD %s%s %s %s %s: %d sold of %d\n", o.Carrier, o.Flight, o.Date, o.Board, o.Compartment, o.Sold, o.Seats)
	}
	for _, u := range v.Unreachable {
		fmt.Printf("UNREACHABLE shard %s\n", u)
	}
	if !v.OK() || len(v.Unreachable) > 0 {
		os.Exit(1)
	}
}
