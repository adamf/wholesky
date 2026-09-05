// Package airline lets someone run one of the world's carriers. Every
// carrier flies on autopilot: the tenant answers sells, announces its
// delays, substitutes aircraft, rushes bags, and the day plan decides what
// its crews and slots do to it. A seat is a person or an agent taking a
// carrier over. Department by department they can switch the autopilot
// off; where it is off the simulation stops deciding and asks -- a
// decision with options, a default and a deadline -- and what comes back
// is what happens. Unanswered decisions fall to the default, so a slow
// player degrades into the autopilot, never into chaos. Levers are the
// actions a seat can pull at any time: cancel or retime a flight, swap the
// aircraft, close a booking class, move the fares, ask the Network Manager
// for a better slot, call reserves. The scorecard says how the day went in
// money and punctuality, against every other carrier's autopilot, which is
// the bar to beat.
//
// The seams are HTTP and JSON (a page for people, the same API for
// agents; cmd/skyagent wraps it as MCP tools) so the same carrier can be
// run by a human at the console, a model, or a script -- or handed between
// them mid-day.
package airline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Department is one part of the airline the autopilot runs until told not to.
type Department struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	About string `json:"about"`
}

// Departments a seat can take off autopilot.
var Departments = []Department{
	{"ops", "Operations control", "delays and time changes announced to distribution, cancellations, aircraft substitutions"},
	{"crew", "Crew control", "what happens when a crew times out: reserves, or the cancellation"},
	{"slots", "Flow management", "the Network Manager's slots: take them, or ask for an improvement"},
	{"pricing", "Pricing and revenue management", "the fare multiplier over the filing and the booking classes open on each flight"},
	{"ground", "Ground and baggage", "short-shipped bags: rush them on the next flight, or hold them for tomorrow"},
}

func departmentKnown(key string) bool {
	for _, d := range Departments {
		if d.Key == key {
			return true
		}
	}
	return false
}

// Seat is who runs a carrier and which departments they have taken.
type Seat struct {
	Carrier string          `json:"carrier"`
	Holder  string          `json:"holder"`
	Since   time.Time       `json:"since"`
	Manual  map[string]bool `json:"manual"`
	// Answered and Defaulted count the seat's decisions.
	Answered  int    `json:"answered"`
	Defaulted int    `json:"defaulted"`
	token     string // returned once, on Take
}

// Option is one way a decision can go; Cost is what the world thinks it
// costs, in cents, for the player's benefit.
type Option struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Cost  int64  `json:"cost,omitempty"`
}

// Decision is the simulation asking the seat something.
type Decision struct {
	ID         string    `json:"id"`
	Carrier    string    `json:"carrier"`
	Department string    `json:"department"`
	Flight     string    `json:"flight,omitempty"`
	Board      string    `json:"board,omitempty"`
	Title      string    `json:"title"`
	Detail     string    `json:"detail"`
	Options    []Option  `json:"options"`
	Default    string    `json:"default"`
	Opened     time.Time `json:"opened"`
	Deadline   time.Time `json:"deadline"`
	// Chosen is set once the decision is closed, and By says who: the seat
	// or the deadline.
	Chosen string `json:"chosen,omitempty"`
	By     string `json:"by,omitempty"`
	chosen chan string
}

// Event is one line of a carrier's tape: a decision opened or closed, an
// action taken, an incident the day threw.
type Event struct {
	At      time.Time `json:"at"`
	Carrier string    `json:"carrier"`
	Kind    string    `json:"kind"`
	Text    string    `json:"text"`
	Data    any       `json:"data,omitempty"`
}

// Registry holds the seats, their inboxes and their tapes.
type Registry struct {
	// Window is how long a decision waits for the seat before the default,
	// in real time.
	Window time.Duration
	Now    func() time.Time

	mu    sync.Mutex
	seats map[string]*Seat
	inbox map[string]map[string]*Decision
	tape  map[string][]Event
	subs  map[string]map[chan Event]struct{}
	seq   int
}

