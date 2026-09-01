package traffic

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Request struct {
	Time       time.Time
	Method     string
	Path       string
	Status     int
	Latency    time.Duration
	Bytes      int64
	TargetHost string
	TargetPort int
}
type Recorder struct {
	mu      sync.RWMutex
	entries []Request
	onAdd   func(Request)
}

func NewRecorder(onAdd func(Request)) *Recorder { return &Recorder{onAdd: onAdd} }
func (r *Recorder) Add(e Request) {
	r.mu.Lock()
	r.entries = append(r.entries, e)
	if len(r.entries) > 1000 {
		r.entries = r.entries[len(r.entries)-1000:]
	}
	r.mu.Unlock()
	if r.onAdd != nil {
		r.onAdd(e)
	}
}
func (r *Recorder) Snapshot() []Request {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Request, len(r.entries))
	copy(out, r.entries)
	return out
}

func NewProxy(target string, recorder *Recorder) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("invalid target: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	original := proxy.Director
	proxy.Director = func(req *http.Request) { original(req); req.Header.Set("X-DevPulse", "1") }
	record := func(req *http.Request, status int, bytes int64, start time.Time) {
		port := 0
		if p := u.Port(); p != "" {
			fmt.Sscanf(p, "%d", &port)
		} else if u.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
		recorder.Add(Request{Time: start, Method: req.Method, Path: req.URL.RequestURI(), Status: status, Latency: time.Since(start), Bytes: bytes, TargetHost: u.Hostname(), TargetPort: port})
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if start, ok := resp.Request.Context().Value(startKey{}).(time.Time); ok {
			record(resp.Request, resp.StatusCode, resp.ContentLength, start)
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		if start, ok := req.Context().Value(startKey{}).(time.Time); ok {
			record(req, 502, 0, start)
		}
		log.Printf("proxy error: %s %v", req.URL.RequestURI(), err)
		http.Error(w, "devpulse proxy error", 502)
	}
	return proxy, nil
}

type startKey struct{}

func Handler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), startKey{}, time.Now())
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
func DrainBody(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
