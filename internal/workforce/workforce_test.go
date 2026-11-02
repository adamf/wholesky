package workforce

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

var day = time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)

// testWorld is one carrier based at ORD flying twelve tails, each out and
// back twice a day: ORD-BOS-ORD in the morning, ORD-LAX-ORD after.
func testWorld() *world.Manifest {
	m := &world.Manifest{
		Airports: []world.Airport{{IATA: "ORD"}, {IATA: "BOS"}, {IATA: "LAX"}},
		Carriers: []world.Carrier{{Designator: "UA", Hub: "ORD", Country: "United States"}},
	}
	n := 100
	for t := 0; t < 12; t++ {
		tail := fmt.Sprintf("N%03dUA", t)
		start := 6*60 + t*10
		legs := [][3]any{{"ORD", "BOS", 120}, {"BOS", "ORD", 135}, {"ORD", "LAX", 270}, {"LAX", "ORD", 240}}
		dep := start
		for _, l := range legs {
			block := l[2].(int)
			m.Flights = append(m.Flights, world.Flight{Carrier: "UA", Number: fmt.Sprintf("%04d", n), From: l[0].(string), To: l[1].(string),
				DepMin: dep, ArrMin: dep + block, BlockMin: block, Seats: 180, Equipment: "320", Tail: tail})
			n++
			dep += block + 45
		}
	}
	return m
}

func noDelay(world.Flight, time.Time) (int, int) { return 0, 0 }

func TestEveryDutyIsStaffedByName(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	p := dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	if p.Summary.Cancelled != 0 {
		t.Fatalf("a calm day with a full roster cancels nothing: %+v", p.Summary)
	}
	r := w.Roster("UA")
	// The summary counts a reserve's takeover as a duty too.
	if r == nil || len(r.duties) != p.Summary.Duties-p.Summary.Reserves {
		t.Fatalf("duties staffed %d of %d", len(r.duties), p.Summary.Duties-p.Summary.Reserves)
	}
	for id, ds := range r.duties {
		groups := map[Group]int{}
		for _, i := range ds.crew {
			groups[r.Members[i].Group]++
			if r.Members[i].duty != id {
				t.Errorf("%s flies %s but is booked on %q", r.Members[i].ID, id, r.Members[i].duty)
			}
		}
		// 180 seats: two plus one for each 50 over 100, rounded up: 4.
		// One who called in sick at report leaves the cabin one short.
		if groups[Captain] != 1 || groups[FirstOfficer] != 1 || groups[FlightAttendant] != 4-ds.short || len(ds.crew) != 6-ds.short {
			t.Errorf("%s crew %v", id, groups)
		}
	}
	// No member flies two duties.
	seen := map[string]string{}
	for id, ds := range r.duties {
		for _, i := range ds.crew {
			if other, ok := seen[r.Members[i].ID]; ok {
				t.Errorf("%s on %s and %s", r.Members[i].ID, other, id)
			}
			seen[r.Members[i].ID] = id
		}
	}
}

// The same world staffs the same way on every machine.
func TestStaffingIsDeterministic(t *testing.T) {
	m := testWorld()
	crewOf := func() string {
		w := New(m, day, nil)
		dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
		r := w.Roster("UA")
		var b strings.Builder
		for _, id := range sortedKeys(r.duties) {
			fmt.Fprint(&b, id, ":")
			for _, i := range r.duties[id].crew {
				fmt.Fprint(&b, r.Members[i].ID, ",")
			}
		}
		return b.String()
	}
	if a, b := crewOf(), crewOf(); a != b {
		t.Error("two builds staffed differently")
	}
}

func sortedKeys(m map[string]*dutyState) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// A captain near 100 hours in 672 cannot be given a long duty: Admits
// sees the record, and the roster picks someone else.
func TestCumulativeLimitKeepsAPilotOff(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	r := w.Roster("UA")
	// Fill every line captain's last 28 days to 97 hours: no captain on
	// the line can fly a duty with more than three hours of block.
	for i := range r.Members {
		mb := &r.Members[i]
		if mb.Group != Captain || mb.Reserve {
			continue
		}
		mb.hist = history{}
		for d := 2; d < 22; d++ {
			mb.hist.days[d] = dayRec{report: 360, length: 600, flight: 291, kind: kindFlight}
		}
	}
	p := dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	called, junior := 0, 0
	for _, ds := range r.duties {
		for _, i := range ds.crew {
			mb := r.Members[i]
			if mb.Group != Captain {
				continue
			}
			if mb.Reserve {
				called++
			}
			if mb.junior {
				junior++
			}
			if !mb.Reserve && !mb.junior {
				t.Errorf("line captain %s flies with 97 hours", mb.ID)
			}
		}
	}
	if called == 0 {
		t.Errorf("no reserve covered the open time: summary %+v", p.Summary)
	}
	if called+junior+p.Summary.TimedOut < p.Summary.Duties-1 {
		t.Errorf("captains: %d reserves, %d day-off, %d uncovered of %d duties", called, junior, p.Summary.TimedOut, p.Summary.Duties)
	}
}

