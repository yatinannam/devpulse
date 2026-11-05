package traffic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Request struct {
	Time       time.Time     `json:"time"`
	Method     string        `json:"method"`
	Path       string        `json:"path"`
	Status     int           `json:"status"`
	Latency    time.Duration `json:"latency"`
	Bytes      int64         `json:"bytes"`
	TargetHost string        `json:"targetHost"`
	TargetPort int           `json:"targetPort"`
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
	if err := checkTarget(target, u); err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	original := proxy.Director
	proxy.Director = func(req *http.Request) { original(req); req.Header.Set("X-DevPulse", "1") }
	record := func(req *http.Request, status int, bytes int64, state *requestState) {
		port := 0
		if p := u.Port(); p != "" {
			fmt.Sscanf(p, "%d", &port)
		} else if u.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
		recorder.Add(Request{Time: state.start, Method: req.Method, Path: state.uri, Status: status, Latency: time.Since(state.start), Bytes: bytes, TargetHost: u.Hostname(), TargetPort: port})
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		state, ok := resp.Request.Context().Value(stateKey{}).(*requestState)
		if !ok {
			return nil
		}
		req, status := resp.Request, resp.StatusCode
		finish := func(bytes int64) {
			state.once.Do(func() { record(req, status, bytes, state) })
		}
		if status == http.StatusSwitchingProtocols {
			// ReverseProxy needs the original read-write body to tunnel an
			// upgraded connection, so record the upgrade itself and leave the
			// body alone.
			finish(0)
			return nil
		}
		resp.Body = &countingBody{ReadCloser: resp.Body, done: finish}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) {
			// The client went away; neither the proxy nor the upstream failed.
			status = StatusClientClosedRequest
		}
		if state, ok := req.Context().Value(stateKey{}).(*requestState); ok {
			state.once.Do(func() { record(req, status, 0, state) })
		}
		if status == StatusClientClosedRequest {
			return
		}
		log.Printf("proxy error: %s %v", req.URL.RequestURI(), err)
		http.Error(w, "devpulse proxy error", 502)
	}
	return proxy, nil
}

// checkTarget rejects targets the proxy cannot forward to. url.Parse accepts
// "localhost:3000" as scheme "localhost", which would otherwise start the
// proxy and fail every request with "unsupported protocol scheme".
func checkTarget(target string, u *url.URL) error {
	const example = "http://localhost:3000"
	webScheme := u.Scheme == "http" || u.Scheme == "https"
	switch {
	case !webScheme && !strings.Contains(target, "://"):
		suggestion := example
		if target != "" {
			suggestion = "http://" + target
		}
		return fmt.Errorf("invalid target %q: include the scheme, for example %s", target, suggestion)
	case !webScheme:
		return fmt.Errorf("invalid target %q: the scheme must be http or https", target)
	case u.Hostname() == "":
		return fmt.Errorf("invalid target %q: missing host, for example %s", target, example)
	}
	return nil
}

// StatusClientClosedRequest is recorded when the client cancels a request
// before the upstream answered (nginx's non-standard 499).
const StatusClientClosedRequest = 499

type stateKey struct{}

// requestState is the per-request bookkeeping Handler stores in the request
// context. uri is the client's request URI, captured before the proxy rewrites
// the URL for the upstream. once guarantees a proxied request is recorded
// exactly once, by whichever of the response body or the error handler
// finishes it first.
type requestState struct {
	start time.Time
	uri   string
	once  sync.Once
}

// countingBody wraps an upstream response body. It counts the bytes the proxy
// streams to the client and reports the total when the body reaches EOF or is
// closed, whichever happens first. ReverseProxy reads and closes the body on
// the handler goroutine, so the counter needs no locking.
type countingBody struct {
	io.ReadCloser
	n    int64
	done func(n int64)
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.n += int64(n)
	if err == io.EOF {
		b.done(b.n)
	}
	return n, err
}

func (b *countingBody) Close() error {
	b.done(b.n)
	return b.ReadCloser.Close()
}

// ProbeHeader marks DevPulse's own service-discovery requests. The proxy
// answers them itself with ProxyHeader set, so `status`, `watch` and `init`
// can recognise the proxy without their probes being forwarded upstream or
// recorded into the captured session.
const (
	ProbeHeader = "X-DevPulse-Probe"
	ProxyHeader = "X-DevPulse-Proxy"
)

func Handler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ProbeHeader) != "" {
			w.Header().Set(ProxyHeader, "1")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		ctx := context.WithValue(r.Context(), stateKey{}, &requestState{start: time.Now(), uri: r.URL.RequestURI()})
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
func DrainBody(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
