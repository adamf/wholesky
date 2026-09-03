package host

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/baggage"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/wholesky/internal/world"
)

// Two bags left behind at LHR ride the next flight. When their flight
// lands at JFK the passengers are without them and the station opens an
// AHL each; when the rush flight lands the bags come off alone, the
// station raises an OHD each, matches them on the tag and forwards them,
// and the files close.
func TestBagOfficeRaisesFilesAndMatchesThem(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	f := world.Flight{Carrier: "BA", Number: "0117", From: "LHR", To: "JFK", DepMin: 8 * 60, ArrMin: 16 * 60}
	rush := world.Flight{Carrier: "BA", Number: "0115", From: "LHR", To: "JFK", DepMin: 10 * 60, ArrMin: 18 * 60}
	var sent []string
	var kinds []string
	tn := &Tenant{Carrier: world.Carrier{Designator: "BA", Hub: "LHR"}, log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		shortAt: map[dcs.Key][]shortBag{}, rushTags: map[dcs.Key][]shortBag{}, openAHL: map[string]*baggage.TracingFile{}, traced: map[dcs.Key]string{},
		tracingSend: func(ctx context.Context, from string, addrs []string, text, kind string) error {
			if from != "JFKLLBA" || addrs[0] != "LHRLLBA" {
				t.Errorf("tracing desk to central office: %s -> %v", from, addrs)
			}
			sent = append(sent, text)
			kinds = append(kinds, kind)
			return nil
		}}
	ex := flightKey(f, day)
	tn.recordShort(f, day, &rush, []shortBag{{tag: "0125123456", surname: "SMITH", ex: ex}, {tag: "0125123457", surname: "JONES", ex: ex}})

	if err := tn.bagOffice(ctx, f, day); err != nil {
		t.Fatal(err)
	}
	if tr := tn.Tracing(); tr.AHL != 2 || tr.Open != 2 || tr.OHD != 0 {
		t.Fatalf("after the passengers' flight: %+v", tr)
	}
	if len(sent) != 2 || !strings.Contains(sent[0], "AHL JFKBA00001") || !strings.Contains(sent[0], ".TN/0125123456001") || !strings.Contains(sent[0], ".FD/BA117/26NOV") {
		t.Errorf("AHL files: %q", sent)
	}
	if err := tn.bagOffice(ctx, rush, day); err != nil {
		t.Fatal(err)
	}
	if tr := tn.Tracing(); tr.OHD != 2 || tr.FWD != 2 || tr.Open != 0 {
		t.Fatalf("after the rush flight: %+v", tr)
	}
	if strings.Join(kinds, ",") != "AHL,AHL,OHD,FWD,OHD,FWD" {
		t.Errorf("kinds %v", kinds)
	}
	fwd, err := baggage.ParseTracing(sent[3])
	if err != nil {
		t.Fatal(err)
	}
	if fwd.Kind != baggage.KindFWD || fwd.Reference != "JFKBA00001" || fwd.Matches != "JFKBA00003" || fwd.Tags[0].Number != "0125123456" {
		t.Errorf("FWD %+v", fwd)
	}
	if line := tn.tracedFor(&dcs.Flight{Flight: ex.Flight, Date: ex.Date, Board: ex.Board}); !strings.Contains(line, "2 matched off BA115") {
		t.Errorf("panel line %q", line)
	}
	// Nothing left for a flight with no story.
	if err := tn.bagOffice(ctx, rush, day); err != nil || len(sent) != 6 {
		t.Errorf("a quiet arrival sends nothing: %d %v", len(sent), err)
	}
}
