package discovery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestProbeIdentifiesDevPulseProxyWithoutRecordingTraffic(t *testing.T) {
	backendHits := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	recorder := traffic.NewRecorder(nil)
	proxy, err := traffic.NewProxy(backend.URL, recorder)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(traffic.Handler(proxy))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}

	isHTTP, kind, _ := probe(port, 2*time.Second)
	if !isHTTP || kind != KindDevPulseProxy {
		t.Fatalf("probe = (%v, %q), want (true, %q)", isHTTP, kind, KindDevPulseProxy)
	}
	if got := len(recorder.Snapshot()); got != 0 {
		t.Fatalf("discovery probe was recorded as %d captured request(s)", got)
	}
	if backendHits != 0 {
		t.Fatalf("discovery probe was forwarded to the upstream app %d time(s)", backendHits)
	}
}