// A long delay pushes a crew over Table B. Eight hours late out of BOS,
// the morning crew (report 05:00) would release at 19:15, over the 14
// hours its table and extension allow; BOS is no crew base, so no reserve
// answers there and the leg is cancelled for crew. Six hours late, the
// crew is legal home on the extension and times out on the next leg,
// out of ORD, where reserves wait: that leg flies ninety minutes later.
func TestReservesAnswerOnlyAtABase(t *testing.T) {
	m := testWorld()
	lateBy := func(min int) dayplan.Own {
		return func(f world.Flight, _ time.Time) (int, int) {
			if f.From == "BOS" {
				return min, min
			}
			return 0, 0
		}
	}
	w := New(m, day, nil)
	p := dayplan.BuildWith(m, day, lateBy(480), dayplan.Options{Calm: true, Staff: w})
	away := 0
	for _, f := range m.Flights {
		fate := p.Of(f)
		if f.From == "BOS" && fate.Cancelled && strings.HasPrefix(fate.Reason, "crew duty limit") {
			away++
		}
		if f.From == "BOS" && fate.Reserve {
			t.Errorf("%s%s has a reserve at BOS", f.Carrier, f.Number)
		}
	}
	if away == 0 {
		t.Fatalf("no crew cancellation at BOS: %+v", p.Summary)
	}
	w = New(m, day, nil)
	p = dayplan.BuildWith(m, day, lateBy(360), dayplan.Options{Calm: true, Staff: w})
	r := w.Roster("UA")
	atBase := 0
	for _, f := range m.Flights {
		fate := p.Of(f)
		if fate.Reserve && f.From == "ORD" && fate.DepDelay >= dayplan.ReserveCall {
			atBase++
		}
		if fate.Reserve && !r.IsBase(f.From) {
			t.Errorf("%s%s from %s has a reserve away from every base", f.Carrier, f.Number, f.From)
		}
	}
	if atBase == 0 {
		t.Fatalf("no reserve answered at ORD: %+v", p.Summary)
	}
	calledOut := 0
	for _, ds := range r.duties {
		calledOut += len(ds.called)
	}
	if calledOut < atBase {
		t.Errorf("the plan called %d reserves, the roster named %d", atBase, calledOut)
	}
}

// With no reserves left at the base a timed-out crew's leg is cancelled
// even there.
func TestNoReservesLeftCancels(t *testing.T) {
	m := testWorld()
	late := func(f world.Flight, _ time.Time) (int, int) {
		if f.From == "BOS" {
			return 360, 360
		}
		return 0, 0
	}
	w := New(m, day, nil)
	r := w.Roster("UA")
	for i := range r.Members {
		if r.Members[i].Reserve {
			r.Members[i].sick, r.Members[i].short = true, false
		}
	}
	p := dayplan.BuildWith(m, day, late, dayplan.Options{Calm: true, Staff: w})
	if p.Summary.Reserves != 0 {
		t.Errorf("reserves answered with every reserve sick: %+v", p.Summary)
	}
	if p.Summary.TimedOut == 0 || p.Summary.Cancelled == 0 {
		t.Errorf("a timed-out crew with no reserves flies: %+v", p.Summary)
	}
}

// A crew member who calls in sick at report needs a reserve: the flight
// waits for them.
func TestShortNoticeSickCallWaitsForAReserve(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	r := w.Roster("UA")
	for i := range r.Members {
		r.Members[i].sick, r.Members[i].short = false, false
	}
	// The first-officer pool's first line member calls in at report.
	var target string
	for i := range r.Members {
		mb := &r.Members[i]
		if mb.Group == FirstOfficer && !mb.Reserve && mb.line.On(0) {
			mb.sick, mb.short = true, true
			target = mb.ID
			break
		}
	}
	p := dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	idx := r.byID[target]
	if r.Members[idx].duty == "" {
		t.Skip("the sick first officer was never rostered")
	}
	if p.Summary.Reserves != 1 {
		t.Fatalf("one sick call, %d reserves: %+v", p.Summary.Reserves, p.Summary)
	}
	ds := r.duties[r.Members[idx].duty]
	if len(ds.called) != 1 || r.Members[ds.called[0]].Group != FirstOfficer {
		t.Errorf("the reserve called was not a first officer")
	}
	for _, f := range m.Flights {
		if fate := p.Of(f); fate.Reserve && fate.DepDelay != dayplan.ReserveCall {
			t.Errorf("the reserve's leg left %d late", fate.DepDelay)
		}
	}
}

