package airline

import (
	"context"
	"testing"
	"time"
)

// A seat is taken once, its token is what changes it, and a department on
// autopilot answers a decision with the default at once; taken manual, the
// decision waits for the seat and is what the seat says.
func TestSeatsDepartmentsAndDecisions(t *testing.T) {
	r := New(200 * time.Millisecond)
	seat, token, err := r.Take("ba", "Adam")
	if err != nil || seat.Carrier != "BA" || token == "" {
		t.Fatalf("take: %+v %q %v", seat, token, err)
	}
	if _, _, err := r.Take("BA", "someone else"); err != ErrHeld {
		t.Errorf("a held seat is held: %v", err)
	}
	if err := r.SetManual("BA", "wrong", "ops", true); err != ErrNotHeld {
		t.Errorf("the wrong token cannot change the seat: %v", err)
	}
	ask := Decision{Carrier: "BA", Department: "ops", Flight: "BA0117", Title: "BA117 is 52 minutes late",
		Options: []Option{{Key: "announce", Label: "announce the delay (ASM TIM)"}, {Key: "hold", Label: "hold the announcement"}}}
	// On autopilot: the default, immediately.
	start := time.Now()
	if got := r.Ask(context.Background(), ask); got != "announce" || time.Since(start) > 50*time.Millisecond {
		t.Errorf("autopilot answered %q after %s", got, time.Since(start))
	}
	if err := r.SetManual("BA", token, "ops", true); err != nil {
		t.Fatal(err)
	}
	// Manual, unanswered: the default at the deadline.
	start = time.Now()
	if got := r.Ask(context.Background(), ask); got != "announce" || time.Since(start) < 150*time.Millisecond {
		t.Errorf("deadline answered %q after %s", got, time.Since(start))
	}
	// Manual, answered: what the seat said.
	got := make(chan string, 1)
	go func() { got <- r.Ask(context.Background(), ask) }()
	var open []Decision
	for i := 0; i < 50 && len(open) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
		open = r.Inbox("BA")
	}
	if len(open) != 1 || open[0].Title != ask.Title || open[0].Deadline.Before(open[0].Opened) {
		t.Fatalf("inbox %+v", open)
	}
	if err := r.Answer("BA", token, open[0].ID, "nonsense"); err == nil {
		t.Error("an answer must be one of the options")
	}
	if err := r.Answer("BA", token, open[0].ID, "hold"); err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "hold" {
		t.Errorf("the seat said hold, the simulation heard %q", v)
	}
	s, _ := r.Seat("BA")
	if s.Answered != 1 || s.Defaulted != 1 || len(r.Inbox("BA")) != 0 {
		t.Errorf("seat %+v inbox %d", s, len(r.Inbox("BA")))
	}
	tape := r.Tape("BA")
	if len(tape) < 4 || tape[0].Kind != "seat" {
		t.Errorf("tape %+v", tape)
	}
	// Releasing closes open decisions with their defaults.
	go func() { got <- r.Ask(context.Background(), ask) }()
	for i := 0; i < 50 && len(r.Inbox("BA")) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	if err := r.Release("BA", token); err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "announce" {
		t.Errorf("release defaulted to %q", v)
	}
	if _, ok := r.Seat("BA"); ok || r.Manual("BA", "ops") {
		t.Error("the seat is free and the autopilot is back")
	}
}

// Subscribers hear the tape as it is written.
func TestSubscribersHearEvents(t *testing.T) {
	r := New(time.Second)
	ch, cancel := r.Subscribe("")
	defer cancel()
	r.Emit("AF", "incident", "weather over CDG", nil)
	select {
	case e := <-ch:
		if e.Carrier != "AF" || e.Kind != "incident" {
			t.Errorf("event %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
}
