package discovery

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yatinannam/devpulse/internal/ports"
)

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
			s := Service{Port: entry.Port, Process: entry.Process, PID: entry.PID, State: entry.State}
			s.HTTP, s.Kind, s.Server = probe(entry.Port, timeout)
			if s.HTTP {
				s.URL = fmt.Sprintf("http://127.0.0.1:%d", entry.Port)
			}
			out[idx] = s
		}(i, e)
	}
	wg.Wait()
	return out, nil
}

func probe(port int, timeout time.Duration) (bool, string, string) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, "", ""
	}
	_ = conn.Close()

	client := http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	// Start with HEAD to avoid downloading application pages. If headers do not identify the service, retry with a bounded GET.
	req, err := http.NewRequest(http.MethodHead, "http://"+addr+"/", nil)
	if err != nil {
		return false, "", ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", ""
	}
	defer resp.Body.Close()

	server := resp.Header.Get("Server")
	kind := identify(resp.Header, "")
	if kind == "HTTP service" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		kind = identify(resp.Header, string(body))
	}
	return true, kind, server
}

func identify(h http.Header, body string) string {
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
	if strings.Contains(server, "express") {
		return "Node / Express"
	}
	if strings.Contains(server, "jetty") || strings.Contains(server, "tomcat") || strings.Contains(x, "spring") {
		return "Java / Spring"
	}
	if strings.Contains(server, "caddy") {
		return "Caddy"
	}
	if strings.Contains(server, "nginx") {
		return "Nginx"
	}
	return "HTTP service"
}
