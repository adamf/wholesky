package host

// The operations side of a carrier: the desk that files the flight plan,
// hears from the aircraft, and tells the network what moved.
//
// Until now the tenant asserted its own movements: the flight day said an
// aircraft had departed, and the tenant wrote the MVT. That is not how an
// operations centre learns of a departure. The aircraft reports OUT and OFF
// over its datalink; the provider forwards the report to the airline; the
// airline's system derives the movement message from it. And before any of
// that, operations files a flight plan with air traffic services, who send
// their own departure and arrival messages back when they see the aircraft
// move. This file is that desk.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/adamf/jetway/pkg/acars"
	"github.com/adamf/jetway/pkg/aftn"
	"github.com/adamf/jetway/pkg/ats"
	"github.com/adamf/jetway/pkg/typeb"

	"github.com/adamf/wholesky/internal/world"
)

// DeptOps is the operations desk's office function on a station address:
// where the datalink provider delivers the aircraft's reports.
const DeptOps = "OO"

// icaoTypes maps the world's coarse equipment labels to ICAO aircraft type
// designators and wake turbulence categories for the flight plan.
var icaoTypes = map[string]struct{ code, wake string }{
	"AT7": {"AT76", "M"}, "320": {"A320", "M"}, "321": {"A321", "M"},
	"789": {"B789", "H"}, "77W": {"B77W", "H"}, "E75": {"E75L", "M"}, "73H": {"B738", "M"},
}

// Callsign is the flight as air traffic services know it: the carrier's
// ICAO designator and the flight number without leading zeros.
func Callsign(c world.Carrier, f world.Flight) string {
	return c.ICAO + strings.TrimLeft(f.Number, "0")
}

// icaoOf is the location indicator for an airport, or "" when unknown.
func (t *Tenant) icaoOf(iata string) string {
	if t.icao == nil {
		return ""
	}
	return t.icao(iata)
}

// AFTNAddress is this carrier's operations indicator at an aerodrome: the
// location indicator, the carrier's designator, and X for the office.
func (t *Tenant) AFTNAddress(iata string) string {
	loc := t.icaoOf(iata)
	if loc == "" {
		return ""
	}
	return loc + t.Carrier.ICAO + "X"
}

// FileFlightPlan files the ICAO flight plan for a departure with air traffic
// services at both ends, over the AFTN. The route is synthetic -- a cruise
// speed and level and DCT -- because the world has no airways; everything
// else on the plan is the flight's own.
func (t *Tenant) FileFlightPlan(ctx context.Context, f world.Flight, day time.Time) error {
	if t.isCancelled(f, day) {
		return nil
	}
	dep, dest := t.icaoOf(f.From), t.icaoOf(f.To)
	if dep == "" || dest == "" {
		return fmt.Errorf("host: no ICAO indicator for %s or %s", f.From, f.To)
	}
	typ, ok := icaoTypes[f.Equipment]
	if !ok {
		typ = icaoTypes["320"]
	}
	level := "F350"
	if f.KM < 800 {
		level = "F250"
	}
	speed := "N0450"
	if f.Equipment == "AT7" {
		speed, level = "N0270", "F200"
	}
	reg := strings.ReplaceAll(registrationOf(f), "-", "")
	m := &ats.Message{
		Type: ats.TypeFPL, AircraftID: Callsign(t.Carrier, f), Rules: "I", FlightType: "S",
		AircraftType: typ.code, Wake: typ.wake, Equipment: "SDE2E3FGHIRWXY/LB1",
		Departure: dep, EOBT: hhmm(f.DepMin), Route: speed + level + " DCT",
		Destination: dest, EET: hhmm(f.BlockMin),
		Other: []ats.Item{{Key: "DOF", Value: day.Format("060102")}, {Key: "REG", Value: reg}},
	}
	text, err := ats.Build(m)
	if err != nil {
		return err
	}
	return t.sendAFTN(ctx, aftn.PrioritySafety, []string{dep + "ZPZX", dest + "ZPZX"}, t.AFTNAddress(f.From), text, "ATS/FPL/"+m.AircraftID)
}

