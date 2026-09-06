package sim

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adamf/wholesky/internal/airline"
)

// Where finished runs live: a core keeps them beside its state file, in
// recordings/<id>.json with a summary alongside for the index; a region
// has no disk and hands each finished run to the core over the private
// network. A world with no state file keeps them in memory alone.

// maxStoredRecordings bounds the runs kept on disk; the oldest go first.
const maxStoredRecordings = 200

// recordingsDir is the directory finished runs are kept in, or "".
func (s *Sim) recordingsDir() string {
	if s.state == nil || s.state.path == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.state.path), "recordings")
}

// storeRecording is handed every finished run by the registry.
func (s *Sim) storeRecording(rec *airline.Recording) {
	if dir := s.recordingsDir(); dir != "" {
		if err := writeRecording(dir, rec); err != nil {
			s.log.Warn("recording not kept", "id", rec.ID, "err", err)
		}
		return
	}
	s.externalMu.RLock()
	core := s.coreURL
	s.externalMu.RUnlock()
	if core == "" {
		return
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPut, strings.TrimRight(core, "/")+"/federation/recording/"+rec.ID, bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(secretHeader, s.linkSecret)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.log.Warn("recording not handed to the core", "id", rec.ID, "err", err)
		return
	}
	resp.Body.Close()
}

// writeRecording puts a run and its summary on disk, and keeps the
// directory to its bound.
func writeRecording(dir string, rec *airline.Recording) error {
	if !airline.ValidRecordingID(rec.ID) {
		return os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	sum := summaryOf(rec)
	sb, _ := json.Marshal(sum)
	for name, data := range map[string][]byte{rec.ID + ".json": b, rec.ID + ".summary.json": sb} {
		tmp := filepath.Join(dir, name+".tmp")
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			return err
		}
		if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	pruneRecordings(dir)
	return nil
}

// summaryOf is the index line for a run.
func summaryOf(rec *airline.Recording) airline.Summary {
	sum := airline.Summary{ID: rec.ID, Carrier: rec.Carrier, Holder: rec.Holder, Started: rec.Started, Ended: rec.Ended,
		StartPos: rec.StartPos, EndPos: rec.EndPos, Events: len(rec.Events)}
	if n := len(rec.Scores); n > 0 {
		sc := rec.Scores[n-1].Score
		sum.Final = &sc
	}
	return sum
}

// pruneRecordings drops the oldest runs past the bound.
func pruneRecordings(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type run struct {
		id  string
		mod time.Time
	}
	var runs []run
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".summary.json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		runs = append(runs, run{strings.TrimSuffix(name, ".summary.json"), info.ModTime()})
	}
	if len(runs) <= maxStoredRecordings {
		return
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].mod.Before(runs[j].mod) })
	for _, r := range runs[:len(runs)-maxStoredRecordings] {
		os.Remove(filepath.Join(dir, r.id+".json"))         //nolint:errcheck
		os.Remove(filepath.Join(dir, r.id+".summary.json")) //nolint:errcheck
	}
}

// Recording implements airline.RecordingStore from the disk.
func (s *Sim) Recording(id string) (*airline.Recording, bool) {
	dir := s.recordingsDir()
	if dir == "" || !airline.ValidRecordingID(id) {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".json"))
	if err != nil {
		return nil, false
	}
	var rec airline.Recording
	if err := json.Unmarshal(b, &rec); err != nil {
		return nil, false
	}
	return &rec, true
}

// Recordings implements airline.RecordingStore from the disk: the index
// lines, newest first.
func (s *Sim) Recordings() []airline.Summary {
	dir := s.recordingsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []airline.Summary
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".summary.json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var sum airline.Summary
		if json.Unmarshal(b, &sum) == nil && sum.ID != "" {
			out = append(out, sum)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out
}

// servePutRecording is PUT /federation/recording/{id} on a core: a region
// handing over a finished run. Behind the world's secret.
func (s *Sim) servePutRecording(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dir := s.recordingsDir()
	if dir == "" {
		http.Error(w, "this core keeps no recordings: no state file", http.StatusNotImplemented)
		return
	}
	if !airline.ValidRecordingID(id) {
		http.Error(w, "not a recording id", http.StatusBadRequest)
		return
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		http.Error(w, "a recording is a JSON document under 16 MB", http.StatusBadRequest)
		return
	}
	var rec airline.Recording
	if err := json.Unmarshal(b, &rec); err != nil || rec.ID != id {
		http.Error(w, "a recording is a JSON document naming its id", http.StatusBadRequest)
		return
	}
	if err := writeRecording(dir, &rec); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
