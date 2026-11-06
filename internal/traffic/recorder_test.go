package traffic

import (
	"fmt"
	"testing"
)

func TestRecorderKeepsTheNewestRequestsAndCountsDrops(t *testing.T) {
	firstDrops := 0
	recorder := NewLimitedRecorder(3, nil, func() { firstDrops++ })
	for i := 1; i <= 5; i++ {
		recorder.Add(Request{Path: fmt.Sprintf("/%d", i)})
	}

	entries := recorder.Snapshot()
	if len(entries) != 3 || entries[0].Path != "/3" || entries[2].Path != "/5" {
		t.Fatalf("kept %+v, want the newest three (/3, /4, /5)", entries)
	}
	if got := recorder.Dropped(); got != 2 {
		t.Fatalf("Dropped() = %d, want 2", got)
	}
	if firstDrops != 1 {
		t.Fatalf("first-drop callback ran %d times, want exactly once", firstDrops)
	}
}

func TestRecorderReportsNoDropsUnderTheLimit(t *testing.T) {
	called := false
	recorder := NewLimitedRecorder(3, nil, func() { called = true })
	recorder.Add(Request{Path: "/only"})
	if got := recorder.Dropped(); got != 0 || called {
		t.Fatalf("Dropped() = %d, callback called = %v, want 0 and false", got, called)
	}
}

func TestNewRecorderKeepsTenThousandRequests(t *testing.T) {
	if DefaultMaxRequests != 10000 {
		t.Fatalf("DefaultMaxRequests = %d, want 10000", DefaultMaxRequests)
	}
	for name, recorder := range map[string]*Recorder{
		"NewRecorder":                  NewRecorder(nil),
		"NewLimitedRecorder below one": NewLimitedRecorder(0, nil, nil),
	} {
		for i := 0; i < DefaultMaxRequests+1; i++ {
			recorder.Add(Request{})
		}
		if got := len(recorder.Snapshot()); got != DefaultMaxRequests {
			t.Errorf("%s kept %d requests, want %d", name, got, DefaultMaxRequests)
		}
		if got := recorder.Dropped(); got != 1 {
			t.Errorf("%s Dropped() = %d, want 1", name, got)
		}
	}
}
