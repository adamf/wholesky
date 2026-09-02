package fill

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/pnr"
	"github.com/adamf/jetway/pkg/store"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/adamf/wholesky/internal/world"
)

// Not a test of behaviour: a measurement. With JETWAY_TEST_DSN and
// FILL_SIZE_WORLD set, fills the largest carrier of that world into the test
// database and reports rows, bytes on disk and time, so the cost of a full
// day can be read off before it is written to a shared cluster.
func TestMeasureFilledCarrierOnPostgres(t *testing.T) {
	dsn, worldPath := os.Getenv("JETWAY_TEST_DSN"), os.Getenv("FILL_SIZE_WORLD")
	if dsn == "" || worldPath == "" {
		t.Skip("JETWAY_TEST_DSN and FILL_SIZE_WORLD not set")
	}
	ctx := context.Background()
	raw, err := os.ReadFile(worldPath)
	if err != nil {
		t.Fatal(err)
	}
	var m world.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, f := range m.Flights {
		count[f.Carrier]++
	}
	biggest := ""
	for c, n := range count {
		if biggest == "" || n > count[biggest] {
			biggest = c
		}
	}
	sub := &world.Manifest{}
	for _, f := range m.Flights {
		if f.Carrier == biggest {
			sub.Flights = append(sub.Flights, f)
		}
	}
	pg, err := store.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE pnr CASCADE`); err != nil {
		t.Fatal(err)
	}
	node := pg.Node(biggest)
	started := time.Now()
	plan, err := Day(ctx, sub, Options{LoadFactor: 0.85, Seed: 1, Day: time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)},
		func(ctx context.Context, carrier string, recs []*pnr.PNR) error {
			return node.LoadPNRs(ctx, recs, "fill")
		})
	if err != nil {
		t.Fatal(err)
	}
	took := time.Since(started)
	var rows int64
	var pnrBytes, eventBytes, dbBytes int64
	pool.QueryRow(ctx, `SELECT count(*) FROM pnr`).Scan(&rows)                         //nolint:errcheck
	pool.QueryRow(ctx, `SELECT pg_total_relation_size('pnr')`).Scan(&pnrBytes)         //nolint:errcheck
	pool.QueryRow(ctx, `SELECT pg_total_relation_size('pnr_event')`).Scan(&eventBytes) //nolint:errcheck
	pool.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&dbBytes)   //nolint:errcheck
	t.Logf("%s: %d flights, %d records, %d passengers of %d seats (%d connecting) in %s", biggest, plan.Flights, plan.Records, plan.Passengers, plan.Seats, plan.Connecting, took.Round(time.Millisecond))
	t.Logf("rows=%d pnr=%.1fMB (%.0f B/row) events=%.1fMB db=%.1fMB", rows, float64(pnrBytes)/1e6, float64(pnrBytes)/float64(max(rows, 1)), float64(eventBytes)/1e6, float64(dbBytes)/1e6)
}