// New makes an empty registry; window is the decision deadline.
func New(window time.Duration) *Registry {
	if window <= 0 {
		window = 45 * time.Second
	}
	return &Registry{Window: window, Now: time.Now, seats: map[string]*Seat{}, inbox: map[string]map[string]*Decision{},
		tape: map[string][]Event{}, subs: map[string]map[chan Event]struct{}{}}
}

// ErrHeld is a seat someone already has.
var ErrHeld = fmt.Errorf("airline: seat is held")

// ErrNotHeld is a seat nobody has, or a token that is not its holder's.
var ErrNotHeld = fmt.Errorf("airline: seat is not held by this token")

// Take gives the carrier to the holder. The token comes back once; every
// change to the seat needs it.
func (r *Registry) Take(carrier, holder string) (Seat, string, error) {
	carrier = strings.ToUpper(strings.TrimSpace(carrier))
	holder = strings.TrimSpace(holder)
	if carrier == "" || holder == "" {
		return Seat{}, "", fmt.Errorf("airline: a seat needs a carrier and a holder")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, held := r.seats[carrier]; held {
		return Seat{}, "", ErrHeld
	}
	b := make([]byte, 12)
	rand.Read(b) //nolint:errcheck
	s := &Seat{Carrier: carrier, Holder: holder, Since: r.Now(), Manual: map[string]bool{}, token: hex.EncodeToString(b)}
	r.seats[carrier] = s
	r.emit(Event{At: r.Now(), Carrier: carrier, Kind: "seat", Text: holder + " took the seat"})
	return *s, s.token, nil
}

// Release hands the carrier back to the autopilot; open decisions fall to
// their defaults.
func (r *Registry) Release(carrier, token string) error {
	carrier = strings.ToUpper(carrier)
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[carrier]
	if !ok || s.token != token {
		return ErrNotHeld
	}
	delete(r.seats, carrier)
	for id, d := range r.inbox[carrier] {
		d.Chosen, d.By = d.Default, "release"
		select {
		case d.chosen <- d.Default:
		default:
		}
		delete(r.inbox[carrier], id)
	}
	r.emit(Event{At: r.Now(), Carrier: carrier, Kind: "seat", Text: s.Holder + " released the seat; autopilot resumes"})
	return nil
}

// Seat is the seat as others may see it: no token.
func (r *Registry) Seat(carrier string) (Seat, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[strings.ToUpper(carrier)]
	if !ok {
		return Seat{}, false
	}
	return *s, true
}

// Authorised says the token holds the seat.
func (r *Registry) Authorised(carrier, token string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[strings.ToUpper(carrier)]
	return ok && token != "" && s.token == token
}

// SetManual takes a department off autopilot (manual true) or gives it back.
func (r *Registry) SetManual(carrier, token, dept string, manual bool) error {
	if !departmentKnown(dept) {
		return fmt.Errorf("airline: no department %q", dept)
	}
	carrier = strings.ToUpper(carrier)
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[carrier]
	if !ok || s.token != token {
		return ErrNotHeld
	}
	s.Manual[dept] = manual
	if !manual {
		delete(s.Manual, dept)
	}
	mode := "autopilot"
	if manual {
		mode = "manual"
	}
	r.emit(Event{At: r.Now(), Carrier: carrier, Kind: "department", Text: dept + " is now " + mode})
	return nil
}

// Manual says whether a department of a carrier is run by its seat.
func (r *Registry) Manual(carrier, dept string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[strings.ToUpper(carrier)]
	return ok && s.Manual[dept]
}

// Ask puts a decision to the seat and waits for the answer, the deadline
// or the context; on autopilot it returns the default at once. What comes
// back is always one of the options' keys.
func (r *Registry) Ask(ctx context.Context, d Decision) string {
	d.Carrier = strings.ToUpper(d.Carrier)
	if d.Default == "" && len(d.Options) > 0 {
		d.Default = d.Options[0].Key
	}
	if !r.Manual(d.Carrier, d.Department) {
		return d.Default
	}
	r.mu.Lock()
	r.seq++
	d.ID = fmt.Sprintf("d%06d", r.seq)
	d.Opened = r.Now()
	d.Deadline = d.Opened.Add(r.Window)
	d.chosen = make(chan string, 1)
	if r.inbox[d.Carrier] == nil {
		r.inbox[d.Carrier] = map[string]*Decision{}
	}
	r.inbox[d.Carrier][d.ID] = &d
	r.emit(Event{At: d.Opened, Carrier: d.Carrier, Kind: "decision", Text: d.Title, Data: d})
	r.mu.Unlock()

	timer := time.NewTimer(r.Window)
	defer timer.Stop()
	var chosen, by string
	select {
	case chosen = <-d.chosen:
		by = "seat"
	case <-timer.C:
		chosen, by = d.Default, "deadline"
	case <-ctx.Done():
		chosen, by = d.Default, "cancelled"
	}
	r.mu.Lock()
	if cur, ok := r.inbox[d.Carrier][d.ID]; ok {
		if cur.Chosen != "" {
			chosen, by = cur.Chosen, cur.By
		}
		delete(r.inbox[d.Carrier], d.ID)
	}
	if s, ok := r.seats[d.Carrier]; ok {
		if by == "seat" {
			s.Answered++
		} else {
			s.Defaulted++
		}
	}
	r.emit(Event{At: r.Now(), Carrier: d.Carrier, Kind: "decided", Text: d.Title + " → " + label(d.Options, chosen) + " (" + by + ")",
		Data: map[string]string{"id": d.ID, "chosen": chosen, "by": by}})
	r.mu.Unlock()
	return chosen
}

func label(opts []Option, key string) string {
	for _, o := range opts {
		if o.Key == key {
			return o.Label
		}
	}
	return key
}

// Answer closes a decision with one of its options.
func (r *Registry) Answer(carrier, token, id, key string) error {
	carrier = strings.ToUpper(carrier)
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[carrier]
	if !ok || s.token != token {
		return ErrNotHeld
	}
	d, ok := r.inbox[carrier][id]
	if !ok {
		return fmt.Errorf("airline: no open decision %s", id)
	}
	valid := false
	for _, o := range d.Options {
		if o.Key == key {
			valid = true
		}
	}
	if !valid {
		return fmt.Errorf("airline: %q is not one of the options", key)
	}
	d.Chosen, d.By = key, "seat"
	select {
	case d.chosen <- key:
	default:
	}
	return nil
}

// Inbox is a carrier's open decisions, oldest first.
func (r *Registry) Inbox(carrier string) []Decision {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Decision
	for _, d := range r.inbox[strings.ToUpper(carrier)] {
		out = append(out, *d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Emit writes a line on a carrier's tape and to its subscribers.
func (r *Registry) Emit(carrier, kind, text string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.emit(Event{At: r.Now(), Carrier: strings.ToUpper(carrier), Kind: kind, Text: text, Data: data})
}

// emit is Emit under the lock.
func (r *Registry) emit(e Event) {
	tape := append(r.tape[e.Carrier], e)
	if len(tape) > 200 {
		tape = tape[len(tape)-200:]
	}
	r.tape[e.Carrier] = tape
	for _, key := range []string{e.Carrier, ""} {
		for ch := range r.subs[key] {
			select {
			case ch <- e:
			default:
			}
		}
	}
}

// Tape is a carrier's recent events, oldest first.
func (r *Registry) Tape(carrier string) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.tape[strings.ToUpper(carrier)]...)
}

// Subscribe follows a carrier's events ("" for every carrier's) until the
// cancel is called.
func (r *Registry) Subscribe(carrier string) (<-chan Event, func()) {
	carrier = strings.ToUpper(carrier)
	ch := make(chan Event, 64)
	r.mu.Lock()
	if r.subs[carrier] == nil {
		r.subs[carrier] = map[chan Event]struct{}{}
	}
	r.subs[carrier][ch] = struct{}{}
	r.mu.Unlock()
	return ch, func() {
		r.mu.Lock()
		delete(r.subs[carrier], ch)
		r.mu.Unlock()
	}
}

// Seats is every held seat, without tokens, by carrier.
func (r *Registry) Seats() []Seat {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Seat
	for _, s := range r.seats {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Carrier < out[j].Carrier })
	return out
}
