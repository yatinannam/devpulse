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
	Time time.Time
	Method string
	Path string
	Status int
	Latency time.Duration
	Bytes int64
}

type Recorder struct {
	mu sync.RWMutex
	entries []Request
	onAdd func(Request)
}

func NewRecorder(onAdd func(Request)) *Recorder {
	return &Recorder{onAdd: onAdd}
}

func (r *Recorder) Add(entry Request) {
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	if len(r.entries) > 1000 {
		r.entries = r.entries[len(r.entries)-1000:]
	}
	r.mu.Unlock()
	if r.onAdd != nil {
		r.onAdd(entry)
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
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-DevPulse", "1")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if start, ok := resp.Request.Context().Value(startKey{}).(time.Time); ok {
			recorder.Add(Request{
				Time: start,
				Method: resp.Request.Method,
				Path: resp.Request.URL.RequestURI(),
				Status: resp.StatusCode,
				Latency: time.Since(start),
				Bytes: resp.ContentLength,
			})
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("proxy error: %s %v", req.URL.RequestURI(), err)
		http.Error(w, "devpulse proxy error", http.StatusBadGateway)
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

func Serve(proxy http.Handler, addr string) error {
	return http.ListenAndServe(addr, proxy)
}

func DrainBody(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
