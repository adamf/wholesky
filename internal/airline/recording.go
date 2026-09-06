package airline

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// The flight recorder.
//
// A seat's run is worth keeping: what the day asked, what the seat
// answered and when, what it did on its own, what it said it was thinking,
// and what the scorecard did about it. A Recording starts when a seat is
// taken and ends when it is released; every line the tape gets in between
// is appended, with the sim clock stamped on it, and the scorecard is
// sampled as the day runs. The replay page plays it back.

// Recording is one seat's run, take to release.
type Recording struct {
	ID       string    `json:"id"`
	Carrier  string    `json:"carrier"`
	Holder   string    `json:"holder"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitempty"`
	StartPos float64   `json:"start_pos"`
	EndPos   float64   `json:"end_pos,omitempty"`
	Live     bool      `json:"live,omitempty"`
	// Answered and Defaulted are the seat's counts at the end.
	Answered  int           `json:"answered"`
	Defaulted int           `json:"defaulted"`
	Events    []Event       `json:"events"`
	Scores    []ScoreSample `json:"scores"`
}

// ScoreSample is the scorecard at one moment of the run.
type ScoreSample struct {
	At    time.Time `json:"at"`
	Pos   float64   `json:"pos"`
	Score Scorecard `json:"score"`
}

// Summary is a recording as the index lists it.
type Summary struct {
	ID       string    `json:"id"`
	Carrier  string    `json:"carrier"`
	Holder   string    `json:"holder"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitempty"`
	StartPos float64   `json:"start_pos"`
	EndPos   float64   `json:"end_pos,omitempty"`
	Events   int       `json:"events"`
	Live     bool      `json:"live,omitempty"`
	// Final is the last scorecard sampled.
	Final *Scorecard `json:"final,omitempty"`
}

// Bounds: a run's tape and its samples, and how many finished runs a
// machine remembers before they are only on disk.
const (
	maxRecordingEvents = 20000
	maxRecordingScores = 5000
	maxDoneRecordings  = 32
	maxNoteChars       = 2000
)

// recordingIDRe is the shape of an id the pages and handlers accept.
var recordingIDRe = regexp.MustCompile(`^[a-f0-9]{8,16}$`)

// ValidRecordingID says whether an id could be one of ours.
func ValidRecordingID(id string) bool { return recordingIDRe.MatchString(id) }

// summary is the recording as the index lists it.
func (rec *Recording) summary() Summary {
	s := Summary{ID: rec.ID, Carrier: rec.Carrier, Holder: rec.Holder, Started: rec.Started, Ended: rec.Ended,
		StartPos: rec.StartPos, EndPos: rec.EndPos, Events: len(rec.Events), Live: rec.Live}
	if n := len(rec.Scores); n > 0 {
		sc := rec.Scores[n-1].Score
		s.Final = &sc
	}
	return s
}

// clone copies a recording for serving, so the reader never shares the
// slices the seat is still appending to.
func (rec *Recording) clone() *Recording {
	out := *rec
	out.Events = append([]Event(nil), rec.Events...)
	out.Scores = append([]ScoreSample(nil), rec.Scores...)
	return &out
}

// pos is the sim clock, when the world gave the registry one.
func (r *Registry) pos() float64 {
	if r.Pos == nil {
		return 0
	}
	return r.Pos()
}

// startRecordingLocked opens a seat's recording.
func (r *Registry) startRecordingLocked(s *Seat) {
	b := make([]byte, 6)
	rand.Read(b) //nolint:errcheck
	rec := &Recording{ID: hex.EncodeToString(b), Carrier: s.Carrier, Holder: s.Holder, Started: r.Now(), StartPos: r.pos(), Live: true}
	if r.recs == nil {
		r.recs = map[string]*Recording{}
	}
	r.recs[s.Carrier] = rec
	s.Recording = rec.ID
}

// finishRecordingLocked closes a seat's recording and keeps it.
func (r *Registry) finishRecordingLocked(s *Seat) *Recording {
	rec := r.recs[s.Carrier]
	if rec == nil {
		return nil
	}
	delete(r.recs, s.Carrier)
	rec.Ended, rec.EndPos, rec.Live = r.Now(), r.pos(), false
	rec.Answered, rec.Defaulted = s.Answered, s.Defaulted
	r.done = append(r.done, rec)
	if len(r.done) > maxDoneRecordings {
		r.done = r.done[len(r.done)-maxDoneRecordings:]
	}
	return rec
}

// recordLocked appends a tape line to the carrier's open recording.
func (r *Registry) recordLocked(e Event) {
	rec := r.recs[e.Carrier]
	if rec == nil {
		return
	}
	if len(rec.Events) < maxRecordingEvents {
		rec.Events = append(rec.Events, e)
	}
}

// Sample notes the scorecard of a held seat at this moment.
func (r *Registry) Sample(carrier string, score Scorecard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.recs[strings.ToUpper(carrier)]
	if rec == nil || len(rec.Scores) >= maxRecordingScores {
		return
	}
	rec.Scores = append(rec.Scores, ScoreSample{At: r.Now(), Pos: r.pos(), Score: score})
}

// Note puts the seat's own words on the tape: what it is thinking, for
// the record. It needs the seat's token.
func (r *Registry) Note(carrier, token, text string) error {
	carrier = strings.ToUpper(carrier)
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("airline: a note says something")
	}
	if len(text) > maxNoteChars {
		text = text[:maxNoteChars]
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.seats[carrier]
	if !ok || !tokenMatch(s.token, token) {
		return ErrNotHeld
	}
	r.emit(Event{At: r.Now(), Carrier: carrier, Kind: "agent", Text: text})
	return nil
}

// LiveRecording is a held seat's run so far.
func (r *Registry) LiveRecording(carrier string) (*Recording, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := r.recs[strings.ToUpper(carrier)]
	if rec == nil {
		return nil, false
	}
	return rec.clone(), true
}

// Recording finds a run this machine holds, live or finished.
func (r *Registry) Recording(id string) (*Recording, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rec := range r.recs {
		if rec.ID == id {
			return rec.clone(), true
		}
	}
	for _, rec := range r.done {
		if rec.ID == id {
			return rec.clone(), true
		}
	}
	return nil, false
}

// Recordings lists the runs this machine holds, newest first.
func (r *Registry) Recordings() []Summary {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Summary
	for _, rec := range r.recs {
		out = append(out, rec.summary())
	}
	for _, rec := range r.done {
		out = append(out, rec.summary())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}
