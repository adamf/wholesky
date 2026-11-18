package sim

// The operations centres in the running world. Each carrier's desk sees
// the day's signals -- weather forecast and in force over its stations,
// the Network Manager's regulations on its flights, its crews and
// reserves, what has already gone wrong -- and, three hours before a
// weather system reaches its stations, projects what the system will cost
// under each policy it could adopt and asks the seat that runs the desk.
// The default is the cheapest projection, so a desk on autopilot does
// what its own tools recommend. Anyone can add an event to the day --
// weather, a ground stop, a closure, a sick-out, an aircraft gone
// technical -- and every desk it touches sees it as it would a forecast.
// With -live-weather the day takes the real terminal forecasts of the
// world's busiest airports as they are issued.

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/airline"
	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/fuel"
	"github.com/adamf/wholesky/internal/noc"
	"github.com/adamf/wholesky/internal/tariff"
	"github.com/adamf/wholesky/internal/workforce"
	"github.com/adamf/wholesky/internal/world"
)

// nocLead is how far ahead of a weather system a desk decides.
const nocLead = 180

// nocEvent is one event added to the day.
type nocEvent struct {
	Kind    string `json:"kind"`
	Airport string `json:"airport,omitempty"`
	Carrier string `json:"carrier,omitempty"`
	// Start is minutes into the day; In, when Start is not given, is
	// minutes from now.
	Start    int     `json:"start,omitempty"`
	In       int     `json:"in,omitempty"`
	Duration int     `json:"duration,omitempty"`
	Factor   float64 `json:"factor,omitempty"`
	RadiusKM float64 `json:"radius_km,omitempty"`
	Rate     float64 `json:"rate,omitempty"`
	Flight   string  `json:"flight,omitempty"`
	Board    string  `json:"board,omitempty"`
	Source   string  `json:"source,omitempty"`
	Result   string  `json:"result,omitempty"`
}

// projection is a desk's last look ahead.
type projection struct {
	At       float64               `json:"at"`
	Clock    string                `json:"clock"`
	Scenario noc.Scenario          `json:"scenario"`
	Describe string                `json:"describe"`
	Results  []noc.Result          `json:"results"`
	Swings   []noc.Swing           `json:"swings,omitempty"`
	Stress   map[string]noc.Impact `json:"stress,omitempty"`
	Best     string                `json:"best"`
	Note     string                `json:"note,omitempty"`
}

type nocDesk struct {
	autopilot bool
	trials    int
	live      bool
	mu        sync.Mutex
	asked     map[string]bool
	last      map[string]*projection
	events    []nocEvent
	// sem bounds the projections running at once: a projection is
	// hundreds of plays of a carrier's day.
	sem     chan struct{}
	tafAt   time.Time
	tafErr  string
	tafSeen map[string]bool
}

func newNOCDesk(opts Options) *nocDesk {
	trials := opts.NOCTrials
	if trials <= 0 {
		trials = 120
	}
	return &nocDesk{autopilot: opts.NOCAutopilot, trials: trials, live: opts.LiveWeather,
		asked: map[string]bool{}, last: map[string]*projection{}, sem: make(chan struct{}, 2), tafSeen: map[string]bool{}}
}

// nocModel is a carrier's day ready for projection: the world's schedule,
// own delays and crews, what each leg sold, and the day's cancellations
// that are not the crews' (the plays work those out again). A recorded
// day is played as a synthetic one whose own delays are drawn from the
// record, because a record's delays would not move for any weather.
func (s *Sim) nocModel(code string) (*noc.Model, error) {
	m := s.Manifest
	md := &noc.Model{World: m, Carrier: code, Day: s.BookingDate, Own: delayFor, Base: map[string]string{}}
	if s.Workforce != nil && s.Workforce.Roster(code) != nil {
		md.Roster = s.Workforce
	}
	if m.Replay != nil {
		cp := *m
		cp.Replay = nil
		cp.Flights = make([]world.Flight, len(m.Flights))
		for i, f := range m.Flights {
			if f.Actual != nil {
				md.History = append(md.History, f.Actual.DepDelay)
			}
			f.Actual = nil
			cp.Flights[i] = f
		}
		md.World, md.Own = &cp, nil
	}
	w := seatWorld{s}
	booked := map[string]int{}
	revenue := map[string]int64{}
	for _, f := range s.Flights[code] {
		b, _ := w.booked(f)
		booked[dayplan.Key(f)] = b
		revenue[dayplan.Key(f)] = s.legRevenue(f)
		if fate := s.fate.Of(f); fate.Cancelled && !strings.HasPrefix(fate.Reason, "crew") && !strings.HasPrefix(fate.Reason, "cancelled as recorded") {
			md.Base[dayplan.Key(f)] = fate.Reason
		}
	}
	md.Booked = func(f world.Flight) int {
		if b, ok := booked[dayplan.Key(f)]; ok && b > 0 {
			return b
		}
		return f.Seats * 82 / 100
	}
	md.Revenue = func(f world.Flight) int64 { return revenue[dayplan.Key(f)] }
	// Where a leg has sold nothing yet, the average fare is taken as six
	// tenths of the full one: most seats sell below Y.
	md.Fare = func(f world.Flight) int64 { return tariff.AverageFare(s.tariff, f.Carrier, f.From, f.To) }
	return md, md.Prepare()
}

