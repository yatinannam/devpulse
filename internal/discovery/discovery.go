package discovery

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yatinannam/devpulse/internal/ports"
	"github.com/yatinannam/devpulse/internal/traffic"
)

// KindDevPulseProxy labels a listener that is DevPulse's own traffic proxy.
const KindDevPulseProxy = "DevPulse proxy"

// KindHTTPService labels a listener that answers HTTP but matched no
// fingerprint.
const KindHTTPService = "HTTP service"

type Service struct {
	Port    int
	Process string
	PID     string
	State   string
	HTTP    bool
	URL     string
	Kind    string
	Server  string
}

func Discover(timeout time.Duration) ([]Service, error) {
	entries, err := ports.List()
	if err != nil {
		return nil, err
	}
	out := make([]Service, len(entries))
	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(idx int, entry ports.Entry) {
			defer wg.Done()
			out[idx] = probeEntry(entry, timeout)
		}(i, e)
	}
	wg.Wait()
	return out, nil
}

// probeEntry probes one listener and describes it as a Service.
func probeEntry(entry ports.Entry, timeout time.Duration) Service {
	s := Service{Port: entry.Port, Process: entry.Process, PID: entry.PID, State: entry.State}
	var addr string
	s.HTTP, s.Kind, s.Server, addr = probe(entry.Port, timeout)
	if s.HTTP {
		s.URL = "http://" + addr
	}
	return s
}

// loopbackHosts are dialled in order. A server bound only to ::1 (Vite binds
// "localhost" there on Node 17 and later) refuses 127.0.0.1.
var loopbackHosts = []string{"127.0.0.1", "::1"}

// dialLoopback returns the first loopback host:port that accepts a TCP
// connection, for example "127.0.0.1:3000" or "[::1]:5173".
func dialLoopback(port int, timeout time.Duration) (string, bool) {
	for _, host := range loopbackHosts {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, timeout)
		if err == nil {
			_ = conn.Close()
			return addr, true
		}
	}
	return "", false
}

// probe reports whether the listener on port speaks HTTP, its fingerprint,
// its Server header and the loopback host:port that answered.
func probe(port int, timeout time.Duration) (bool, string, string, string) {
	addr, ok := dialLoopback(port, timeout)
	if !ok {
		return false, "", "", ""
	}

	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// Start with HEAD to avoid downloading application pages. If headers do not identify the service, retry with a bounded GET.
	req, err := http.NewRequest(http.MethodHead, "http://"+addr+"/", nil)
	if err != nil {
		return false, "", "", ""
	}
	req.Header.Set(traffic.ProbeHeader, "1")
	resp, err := client.Do(req)
	if err != nil {
		return false, "", "", ""
	}
	defer resp.Body.Close()

	server := resp.Header.Get("Server")
	kind := identify(resp.Header, "")
	if kind == KindHTTPService {
		_ = resp.Body.Close()
		getReq, err := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
		if err == nil {
			getReq.Header.Set(traffic.ProbeHeader, "1")
			// Ask for HTML as a browser does. Spring Boot renders its Whitelabel
			// Error Page only for HTML clients and sends JSON to the rest.
			getReq.Header.Set("Accept", "text/html,*/*;q=0.8")
			getResp, err := client.Do(getReq)
			if err == nil {
				body, _ := io.ReadAll(io.LimitReader(getResp.Body, 8*1024))
				kind = identify(getResp.Header, string(body))
				_ = getResp.Body.Close()
			}
		}
	}
	return true, kind, server, addr
}

func identify(h http.Header, body string) string {
	if h.Get(traffic.ProxyHeader) != "" {
		return KindDevPulseProxy
	}
	server := strings.ToLower(h.Get("Server"))
	x := strings.ToLower(body)
	if strings.Contains(server, "uvicorn") || strings.Contains(x, "fastapi") {
		return "Python / FastAPI"
	}
	if strings.Contains(server, "gunicorn") {
		return "Python / Gunicorn"
	}
	if strings.Contains(server, "next") || strings.Contains(x, "__next_data__") || strings.Contains(x, "_next/static") {
		return "Node / Next.js"
	}
	if strings.Contains(server, "vite") || strings.Contains(x, "@vite") || strings.Contains(x, "vite/client") {
		return "Node / Vite"
	}
	// Stock Express sends X-Powered-By: Express and no Server header.
	if strings.Contains(server, "express") || strings.Contains(strings.ToLower(h.Get("X-Powered-By")), "express") {
		return "Node / Express"
	}
	// The word "spring" in a page is no evidence of Spring. Spring Boot 1.x
	// sends X-Application-Context; later versions render the Whitelabel Error
	// Page for an unmapped path.
	if strings.Contains(server, "jetty") || strings.Contains(server, "tomcat") || h.Get("X-Application-Context") != "" || strings.Contains(x, "whitelabel error page") {
		return "Java / Spring"
	}
	if strings.Contains(server, "caddy") {
		return "Caddy"
	}
	if strings.Contains(server, "nginx") {
		return "Nginx"
	}
	return KindHTTPService
}
