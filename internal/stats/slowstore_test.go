package stats

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A slow queue-depth read must not hold the collector's lock: the recorded
// day's stats page took eight seconds because the store read ran under it
// every tenth second, and every event behind it waited too.
func TestSlowQueueReadDoesNotBlockThePage(t *testing.T) {
	c := New()
	c.QueueDepths = func() map[string]int {
		time.Sleep(600 * time.Millisecond)
		return map[string]int{"general": 1}
	}
	stop := make(chan struct{})
	defer close(stop)
	go c.Run(stop)
	// The depth read happens on every fifth two-second tick; catch the page while
	// it is in flight.
	time.Sleep(10100 * time.Millisecond)
	start := time.Now()
	rec := httptest.NewRecorder()
	c.data(rec, httptest.NewRequest(http.MethodGet, "/stats/data.json", nil))
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Fatalf("the page waited on the store read: %v", d)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}