// centreOf is the airport at a cell's centre.
func (s *Sim) centreOf(c dayplan.Cell) string {
	best, bestKM := "", math.MaxFloat64
	for _, code := range c.Airports {
		a := s.airports[code]
		if d := kmBetween(c.Lat, c.Lon, a.Lat, a.Lon); d < bestKM {
			best, bestKM = code, d
		}
	}
	return best
}

func kmBetween(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371.0
	p := math.Pi / 180
	dLat, dLon := (lat2-lat1)*p, (lon2-lon1)*p
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1*p)*math.Cos(lat2*p)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// arrivalsUnder counts a carrier's arrivals into a cell's airports in its
// window.
func (s *Sim) arrivalsUnder(code string, c dayplan.Cell) int {
	under := map[string]bool{}
	for _, a := range c.Airports {
		under[a] = true
	}
	n := 0
	for _, f := range s.Flights[code] {
		if under[f.To] && f.ArrMin >= c.Start && f.ArrMin < c.End {
			n++
		}
	}
	return n
}

// forecast is the desk's picture of the day ahead as a scenario: every
// weather system over the carrier's stations that has not begun, with the
// uncertainty a forecast has -- onset to within a quarter of the lead
// time, duration to within three tenths, the rate to within a tenth --
// and what has begun or passed as fixed. The plan's cells are the truth;
// the desk sees them as a forecaster would.
func (s *Sim) forecast(code string, pos float64, md *noc.Model) noc.Scenario {
	cells, _, _ := s.fate.Picture()
	sc := noc.Scenario{Name: "forecast"}
	md.Fixed, md.FixedStops = nil, nil
	for _, c := range cells {
		if s.arrivalsUnder(code, c) == 0 {
			continue
		}
		if float64(c.Start) <= pos {
			md.Fixed = append(md.Fixed, c)
			continue
		}
		lead := float64(c.Start) - pos
		d := float64(c.End - c.Start)
		sc.Events = append(sc.Events, noc.Event{Kind: noc.KindWeather, Name: c.Name, Airport: s.centreOf(c), RadiusKM: c.RadiusKM,
			Onset: noc.Normal(float64(c.Start), 15+lead/4), Duration: noc.Tri(0.7*d, d, 1.3*d),
			Factor: noc.Tri(math.Max(0.05, c.Factor-0.1), c.Factor, math.Min(1, c.Factor+0.1))})
	}
	s.noc.mu.Lock()
	events := append([]nocEvent(nil), s.noc.events...)
	s.noc.mu.Unlock()
	for _, ev := range events {
		if ev.Kind != "groundstop" && ev.Kind != "closure" {
			continue
		}
		stop := dayplan.GroundStop{Airport: ev.Airport, Start: ev.Start, End: ev.Start + ev.Duration, Departures: ev.Kind == "closure"}
		if float64(ev.Start) <= pos {
			md.FixedStops = append(md.FixedStops, stop)
			continue
		}
		kind := noc.KindGroundStop
		if ev.Kind == "closure" {
			kind = noc.KindClosure
		}
		sc.Events = append(sc.Events, noc.Event{Kind: kind, Airport: ev.Airport, Onset: noc.Fixed(float64(ev.Start)),
			Duration: noc.Tri(0.7*float64(ev.Duration), float64(ev.Duration), 1.5*float64(ev.Duration))})
	}
	return sc
}

func seedOf(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write([]byte(p)) //nolint:errcheck
		h.Write([]byte{0}) //nolint:errcheck
	}
	return h.Sum64()
}

