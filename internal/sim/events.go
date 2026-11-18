package sim

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/adamf/wholesky/internal/dayplan"
	"github.com/adamf/wholesky/internal/world"
)

// A world on several machines is one world: what a carrier does on the
// machine that runs it changes what every other carrier can do, wherever
// that one runs. A cancellation gives a slot back to a flight flown from
// another region; a fare filed at a carrier's desk is the price the
// distribution systems quote on theirs. These reach the other machines as
// events. The machine where the thing happened applies it and publishes
// it; a peer sends it to the core, the core applies it and sends it to
// every peer, and each machine applies an event once however many times
// it arrives.

// worldEvent is one act every machine's copy of the day must hear.
type worldEvent struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// A flight, by carrier, number and boarding point.
	Carrier string `json:"carrier,omitempty"`
	Number  string `json:"number,omitempty"`
	From    string `json:"from,omitempty"`
	// Pos is the day's clock when it happened, in minutes.
	Pos    int    `json:"pos,omitempty"`
	Reason string `json:"reason,omitempty"`
	Code   string `json:"code,omitempty"`
	// Body carries what the kind needs beyond a flight.
	Body json.RawMessage `json:"body,omitempty"`
}

const (
	// evCancel is a cancellation announced: the flight's slots go back.
	evCancel = "cancel"
	// evReady is a flight reporting ready for an earlier slot.
	evReady = "ready"
)

// eventLog remembers the events this machine has applied.
type eventLog struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// first reports whether an event is new here, and remembers it.
func (l *eventLog) first(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.seen == nil {
		l.seen = map[string]time.Time{}
	}
	if _, ok := l.seen[id]; ok {
		return false
	}
	if len(l.seen) > 20000 {
		cut := time.Now().Add(-10 * time.Minute)
		for k, at := range l.seen {
			if at.Before(cut) {
				delete(l.seen, k)
			}
		}
	}
	l.seen[id] = time.Now()
	return true
}

func newEventID() string {
	var b [8]byte
	rand.Read(b[:]) //nolint:errcheck
	return hex.EncodeToString(b[:])
}

// publish sends an event this machine has already applied to the rest of
// the world: from the core to every peer, from a peer to the core.
func (s *Sim) publish(ev worldEvent) {
	if ev.ID == "" {
		ev.ID = newEventID()
	}
	s.events.first(ev.ID)
	if s.onEvent != nil {
		go s.onEvent(ev)
		return
	}
	s.externalMu.RLock()
	core := s.coreURL
	s.externalMu.RUnlock()
	if core == "" {
		return
	}
	go func() {
		body, _ := json.Marshal(ev)
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := s.fedPost(client, strings.TrimRight(core, "/")+"/federation/event", body)
		if err != nil {
			s.log.Warn("the core did not take the event", "kind", ev.Kind, "err", err)
			return
		}
		resp.Body.Close()
	}()
}

// serveEvent is POST /federation/event on a core and POST /shard/event on
// a peer: an event from another machine of this world.
func (s *Sim) serveEvent(w http.ResponseWriter, r *http.Request) {
	var ev worldEvent
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&ev); err != nil || ev.ID == "" || ev.Kind == "" {
		http.Error(w, "malformed event", http.StatusBadRequest)
		return
	}
	if s.events.first(ev.ID) {
		s.applyEvent(r.Context(), ev)
		if s.onEvent != nil {
			go s.onEvent(ev)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// applyEvent does on this machine what another machine did on its own.
func (s *Sim) applyEvent(ctx context.Context, ev worldEvent) {
	switch ev.Kind {
	case evCancel, evReady:
		f, ok := seatWorld{s}.findFlight(strings.ToUpper(ev.Carrier), ev.Carrier+ev.Number, ev.From)
		if !ok || s.fate == nil {
			return
		}
		if ev.Kind == evReady {
			s.slotsImproved(ctx, s.fate.Ready(s.Manifest, f, ev.Pos), f.Carrier+strings.TrimLeft(f.Number, "0")+" reported ready", ev.Pos)
			return
		}
		s.fate.Update(f, func(pf *dayplan.Flight) {
			if !pf.Cancelled {
				pf.Cancelled, pf.Reason, pf.Code = true, ev.Reason, ev.Code
			}
		})
		s.slotsImproved(ctx, s.fate.Release(s.Manifest, f, ev.Pos), f.Carrier+strings.TrimLeft(f.Number, "0")+" was cancelled", ev.Pos)
	default:
		if h := s.eventHandlers[ev.Kind]; h != nil {
			h(ctx, ev)
		}
	}
}

// releaseSlots gives back the slots of a flight whose cancellation has
// just been announced, here and on every machine of the world, and tells
// the flights that move up.
func (s *Sim) releaseSlots(ctx context.Context, f world.Flight, reason string) {
	if s.fate == nil {
		return
	}
	s.releaseSlotsAt(ctx, f, reason, int(s.clock.Pos(time.Now())))
}

// releaseSlotsAt is releaseSlots at a given minute of the day.
func (s *Sim) releaseSlotsAt(ctx context.Context, f world.Flight, reason string, pos int) []dayplan.Change {
	fate := s.fate.Of(f)
	changes := s.fate.Release(s.Manifest, f, pos)
	s.slotsImproved(ctx, changes, f.Carrier+strings.TrimLeft(f.Number, "0")+" was cancelled", pos)
	s.publish(worldEvent{Kind: evCancel, Carrier: f.Carrier, Number: f.Number, From: f.From, Pos: pos, Reason: reason, Code: fate.Code})
	return changes
}

// slotsImproved tells each flight that took an earlier slot: the Network
// Manager's SRM with the new take-off time, or its SLC when no wait is
// left, to a flight that already holds its slot message; and a line on
// the tape of the carrier that flies it. Only the machine that runs the
// carrier says so, so a world on several machines says it once.
func (s *Sim) slotsImproved(ctx context.Context, changes []dayplan.Change, why string, pos int) {
	if len(changes) == 0 || s.fate == nil {
		return
	}
	for _, c := range changes {
		f := c.Flight
		if _, ours := s.Tenants[f.Carrier]; !ours {
			continue
		}
		now := s.fate.Of(f)
		s.SlotsImproved.Add(1)
		// The slot message goes out two hours before off-block; a flight
		// improved before that hears the better time in its first one.
		if s.ANSP != nil && f.DepMin-slotBefore <= pos {
			var err error
			if now.CTOT > 0 {
				err = s.ANSP.Revise(ctx, s.carriers[f.Carrier], f, s.BookingDate, now)
			} else {
				err = s.ANSP.Lift(ctx, s.carriers[f.Carrier], f, s.BookingDate)
			}
			if err != nil {
				s.log.Debug("slot revision not sent", "flight", f.Carrier+f.Number, "err", err)
			}
		}
		if s.Airline == nil {
			continue
		}
		name := f.Carrier + strings.TrimLeft(f.Number, "0") + " " + f.From + "-" + f.To
		text := fmt.Sprintf("%s: slot improved, CTOT %s (+%d, was +%d): %s", name, hhmm(now.CTOT), now.ATFM, c.Was.ATFM, why)
		if now.CTOT <= 0 {
			text = fmt.Sprintf("%s: slot lifted, was +%d: %s", name, c.Was.ATFM, why)
		}
		s.Airline.Emit(f.Carrier, "incident", text, map[string]any{"flight": f.Carrier + f.Number, "board": f.From, "slot_delay": now.ATFM, "was": c.Was.ATFM})
	}
}
