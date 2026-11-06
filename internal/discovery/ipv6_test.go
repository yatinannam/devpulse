package discovery

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/ports"
)

func TestProbeEntryUsesIPv4LoopbackFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	got := probeEntry(ports.Entry{Port: port}, Timeouts{Dial: time.Second, HTTP: time.Second})
	if wantURL := fmt.Sprintf("http://127.0.0.1:%d", port); !got.HTTP || got.URL != wantURL {
		t.Fatalf("probeEntry() = %+v, want HTTP service at %s", got, wantURL)
	}
}

// TestProbeEntryFallsBackToIPv6Loopback covers a server bound only to ::1,
// which is what Vite does for "localhost" on Node 17 and later.
func TestProbeEntryFallsBackToIPv6Loopback(t *testing.T) {
	ln, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "uvicorn")
		w.WriteHeader(http.StatusOK)
	}))
	server.Listener.Close()
	server.Listener = ln
	server.Start()
	defer server.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	got := probeEntry(ports.Entry{Port: port, PID: "1", State: "LISTEN"}, Timeouts{Dial: time.Second, HTTP: time.Second})

	wantURL := fmt.Sprintf("http://[::1]:%d", port)
	if !got.HTTP || got.URL != wantURL || got.Kind != "Python / FastAPI" {
		t.Fatalf("probeEntry() = %+v, want HTTP service at %s identified as Python / FastAPI", got, wantURL)
	}
}