// project runs a projection under the desk's bound on concurrent work.
func (s *Sim) project(ctx context.Context, code string, md *noc.Model, sc noc.Scenario, pols []noc.Policy, o noc.Options, sensitivity bool) (*projection, error) {
	select {
	case s.noc.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.noc.sem }()
	pos, _ := seatWorld{s}.Clock()
	pr := &projection{At: pos, Clock: hhmm(int(pos)), Scenario: sc, Describe: sc.Describe()}
	if len(sc.Events) == 0 {
		pr.Note = "nothing forecast over the carrier's stations for the rest of the day"
	}
	rs, err := md.Compare(sc, pols, o)
	if err != nil {
		return nil, err
	}
	pr.Results = rs
	best := 0
	for i, r := range rs {
		if r.Cost.Mean < rs[best].Cost.Mean {
			best = i
		}
	}
	pr.Best = rs[best].Policy
	if sensitivity && len(sc.Events) > 0 {
		pr.Swings, _ = md.Sensitivity(sc, pols[0])
		pr.Stress = map[string]noc.Impact{}
		for name, q := range map[string]float64{"expected": 0.5, "bad": 0.9, "severe": 0.99} {
			im, _ := md.Stress(sc, pols[0], q)
			pr.Stress[name] = im
		}
	}
	s.noc.mu.Lock()
	s.noc.last[code] = pr
	s.noc.mu.Unlock()
	return pr, nil
}

// nocTick runs the desks' clock: once a weather system is three hours or
// less from a carrier's stations and has not begun, the desk projects and
// decides, once. A system added at short notice is decided on the next
// tick.
func (s *Sim) nocTick(ctx context.Context, cur int) {
	if s.Airline == nil || s.fate == nil {
		return
	}
	cells, _, _ := s.fate.Picture()
	for _, c := range cells {
		if c.Start <= cur || c.Start-nocLead > cur {
			continue
		}
		for code := range s.Tenants {
			if s.External(code) {
				continue
			}
			if !s.Airline.Manual(code, "noc") && !s.noc.autopilot {
				continue
			}
			if s.arrivalsUnder(code, c) < 3 {
				continue
			}
			key := fmt.Sprintf("%s/%s/%d", code, c.Name, c.Start)
			s.noc.mu.Lock()
			done := s.noc.asked[key]
			s.noc.asked[key] = true
			s.noc.mu.Unlock()
			if done {
				continue
			}
			go s.nocDecide(ctx, code, c)
		}
	}
}

// nocDecide projects the policies against the forecast and asks the desk.
func (s *Sim) nocDecide(ctx context.Context, code string, c dayplan.Cell) {
	md, err := s.nocModel(code)
	if err != nil {
		return
	}
	pos, _ := seatWorld{s}.Clock()
	sc := s.forecast(code, pos, md)
	// The decision is about this system: each policy acts on it alone, and
	// the day's other weather plays as forecast.
	pols := noc.Policies()
	for i := range pols {
		pols[i].Target = c.Name
	}
	pr, err := s.project(ctx, code, md, sc, pols, noc.Options{Trials: s.noc.trials, Seed: seedOf(code, c.Name, fmt.Sprint(c.Start)), Method: "lhs"}, false)
	if err != nil {
		return
	}
	n := s.arrivalsUnder(code, c)
	var lines []string
	opts := make([]airline.Option, 0, len(pr.Results))
	for _, r := range pr.Results {
		line := fmt.Sprintf("%s: expected cost %s, 1 day in 20 worse than %s", r.Policy, money(r.Cost.Mean), money(r.Cost.P95))
		if r.Cancels > 0 {
			line += fmt.Sprintf(", %d flights cancelled ahead", r.Cancels)
		}
		lines = append(lines, line)
		opts = append(opts, airline.Option{Key: r.Policy, Label: fmt.Sprintf("%s (%s)", r.Policy, money(r.Cost.Mean)), Cost: int64(r.Cost.Mean)})
	}
	if s.Airline != nil {
		s.Airline.Emit(code, "incident", fmt.Sprintf("forecast: weather %s over %s from %s to %s at ×%.2f; %d of your arrivals in it", c.Name, strings.Join(c.Airports, " "), hhmm(c.Start), hhmm(c.End), c.Factor, n), nil)
	}
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: code, Department: "noc",
		Title: fmt.Sprintf("Weather %s reaches %s at %s: %d of your arrivals", c.Name, s.centreOf(c), hhmm(c.Start), n),
		Detail: fmt.Sprintf("%d plays of the day (Latin hypercube) under the forecast's uncertainty. Costs are the disruption over a calm day: revenue lost, compensation and care owed, crews, ground staff, delay.\n%s\nThe desk recommends %s.",
			s.noc.trials, strings.Join(lines, "\n"), pr.Best),
		Options: opts, Default: pr.Best})
	pol, ok := noc.PolicyByName(choice)
	if !ok {
		return
	}
	pol.Target = c.Name
	res := s.applyPolicy(ctx, code, md, sc, pol)
	s.Airline.Emit(code, "action", "operations centre: "+res, nil)
}

