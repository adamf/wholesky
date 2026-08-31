package stats

import (
	"path/filepath"
	"testing"
)

// The rings survive a restart: what one collector persisted, the next
// restores, so a redeploy does not open on blank charts.
func TestSnapshotSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	a := New()
	a.SetSnapshotPath(path)
	for i := 0; i < 5; i++ {
		a.OnMessage(map[string]any{"kind": "MVT/BA0117", "format": "typeb"})
		a.OnBooking()
	}
	a.mu.Lock()
	a.sTotal.push(2.5)
	a.sBookings.push(1.5)
	a.mu.Unlock()
	a.persist()

	b := New()
	b.SetSnapshotPath(path)
	b.mu.Lock()
	defer b.mu.Unlock()
	if got := b.sTotal.slice(); len(got) != 1 || got[0] != 2.5 {
		t.Errorf("restored total series = %v, want [2.5]", got)
	}
	if got := b.sBookings.slice(); len(got) != 1 || got[0] != 1.5 {
		t.Errorf("restored bookings series = %v, want [1.5]", got)
	}
	if b.tTotal != 5 || b.tBookings != 5 {
		t.Errorf("restored totals = %d/%d, want 5/5", b.tTotal, b.tBookings)
	}
}
