package discovery

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestIdentify(t *testing.T) {
	cases := []struct {
		name, server, body, want string
		// extra holds response headers besides Server.
		extra map[string]string
	}{
		{"fastapi", "uvicorn", "", "Python / FastAPI", nil},
		{"fastapi body", "", `{"title":"FastAPI"}`, "Python / FastAPI", nil},
		// A frontend that names its FastAPI backend is still the frontend.
		{"vite page mentions fastapi", "", `<script type="module" src="/@vite/client"></script><title>Full Stack FastAPI Project</title>`, "Node / Vite", nil},
		{"next page mentions fastapi", "", `<link href="/_next/static/css/app.css"><h1>Built with FastAPI</h1>`, "Node / Next.js", nil},
		{"next", "", "_next/static/chunks", "Node / Next.js", nil},
		{"vite", "", "<script src=\"/@vite/client\">", "Node / Vite", nil},
		{"spring", "tomcat", "", "Java / Spring", nil},
		{"generic", "", "", "HTTP service", nil},
		// Stock Express sends X-Powered-By but no Server header.
		{"express powered-by", "", "Cannot GET /", "Node / Express", map[string]string{"X-Powered-By": "Express"}},
		{"express server", "express", "", "Node / Express", nil},
		{"spring whitelabel", "", "<html><body><h1>Whitelabel Error Page</h1></body></html>", "Java / Spring", nil},
		{"spring application context", "", "", "Java / Spring", map[string]string{"X-Application-Context": "application:8080"}},
		// A page that merely mentions spring is not a Spring app.
		{"mentions spring", "", "<h1>Spring sale: 20% off garden tools</h1>", "HTTP service", nil},
		{"proxy first", "express", "Whitelabel Error Page", KindDevPulseProxy, map[string]string{traffic.ProxyHeader: "1", "X-Powered-By": "Express"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("Server", tc.server)
			for k, v := range tc.extra {
				h.Set(k, v)
			}
			if got := identify(h, tc.body); got != tc.want {
				t.Fatalf("identify()=%q want %q", got, tc.want)
			}
		})
	}
}

// TestProbeIdentifiesStockSpringBoot mimics Spring Boot's error controller:
// no Server header, and the Whitelabel Error Page only for clients that
// accept HTML. Other clients get a JSON error body.
func TestProbeIdentifiesStockSpringBoot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Content-Type", "text/html;charset=UTF-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("<html><body><h1>Whitelabel Error Page</h1><p>This application has no explicit mapping for /error, so you are seeing this as a fallback.</p></body></html>"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"timestamp":"2026-11-05T10:00:00.000+00:00","status":404,"error":"Not Found","path":"/"}`))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	if isHTTP, kind, _, _ := probe(port, 2*time.Second); !isHTTP || kind != "Java / Spring" {
		t.Fatalf("probe = (%v, %q), want (true, %q)", isHTTP, kind, "Java / Spring")
	}
}

// TestProbeWaitsForASlowFirstResponse mimics next dev, which compiles a page
// on its first request. The dial timeout that init passes (250 ms) must not
// also cut off the HTTP requests.
func TestProbeWaitsForASlowFirstResponse(t *testing.T) {
	var compile sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compile.Do(func() { time.Sleep(600 * time.Millisecond) })
		w.Header().Set("X-Powered-By", "Next.js")
		_, _ = w.Write([]byte(`<script src="/_next/static/chunks/main.js"></script>`))
	}))
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	got := probeEntry(ports.Entry{Port: port}, 250*time.Millisecond)
	if !got.HTTP || got.Kind != "Node / Next.js" {
		t.Fatalf("probeEntry() = %+v, want an HTTP service identified as Node / Next.js", got)
	}
}

// TestProbeRejectsAnHTTPSServer covers Go TLS servers (Caddy, the Kubernetes
// API, Traefik), which answer plain HTTP with a 400 instead of closing the
// connection. Saving http:// for one would make every proxied request fail.
func TestProbeRejectsAnHTTPSServer(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()

	port := server.Listener.Addr().(*net.TCPAddr).Port
	if got := probeEntry(ports.Entry{Port: port}, 250*time.Millisecond); got.HTTP || got.URL != "" {
		t.Fatalf("probeEntry() = %+v, want an HTTPS-only server to be non-HTTP with no URL", got)
	}
}

// TestProbeIsBoundedAndLeavesNoGoroutines covers a listener that accepts
// connections and never answers. The probe must give up within its budget
// and leave no connection goroutines behind once it returns.
func TestProbeIsBoundedAndLeavesNoGoroutines(t *testing.T) {
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	var conns []net.Conn
	var mu sync.Mutex
	go func() {
		for {
			c, err := silent.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c)
			mu.Unlock()
		}
	}()
	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	answering := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer answering.Close()

	before := runtime.NumGoroutine()
	start := time.Now()
	got := probeEntry(ports.Entry{Port: silent.Addr().(*net.TCPAddr).Port}, 250*time.Millisecond)
	if elapsed := time.Since(start); elapsed > httpBudget+time.Second {
		t.Fatalf("probe of a silent listener took %s, want at most about %s", elapsed, httpBudget)
	}
	if got.HTTP {
		t.Fatalf("probeEntry() = %+v, want a listener that never answers to be non-HTTP", got)
	}
	if got := probeEntry(ports.Entry{Port: answering.Listener.Addr().(*net.TCPAddr).Port}, 250*time.Millisecond); !got.HTTP {
		t.Fatalf("probeEntry() = %+v, want HTTP", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		t.Fatalf("%d goroutines after the probes, %d before: probe connections were left open", n, before)
	}
}