// applyPolicy turns a policy into acts on the live day: the thinned
// flights cancelled now, with every message a cancellation sends, and the
// standby crews brought in, after which every tail with a crew
// cancellation still to come is staffed again.
func (s *Sim) applyPolicy(ctx context.Context, code string, md *noc.Model, sc noc.Scenario, pol noc.Policy) string {
	d := pol.Decide(md, sc)
	pos, _ := seatWorld{s}.Clock()
	t := s.Tenants[code]
	cancelled := 0
	for _, f := range s.Flights[code] {
		reason, ok := d.Cancel[dayplan.Key(f)]
		if !ok || t == nil {
			continue
		}
		fate := s.fate.Of(f)
		if fate.Cancelled || pos >= float64(f.DepMin+fate.DepDelay) {
			continue
		}
		s.fate.Update(f, func(pf *dayplan.Flight) { pf.Cancelled, pf.Reason, pf.Code = true, reason, "B" })
		s.announceCancellation(ctx, t, f, s.BookingDate, reason)
		s.replan(f)
		cancelled++
	}
	brought := 0
	if s.Workforce != nil {
		for base, n := range d.Standby {
			brought += s.Workforce.AddStandby(code, base, n)
		}
		if brought > 0 {
			s.restaffPending(code, pos)
		}
	}
	switch {
	case cancelled > 0:
		return fmt.Sprintf("%s: %d flights cancelled ahead of the weather", pol.Name, cancelled)
	case brought > 0:
		return fmt.Sprintf("%s: %d standby crews brought in", pol.Name, brought)
	}
	return pol.Name + ": the schedule stands"
}

// restaffPending works again every tail of the carrier with a crew
// cancellation not yet announced: a reserve that has come in may save it.
func (s *Sim) restaffPending(code string, pos float64) {
	seen := map[string]bool{}
	for _, f := range s.Flights[code] {
		fate := s.fate.Of(f)
		if !fate.Cancelled || !strings.HasPrefix(fate.Reason, "crew") || pos >= float64(f.DepMin-cancelledBefore) || seen[f.Tail] {
			continue
		}
		seen[f.Tail] = true
		s.replan(f)
	}
}

// inject adds an event to the day here and, on a core, at every peer.
func (s *Sim) inject(ctx context.Context, ev nocEvent) (string, error) {
	res, err := s.applyInject(ctx, ev)
	if err != nil {
		return "", err
	}
	if s.onInject != nil {
		go s.onInject(ev)
	}
	return res, nil
}

