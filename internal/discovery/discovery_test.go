package discovery

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yatinannam/devpulse/internal/traffic"
)

func TestIdentify(t *testing.T) {
	cases := []struct {
		name, server, body, want string
		// extra holds response headers besides Server.
		extra map[string]string
	}{
		{"fastapi", "uvicorn", "", "Python / FastAPI", nil},
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
