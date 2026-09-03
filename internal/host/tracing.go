package host

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/baggage"
	"github.com/adamf/jetway/pkg/dcs"
	"github.com/adamf/wholesky/internal/world"
)

// The bag office. A bag short-shipped at the door is rushed on the next
// flight (rushBags); at the other end its passenger arrives without it,
// and the arrival station's tracing desk opens an AHL file for it. When
// the rush flight lands, the bag comes off without a passenger and the
// same desk raises an OHD. The office matches the two on the tag, sends
// the FWD that delivers the bag, and closes the file. The messages are
// jetway's pkg/baggage tracing profile -- WorldTracer's own formats are
// not published -- and go from the station's tracing desk to the
// carrier's central baggage office, where a real carrier's tracing
// system would read them.

// deptTracing is the Type B department a station's baggage tracing desk
// answers to.
const deptTracing = "LL"

// shortBag is one bag the door left behind, and the departure it was
// meant to be on.
type shortBag struct {
	tag, surname string
	ex           dcs.Key
}

// Tracing is the bag office's day so far.
type Tracing struct {
	// AHL, OHD and FWD are the files raised; Open is how many AHLs have
	// not yet met their bag.
	AHL, OHD, FWD, Open int
}

// Tracing is what the bag office has done today.
func (t *Tenant) Tracing() Tracing {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.tracing
}

// recordShort remembers a departure's short-shipped bags: the arrival
// station will miss them when the flight lands, and expects them off the
// rush flight if there is one.
func (t *Tenant) recordShort(f world.Flight, day time.Time, rush *world.Flight, bags []shortBag) {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	key := flightKey(f, day)
	t.shortAt[key] = append(t.shortAt[key], bags...)
	if rush != nil {
		rk := flightKey(*rush, day)
		t.rushTags[rk] = append(t.rushTags[rk], bags...)
	}
}

// bagOffice is the arrival station's tracing desk when a flight lands:
// files for the passengers whose bags did not come, and for the bags that
// came without their passengers, matched where they meet.
func (t *Tenant) bagOffice(ctx context.Context, f world.Flight, day time.Time) error {
	key := flightKey(f, day)
	t.groundMu.Lock()
	short := t.shortAt[key]
	delete(t.shortAt, key)
	arrived := t.rushTags[key]
	delete(t.rushTags, key)
	t.groundMu.Unlock()
	if len(short) == 0 && len(arrived) == 0 {
		return nil
	}
	wire := strings.ToUpper(day.Format("02Jan"))
	from := t.stationAddress(f.To, deptTracing)
	to := []string{t.stationAddress(t.Carrier.Hub, deptTracing)}
	var firstErr error
	send := func(file *baggage.TracingFile) {
		text, err := baggage.BuildTracing(file)
		if err == nil {
			if t.tracingSend != nil {
				err = t.tracingSend(ctx, from, to, text, string(file.Kind))
			} else {
				err = t.sendTypeBFrom(ctx, from, to, text, string(file.Kind))
			}
		}
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, b := range short {
		t.groundMu.Lock()
		t.tracingSeq++
		file := &baggage.TracingFile{
			Kind: baggage.KindAHL, Reference: fmt.Sprintf("%s%s%05d", f.To, t.Carrier.Designator, t.tracingSeq),
			Station: f.To, Carrier: t.Carrier.Designator, Tags: []baggage.Tag{{Number: b.tag, Count: 1}}, Surname: b.surname,
			ColourType: "BK22", Routing: []string{f.From, f.To},
			Flights: []baggage.FlightLeg{{Flight: f.Carrier + strings.TrimLeft(f.Number, "0"), Date: wire}},
			Text:    "PAX ARRIVED WITHOUT BAG SHORT SHIPPED EX " + f.From,
		}
		t.openAHL[b.tag] = file
		t.tracing.AHL++
		t.tracing.Open++
		t.traced[b.ex] = fmt.Sprintf("%d missing on arrival at %s, AHL raised", len(short), f.To)
		t.groundMu.Unlock()
		send(file)
	}
	forwarded := map[dcs.Key]int{}
	for _, b := range arrived {
		t.groundMu.Lock()
		t.tracingSeq++
		ohd := &baggage.TracingFile{
			Kind: baggage.KindOHD, Reference: fmt.Sprintf("%s%s%05d", f.To, t.Carrier.Designator, t.tracingSeq),
			Station: f.To, Carrier: t.Carrier.Designator, Tags: []baggage.Tag{{Number: b.tag, Count: 1}}, Surname: b.surname,
			ColourType: "BK22", Routing: []string{f.From, f.To},
			Flights: []baggage.FlightLeg{{Flight: f.Carrier + strings.TrimLeft(f.Number, "0"), Date: wire}},
			Text:    "BAG WITHOUT PAX RUSH EX " + b.ex.Flight,
		}
		t.tracing.OHD++
		ahl := t.openAHL[b.tag]
		var fwd *baggage.TracingFile
		if ahl != nil && baggage.Match(ahl, ohd) {
			delete(t.openAHL, b.tag)
			t.tracing.FWD++
			t.tracing.Open--
			forwarded[b.ex]++
			fwd = &baggage.TracingFile{
				Kind: baggage.KindFWD, Reference: ahl.Reference, Station: f.To, Carrier: t.Carrier.Designator,
				Tags: ahl.Tags, Surname: ahl.Surname, ColourType: ahl.ColourType, Routing: ahl.Routing,
				Flights: ohd.Flights, ForwardTo: f.To, Matches: ohd.Reference, Text: "DELIVER TO PAX",
			}
		}
		t.groundMu.Unlock()
		send(ohd)
		if fwd != nil {
			send(fwd)
		}
	}
	t.groundMu.Lock()
	for ex, n := range forwarded {
		t.traced[ex] = fmt.Sprintf("%d missing on arrival at %s, AHL raised; %d matched off %s%s and forwarded", n, f.To, n, f.Carrier, strings.TrimLeft(f.Number, "0"))
	}
	t.groundMu.Unlock()
	if len(short) > 0 || len(arrived) > 0 {
		t.log.Info("bag office", "station", f.To, "flight", f.Carrier+f.Number, "ahl", len(short), "ohd", len(arrived), "fwd", len(forwarded))
	}
	return firstErr
}

func (t *Tenant) tracedFor(fl *dcs.Flight) string {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	return t.traced[dcs.Key{Flight: fl.Flight, Date: fl.Date, Board: fl.Board}]
}