// applyInject adds an event to this machine's day.
func (s *Sim) applyInject(ctx context.Context, ev nocEvent) (string, error) {
	pos, _ := seatWorld{s}.Clock()
	ev.Airport = strings.ToUpper(ev.Airport)
	ev.Carrier = strings.ToUpper(ev.Carrier)
	if ev.Start == 0 {
		ev.Start = int(pos) + max(0, ev.In)
	}
	if ev.Duration <= 0 {
		ev.Duration = 180
	}
	if ev.Duration > 24*60 {
		return "", fmt.Errorf("an event lasts a day at most")
	}
	var changed []world.Flight
	switch ev.Kind {
	case "weather", "groundstop", "closure":
		a, ok := s.airports[ev.Airport]
		if !ok {
			return "", fmt.Errorf("no airport %q in this world", ev.Airport)
		}
		if ev.Kind == "weather" {
			if ev.Factor <= 0 || ev.Factor >= 1 {
				ev.Factor = 0.5
			}
			c := dayplan.Cell{Name: fmt.Sprintf("%sX%02d", ev.Airport, ev.Start/60), Lat: a.Lat, Lon: a.Lon, RadiusKM: math.Min(ev.RadiusKM, 600),
				Start: ev.Start, End: ev.Start + ev.Duration, Factor: ev.Factor}
			if ev.RadiusKM <= 0 {
				c.Airports = []string{ev.Airport}
			}
			changed = s.fate.Inject(s.Manifest, []dayplan.Cell{c}, nil, int(pos))
		} else {
			stop := dayplan.GroundStop{Airport: ev.Airport, Start: ev.Start, End: ev.Start + ev.Duration, Departures: ev.Kind == "closure"}
			changed = s.fate.Inject(s.Manifest, nil, []dayplan.GroundStop{stop}, int(pos))
		}
	case "sickout":
		if s.Workforce == nil || s.Workforce.Roster(ev.Carrier) == nil {
			if _, ok := s.carriers[ev.Carrier]; ok {
				return "the carrier's crews are on another machine", nil
			}
			return "", fmt.Errorf("no carrier %q", ev.Carrier)
		}
		if ev.Rate <= 0 || ev.Rate > 0.5 {
			ev.Rate = 0.12
		}
		s.Workforce.SetSickRate(ev.Carrier, ev.Rate, fmt.Sprint("inject", len(s.noc.events)))
		seen := map[string]bool{}
		for _, f := range s.Flights[ev.Carrier] {
			fate := s.fate.Of(f)
			if seen[f.Tail] || pos >= float64(f.DepMin+fate.DepDelay) {
				continue
			}
			seen[f.Tail] = true
			changed = append(changed, s.fate.Replan(s.Manifest, f.Carrier, f.Tail)...)
		}
	case "aog":
		code := ev.Carrier
		if code == "" && len(ev.Flight) >= 2 {
			code = strings.ToUpper(ev.Flight[:2])
		}
		f, ok := seatWorld{s}.findFlight(code, ev.Flight, ev.Board)
		if !ok {
			return "", fmt.Errorf("no departure %s from %s", ev.Flight, ev.Board)
		}
		s.fate.Update(f, func(pf *dayplan.Flight) {
			pf.Own += 150
			pf.DepDelay = max(pf.DepDelay, pf.Own)
			pf.ArrDelay = max(pf.ArrDelay, pf.Own)
		})
		changed = append([]world.Flight{f}, s.fate.Replan(s.Manifest, f.Carrier, f.Tail)...)
	case "fuel":
		// A shock to the price of fuel: Factor is the size of the step (0.4
		// is about half as much again), Duration its half-life in days.
		if s.econ == nil {
			return "", fmt.Errorf("this world keeps no economy")
		}
		if ev.Factor == 0 || ev.Factor > 1.5 || ev.Factor < -1.5 {
			ev.Factor = 0.4
		}
		half := float64(ev.Duration)
		if half <= 0 || half == 180 {
			half = 120
		}
		day := int(s.econ.day.Load())
		before := s.econ.fuel.Reference(day)
		s.econ.fuel.AddShock(fuel.Shock{Day: day, Step: ev.Factor, HalfLife: half, Name: ev.Source})
		s.econ.mu.Lock()
		s.econ.index = map[int]float64{}
		s.econ.mu.Unlock()
		s.saveState()
		res := fmt.Sprintf("fuel shock: spot from $%.2f to $%.2f a gallon, fading by half every %.0f days", before, s.econ.fuel.Reference(day), half)
		ev.Result = res
		s.noc.mu.Lock()
		s.noc.events = append(s.noc.events, ev)
		s.noc.mu.Unlock()
		if s.Airline != nil {
			for code := range s.Tenants {
				s.Airline.Emit(code, "incident", res, nil)
			}
		}
		return res, nil
	default:
		return "", fmt.Errorf("an event is weather, groundstop, closure, sickout, aog or fuel")
	}
	byCarrier := map[string][2]int{}
	for _, f := range changed {
		fate := s.fate.Of(f)
		n := byCarrier[f.Carrier]
		if fate.Cancelled {
			n[1]++
		} else {
			n[0]++
		}
		byCarrier[f.Carrier] = n
	}
	what := fmt.Sprintf("%s %s from %s for %dm", ev.Kind, ev.Airport+ev.Carrier+ev.Flight, hhmm(ev.Start), ev.Duration)
	if s.Airline != nil {
		for code, n := range byCarrier {
			if _, ok := s.Tenants[code]; ok {
				s.Airline.Emit(code, "incident", fmt.Sprintf("%s: %d of your flights now later, %d cancelled", what, n[0], n[1]), nil)
			}
		}
	}
	ev.Result = fmt.Sprintf("%s: %d flights changed across %d carriers", what, len(changed), len(byCarrier))
	s.noc.mu.Lock()
	s.noc.events = append(s.noc.events, ev)
	if len(s.noc.events) > 200 {
		s.noc.events = s.noc.events[len(s.noc.events)-200:]
	}
	s.noc.mu.Unlock()
	return ev.Result, nil
}

// serveInject is POST /noc/inject: anyone may add an event to the day, as
// anyone may close an airport, one act per visitor per half-minute; the
// world's own operator, presenting its secret, is not limited.
func (s *Sim) serveInject(w http.ResponseWriter, r *http.Request) {
	if !s.secretOK(r) && s.chaosLimiter != nil && !s.chaosLimiter.allow(clientIP(r)) {
		http.Error(w, "one event per visitor per half-minute", http.StatusTooManyRequests)
		return
	}
	var ev nocEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&ev); err != nil {
		http.Error(w, "malformed event", http.StatusBadRequest)
		return
	}
	ev.Source = "visitor"
	if s.secretOK(r) {
		ev.Source = "operator"
	}
	res, err := s.inject(r.Context(), ev)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:errcheck
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"ok": res}) //nolint:errcheck
}