// sendAFTN puts an ATS message in an AFTN envelope and sends it down the
// tenant's one circuit; the switch carries it by indicator.
func (t *Tenant) sendAFTN(ctx context.Context, prio aftn.Priority, addressees []string, originator, text, kind string) error {
	if originator == "" {
		return fmt.Errorf("host: no AFTN originator")
	}
	now := time.Now().UTC()
	env := &aftn.Message{
		TransmissionID: fmt.Sprintf("%s%03d", t.Carrier.ICAO, now.Unix()%1000),
		Priority:       prio, Addressees: addressees,
		FilingTime: now.Format("021504"), Originator: originator, Text: text,
	}
	raw, err := env.Encode(aftn.EncodeOptions{CRLF: true})
	if err != nil {
		return err
	}
	peer := t.Gateway.Peer("net")
	_, err = t.Gateway.Send(ctx, peer, raw, kind, "", "")
	return err
}

// hhmm renders a minute of the day.
func hhmm(min int) string {
	min = ((min % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d%02d", min/60, min%60)
}

// Datalink implements gateway.Ground: the aircraft has reported, via the
// provider. An OFF is a departure and an IN is an arrival; the movement
// message the network runs on is derived from the report, with the delay
// against schedule the report implies.
func (t *Tenant) Datalink(ctx context.Context, m *acars.Message, origin typeb.Address) error {
	f, ok := t.flightsByNum[normaliseFlight(m.Flight)]
	if !ok {
		return fmt.Errorf("host: datalink report for unknown flight %s", m.Flight)
	}
	day := t.opsDay()
	reg := m.Registration
	if reg == "" {
		reg = registrationOf(f)
	}
	// The scheduled time comes from the same day base the report's absolute
	// time did, so an unaligned booking date -- a synthetic day that does
	// not start at midnight -- does not read as hours of delay.
	sched := func(min int) string { return day.Add(time.Duration(min) * time.Minute).Format("1504") }
	switch {
	case m.Kind == acars.KindDEP && m.Off != "":
		out := m.Out
		if out == "" {
			out = m.Off
		}
		delay := minutesBetween(sched(f.DepMin), out)
		return t.Depart(ctx, f, day, reg, delay)
	case m.Kind == acars.KindARR && m.In != "":
		delay := minutesBetween(sched(f.ArrMin), m.In)
		return t.Arrive(ctx, f, day, reg, delay)
	}
	// An OUT without an OFF, or an ON without an IN, is half a movement;
	// operations waits for the rest.
	return nil
}

// normaliseFlight turns a callsign-style flight (BA117, JA401) into the
// world's carrier plus four-digit number.
func normaliseFlight(flight string) string {
	if len(flight) < 3 {
		return flight
	}
	num := strings.TrimLeft(flight[2:], "0")
	for len(num) < 4 {
		num = "0" + num
	}
	return flight[:2] + num
}

// minutesBetween is the signed minutes from a scheduled HHMM to an actual
// one, across midnight the short way.
func minutesBetween(scheduled, actual string) int {
	toMin := func(s string) int {
		if len(s) != 4 {
			return 0
		}
		var h, m int
		fmt.Sscanf(s, "%02d%02d", &h, &m)
		return h*60 + m
	}
	d := toMin(actual) - toMin(scheduled)
	if d > 720 {
		d -= 1440
	}
	if d < -720 {
		d += 1440
	}
	return d
}

// ATS implements gateway.Ground: air traffic services writing to the
// airline -- the departure and arrival messages about its flights, which an
// operations centre files against the flight and reconciles with what the
// aircraft said.
func (t *Tenant) ATS(ctx context.Context, m *ats.Message, env *aftn.Message) error {
	t.groundMu.Lock()
	t.atsSeen[m.Type]++
	t.groundMu.Unlock()
	return nil
}

// ATSMessages reports how many of each ATS message type reached this
// carrier's operations desk.
func (t *Tenant) ATSMessages() map[ats.Type]int {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	out := map[ats.Type]int{}
	for k, v := range t.atsSeen {
		out[k] = v
	}
	return out
}

// opsDay is the day operations is flying: set by the flight day, so a
// report can be matched to the right departure.
func (t *Tenant) opsDay() time.Time {
	t.groundMu.Lock()
	defer t.groundMu.Unlock()
	if t.day.IsZero() {
		return time.Now().UTC().Truncate(24 * time.Hour)
	}
	return t.day
}

// SetDay tells operations which day the world is flying.
func (t *Tenant) SetDay(day time.Time) {
	t.groundMu.Lock()
	t.day = day
	t.groundMu.Unlock()
}
