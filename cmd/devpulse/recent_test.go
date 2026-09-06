package main

import (
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestRecentRequests(t *testing.T) {
	entries := []traffic.Request{
		{Path: "/one", Time: time.Unix(1, 0)},
		{Path: "/two", Time: time.Unix(2, 0)},
		{Path: "/three", Time: time.Unix(3, 0)},
	}
	got := recentRequests(entries, 2)
	if len(got) != 2 || got[0].Path != "/two" || got[1].Path != "/three" {
		t.Fatalf("got %+v, want /two /three", got)
	}
}

func TestRecentRequestsHandlesLargeLimit(t *testing.T) {
	entries := []traffic.Request{{Path: "/one"}}
	got := recentRequests(entries, 10)
	if len(got) != 1 || got[0].Path != "/one" {
		t.Fatalf("got %+v, want original entry", got)
	}
}

func TestRecentRequestsRejectsNonPositiveLimit(t *testing.T) {
	entries := []traffic.Request{{Path: "/one"}}
	if got := recentRequests(entries, 0); got != nil {
		t.Fatalf("got %+v, want nil", got)
	}
}