// serveShardInject is POST /shard/noc/inject: the core handing a peer an
// event added at the core.
func (s *Sim) serveShardInject(w http.ResponseWriter, r *http.Request) {
	var ev nocEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&ev); err != nil {
		http.Error(w, "malformed event", http.StatusBadRequest)
		return
	}
	if _, err := s.applyInject(r.Context(), ev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// liveWeather reads the real forecasts of the world's busiest airports
// every half hour and adds the weather they forecast, likelier than not,
// to the day as it is issued.
func (s *Sim) liveWeather(ctx context.Context) {
	arrivals := map[string]int{}
	for _, f := range s.Manifest.Flights {
		arrivals[f.To]++
	}
	type ap struct {
		iata, icao string
		n          int
	}
	var list []ap
	for code, n := range arrivals {
		if a := s.airports[code]; a.ICAO != "" {
			list = append(list, ap{code, a.ICAO, n})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].n > list[j].n })
	if len(list) > 40 {
		list = list[:40]
	}
	icao := make([]string, len(list))
	byICAO := map[string]string{}
	for i, a := range list {
		icao[i] = a.icao
		byICAO[a.icao] = a.iata
	}
	read := func() {
		tafs, err := noc.FetchTAFs(ctx, nil, icao)
		s.noc.mu.Lock()
		s.noc.tafAt = time.Now()
		s.noc.tafErr = ""
		if err != nil {
			s.noc.tafErr = err.Error()
		}
		s.noc.mu.Unlock()
		if err != nil {
			s.log.Warn("live weather unavailable", "err", err)
			return
		}
		sc := noc.FromTAFs(tafs, time.Now().UTC(), func(c string) string { return byICAO[c] })
		pos, _ := seatWorld{s}.Clock()
		for _, e := range sc.Events {
			if e.Probability > 0 && e.Probability < 0.5 {
				continue
			}
			start := int(e.Onset.Quantile(0.5))
			key := fmt.Sprintf("%s/%d", e.Name, start)
			s.noc.mu.Lock()
			seen := s.noc.tafSeen[key]
			s.noc.tafSeen[key] = true
			s.noc.mu.Unlock()
			if seen || float64(start) <= pos || start >= 24*60 {
				continue
			}
			ev := nocEvent{Kind: "weather", Airport: e.Airport, Start: start, Duration: int(e.Duration.Quantile(0.5)), Factor: e.Factor.Quantile(0.5), Source: "taf " + e.Name}
			if res, err := s.inject(ctx, ev); err == nil {
				s.log.Info("live weather added", "event", res)
			}
		}
	}
	read()
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			read()
		}
	}
}

// labourTick is the unions' morning: at 06:00 every open proposal is
// answered, by the seat that runs labour relations or by the autopilot,
// which counters.
func (s *Sim) labourTick(ctx context.Context, prev, cur int) {
	if s.Workforce == nil || !(prev < 6*60 && cur >= 6*60) {
		return
	}
	for code := range s.Tenants {
		if s.External(code) {
			continue
		}
		for _, u := range []workforce.Unit{workforce.Pilots, workforce.FlightAttendants, workforce.Ground} {
			c, ok := s.Workforce.Contract(code, u)
			if !ok || c.Proposal == 0 {
				continue
			}
			if s.Airline == nil || !s.Airline.Manual(code, "labour") {
				if res, err := s.Workforce.Respond(code, u, "counter"); err == nil && s.Airline != nil {
					s.Airline.Emit(code, "action", "labour relations: "+res, nil)
				}
				continue
			}
			go s.askLabour(ctx, code, c)
		}
	}
}

func (s *Sim) askLabour(ctx context.Context, code string, c *workforce.Contract) {
	u, ask, union := c.Unit, c.Proposal, c.Union
	choice := s.Airline.Ask(ctx, airline.Decision{Carrier: code, Department: "labour",
		Title: fmt.Sprintf("The %s asks %.1f%% for the %s", union, ask*100, u),
		Detail: fmt.Sprintf("The agreement is %s, amendable since %s. Accept: the scale rises %.1f%% at once and the agreement is in force for four years. Counter at half: the union takes it about half the time and the talks go on. Refuse: the talks break down, the union allows no day-off assignments, and sick calls more than double from tomorrow.",
			c.Status, c.Amendable.Format("2 Jan 2006"), ask*100),
		Options: []airline.Option{{Key: "counter", Label: fmt.Sprintf("counter at %.1f%%", ask*50)}, {Key: "accept", Label: fmt.Sprintf("accept %.1f%%", ask*100)}, {Key: "refuse", Label: "refuse"}},
		Default: "counter"})
	if res, err := s.Workforce.Respond(code, u, choice); err == nil {
		s.Airline.Emit(code, "action", "labour relations: "+res, nil)
	}
}