// The day rolls into the record: tomorrow, today's duty is yesterday's.
func TestNextDayRollsTheRecord(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	r := w.Roster("UA")
	var flying int32 = -1
	for i := range r.Members {
		if r.Members[i].duty != "" && r.Members[i].Group == Captain {
			flying = int32(i)
			break
		}
	}
	ds := r.duties[r.Members[flying].duty]
	block, length := dutySpan(ds.actual)
	before := r.Members[flying].hist.days[0]
	w.NextDay()
	got := r.Members[flying].hist.days[0]
	if got.kind != kindFlight || time.Duration(got.flight)*time.Minute != block || time.Duration(got.length)*time.Minute != length {
		t.Errorf("yesterday %+v, want block %s length %s", got, block, length)
	}
	if r.Members[flying].hist.days[1] != before {
		t.Error("the record did not move a day")
	}
	if r.Members[flying].duty != "" || len(r.duties) != 0 {
		t.Error("the new day starts unstaffed")
	}
}

// Crews cost what their agreements say: a calm day's pay, nothing at a
// premium, and a cancelled duty still paid.
func TestCrewCostsUnderTheAgreements(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	calm, _ := w.Costs("UA", 1440)
	if calm.Pay <= 0 || calm.Premium != 0 || calm.Protected != 0 {
		t.Fatalf("calm day %+v", calm)
	}
	// A captain on a 10-hour duty earns at least five; at about 300
	// dollars an hour the day's pilot pay is thousands per duty.
	if per := calm.Pay / int64(calm.Duties); per < 200000 {
		t.Errorf("crew pay per duty %d cents", per)
	}
	// Cancel every BOS-ORD leg before the day: pay protection keeps the
	// crews paid.
	cancel := map[string]string{}
	for _, f := range m.Flights {
		if f.From == "BOS" {
			cancel[dayplan.Key(f)] = "thinned"
		}
	}
	w2 := New(m, day, nil)
	dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w2, Cancel: cancel})
	thinned, _ := w2.Costs("UA", 1440)
	if thinned.Pay < calm.Pay*8/10 {
		t.Errorf("pay protection: %d against %d", thinned.Pay, calm.Pay)
	}
	if g := w.GroundCost("UA", 180, 75, false, 0); g <= 0 {
		t.Errorf("an hour's delay costs the ramp nothing")
	}
}

// The ring answers the History questions the way the record says.
func TestHistoryView(t *testing.T) {
	var h history
	h.days[0] = dayRec{report: 6 * 60, length: 600, flight: 300, kind: kindFlight}
	h.days[2] = dayRec{report: 5 * 60, length: 840, kind: kindReserve}
	h.year = 600 * 60
	v := view{h: &h, day: day}
	if got := v.FlightTime(day.Add(-48*time.Hour), day); got != 5*time.Hour {
		t.Errorf("flight time %s", got)
	}
	if got := v.FlightTime(day.AddDate(-1, 0, 0), day); got != 605*time.Hour {
		t.Errorf("flight time in a year %s", got)
	}
	if got := v.DutyTime(day.Add(-72*time.Hour), day); got != 10*time.Hour {
		t.Errorf("reserve is not FDP: %s", got)
	}
	_, rel, ok := v.LastDuty(day)
	if !ok || !rel.Equal(day.Add(-24*time.Hour).Add(16*time.Hour)) {
		t.Errorf("last release %s", rel)
	}
}

// A flight attendant who calls in sick at report does not stop the
// flight while another is aboard: it goes one short, seats blocked, on
// time, and no reserve is called.
func TestCabinFliesShortRatherThanWait(t *testing.T) {
	m := testWorld()
	w := New(m, day, nil)
	r := w.Roster("UA")
	for i := range r.Members {
		r.Members[i].sick, r.Members[i].short = false, false
	}
	var target int32 = -1
	for i := range r.Members {
		mb := &r.Members[i]
		if mb.Group == FlightAttendant && !mb.Reserve && mb.line.On(0) {
			mb.sick, mb.short = true, true
			target = int32(i)
			break
		}
	}
	p := dayplan.BuildWith(m, day, noDelay, dayplan.Options{Calm: true, Staff: w})
	if !r.Members[target].relieve {
		t.Skip("the sick flight attendant was never rostered")
	}
	if p.Summary.Reserves != 0 || p.Summary.Cancelled != 0 {
		t.Errorf("a short cabin waited or cancelled: %+v", p.Summary)
	}
	short := 0
	for _, ds := range r.duties {
		short += ds.short
	}
	if short != 1 {
		t.Errorf("%d duties flew short", short)
	}
}