// money is cents as dollars for a person.
func money(cents float64) string {
	d := cents / 100
	sign := ""
	if d < 0 {
		sign, d = "-", -d
	}
	switch {
	case d >= 1e6:
		return fmt.Sprintf("%s$%.2fM", sign, d/1e6)
	case d >= 1e4:
		return fmt.Sprintf("%s$%.0fk", sign, d/1e3)
	}
	return fmt.Sprintf("%s$%.0f", sign, d)
}

// nocView is a desk's picture for the page and the agents.
type nocView struct {
	Carrier     string              `json:"carrier"`
	Hub         string              `json:"hub"`
	Pos         float64             `json:"pos"`
	Clock       string              `json:"clock"`
	Manual      bool                `json:"manual"`
	Weather     []weatherSignal     `json:"weather"`
	Regulations []regSignal         `json:"regulations"`
	Day         daySignal           `json:"day"`
	Crew        *workforce.Overview `json:"crew,omitempty"`
	Events      []nocEvent          `json:"events"`
	Projection  *projection         `json:"projection,omitempty"`
	LiveWeather string              `json:"live_weather,omitempty"`
}

type weatherSignal struct {
	Name     string   `json:"name"`
	Airports []string `json:"airports"`
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Factor   float64  `json:"factor"`
	State    string   `json:"state"`
	Arrivals int      `json:"arrivals"`
}

type regSignal struct {
	Name     string `json:"name"`
	Airport  string `json:"airport"`
	Window   string `json:"window"`
	Rate     string `json:"rate"`
	Cause    string `json:"cause"`
	Flights  int    `json:"flights"`
	DelayMin int    `json:"delay_min"`
}

type daySignal struct {
	Flights   int `json:"flights"`
	Flown     int `json:"flown"`
	Cancelled int `json:"cancelled"`
	Late      int `json:"late"`
	Slotted   int `json:"slotted"`
	Reserves  int `json:"reserves"`
	CrewCxl   int `json:"crew_cancelled"`
	Extended  int `json:"extended"`
}

// NOC implements airline.Operations.
func (w seatWorld) NOC(code string) (any, bool) {
	s := w.s
	c, ok := s.carriers[code]
	if !ok || s.fate == nil {
		return nil, false
	}
	if _, runs := s.Tenants[code]; !runs {
		return nil, false
	}
	pos, _ := w.Clock()
	v := nocView{Carrier: code, Hub: c.Hub, Pos: pos, Clock: hhmm(int(pos)), Manual: s.Airline != nil && s.Airline.Manual(code, "noc")}
	cells, regs, _ := s.fate.Picture()
	for _, cell := range cells {
		n := s.arrivalsUnder(code, cell)
		if n == 0 {
			continue
		}
		state := "forecast"
		switch {
		case pos >= float64(cell.End):
			state = "passed"
		case pos >= float64(cell.Start):
			state = "in force"
		}
		v.Weather = append(v.Weather, weatherSignal{Name: cell.Name, Airports: cell.Airports, Start: hhmm(cell.Start), End: hhmm(cell.End), Factor: cell.Factor, State: state, Arrivals: n})
	}
	mine := map[string][2]int{}
	for _, f := range s.Flights[code] {
		fate := s.fate.Of(f)
		v.Day.Flights++
		status := w.flightStatus(f, fate, pos)
		switch status {
		case "cancelled":
			v.Day.Cancelled++
			if strings.HasPrefix(fate.Reason, "crew") {
				v.Day.CrewCxl++
			}
		case "departed", "landed":
			v.Day.Flown++
		}
		if fate.DepDelay > 15 && !fate.Cancelled {
			v.Day.Late++
		}
		if fate.Reserve {
			v.Day.Reserves++
		}
		if fate.Extended {
			v.Day.Extended++
		}
		if fate.Regulation != "" {
			v.Day.Slotted++
			n := mine[fate.Regulation]
			n[0]++
			n[1] += fate.ATFM
			mine[fate.Regulation] = n
		}
	}
	for _, r := range regs {
		n, ok := mine[r.Name]
		if !ok {
			continue
		}
		v.Regulations = append(v.Regulations, regSignal{Name: r.Name, Airport: r.Airport, Window: hhmm(r.Start) + "-" + hhmm(r.End),
			Rate: fmt.Sprintf("%d of %d an hour", r.Rate, r.NormalRate), Cause: r.CauseText, Flights: n[0], DelayMin: n[1]})
		delete(mine, r.Name)
	}
	if s.Workforce != nil {
		if o, ok := s.Workforce.Overview(code, pos); ok {
			v.Crew = &o
		}
	}
	s.noc.mu.Lock()
	v.Events = append([]nocEvent(nil), s.noc.events...)
	v.Projection = s.noc.last[code]
	if s.noc.live {
		v.LiveWeather = "real terminal forecasts from aviationweather.gov"
		if !s.noc.tafAt.IsZero() {
			v.LiveWeather += ", last read " + s.noc.tafAt.UTC().Format("15:04Z")
		}
		if s.noc.tafErr != "" {
			v.LiveWeather += " (" + s.noc.tafErr + ")"
		}
	}
	s.noc.mu.Unlock()
	return v, true
}

// Crew implements airline.Operations.
func (w seatWorld) Crew(code string) (any, bool) {
	if w.s.Workforce == nil {
		return nil, false
	}
	pos, _ := w.Clock()
	return w.s.Workforce.Overview(code, pos)
}

// CrewMember implements airline.Operations.
func (w seatWorld) CrewMember(code, id string) (any, bool) {
	if w.s.Workforce == nil {
		return nil, false
	}
	return w.s.Workforce.Member(code, id)
}

// projectRequest is what a seat or an agent asks a desk to project.
type projectRequest struct {
	Trials      int           `json:"trials"`
	Method      string        `json:"method"`
	Policies    []string      `json:"policies"`
	Sensitivity bool          `json:"sensitivity"`
	Scenario    *noc.Scenario `json:"scenario"`
	// TAF names stations whose real forecasts make the scenario.
	TAF []string `json:"taf"`
	// Events are added to the forecast, in the scenario's form.
	Events []noc.Event `json:"events"`
	Seed   uint64      `json:"seed"`
}

// Project implements airline.Operations.
func (w seatWorld) Project(ctx context.Context, code string, raw json.RawMessage) (any, error) {
	s := w.s
	if _, runs := s.Tenants[code]; !runs {
		return nil, fmt.Errorf("this machine does not run %s", code)
	}
	var req projectRequest
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req); err != nil {
			return nil, fmt.Errorf("malformed projection request: %w", err)
		}
	}
	md, err := s.nocModel(code)
	if err != nil {
		return nil, err
	}
	pos, _ := w.Clock()
	sc := s.forecast(code, pos, md)
	switch {
	case req.Scenario != nil:
		sc = *req.Scenario
	case len(req.TAF) > 0:
		if len(req.TAF) > 20 {
			return nil, fmt.Errorf("twenty stations at most")
		}
		byICAO := map[string]string{}
		for _, a := range s.Manifest.Airports {
			if a.ICAO != "" {
				byICAO[a.ICAO] = a.IATA
			}
		}
		tafs, err := noc.FetchTAFs(ctx, nil, req.TAF)
		if err != nil {
			return nil, err
		}
		sc = noc.FromTAFs(tafs, time.Now().UTC(), func(c string) string { return byICAO[strings.ToUpper(c)] })
	}
	sc.Events = append(sc.Events, req.Events...)
	if len(sc.Events) > 40 {
		return nil, fmt.Errorf("forty events at most")
	}
	pols := noc.Policies()
	if len(req.Policies) > 0 {
		pols = nil
		for _, name := range req.Policies {
			if p, ok := noc.PolicyByName(name); ok {
				pols = append(pols, p)
			}
		}
		if len(pols) == 0 {
			return nil, fmt.Errorf("no such policy; choose from absorb, thin 15%%, thin 30%%, standby crews")
		}
	}
	o := noc.Options{Trials: req.Trials, Method: req.Method, Seed: req.Seed}
	if o.Trials <= 0 {
		o.Trials = s.noc.trials
	}
	o.Trials = min(o.Trials, 2000)
	if o.Seed == 0 {
		o.Seed = seedOf(code, fmt.Sprint(int(pos)))
	}
	return s.project(ctx, code, md, sc, pols, o, req.Sensitivity)
}

var _ airline.Operations = seatWorld{}

// newDay turns the world over to a new day of the same schedule: every
// crew's record takes the day flown, the day is worked out again from the
// schedule with crews staffed afresh, the cancellations are the new day's
// to announce, and the desks decide again.
func (s *Sim) newDay() {
	if s.econ != nil {
		s.econ.day.Add(1)
		s.saveState()
	}
	if s.Workforce != nil && s.fate != nil {
		s.Workforce.NextDay()
		s.fate.Rebuild(s.Manifest, delayFor)
		_, _, sum := s.fate.Picture()
		s.log.Info("new day staffed", "duties", sum.Duties, "reserves", sum.Reserves, "crew cancellations", sum.TimedOut)
	}
	s.announced.Clear()
	s.noc.mu.Lock()
	s.noc.asked = map[string]bool{}
	s.noc.events = nil
	s.noc.mu.Unlock()
}
