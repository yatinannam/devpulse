package traffic

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startCapture puts a capturing proxy in front of target. Closing the returned
// server waits for every in-flight proxied request to finish, so tests close it
// before they read the recorder.
func startCapture(t *testing.T, target string) (*Recorder, *httptest.Server) {
	t.Helper()
	recorder := NewRecorder(nil)
	proxy, err := NewProxy(target, recorder)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Handler(proxy))
	t.Cleanup(server.Close)
	return recorder, server
}

// fetch sends one request through the proxy and reads the whole response.
func fetch(t *testing.T, method, url string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func onlyEntry(t *testing.T, recorder *Recorder) Request {
	t.Helper()
	entries := recorder.Snapshot()
	if len(entries) != 1 {
		t.Fatalf("got %d recorded requests, want exactly 1: %+v", len(entries), entries)
	}
	return entries[0]
}

func TestProxyRecordsStreamedBytesForChunkedResponse(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for _, chunk := range []string{"hello, ", "chunked ", "world"} {
			_, _ = io.WriteString(w, chunk)
			flusher.Flush()
		}
	}))
	defer backend.Close()
	recorder, proxy := startCapture(t, backend.URL)

	_, body := fetch(t, http.MethodGet, proxy.URL+"/stream")
	proxy.Close()

	if string(body) != "hello, chunked world" {
		t.Fatalf("client got body %q", body)
	}
	if got := onlyEntry(t, recorder).Bytes; got != int64(len(body)) {
		t.Fatalf("recorded %d bytes, want the %d bytes actually streamed", got, len(body))
	}
}

func TestProxyLatencyCoversTheWholeResponseBody(t *testing.T) {
	const delay = 150 * time.Millisecond
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "first ")
		w.(http.Flusher).Flush()
		time.Sleep(delay)
		_, _ = io.WriteString(w, "second")
	}))
	defer backend.Close()
	recorder, proxy := startCapture(t, backend.URL)

	fetch(t, http.MethodGet, proxy.URL+"/slow-body")
	proxy.Close()

	if got := onlyEntry(t, recorder).Latency; got < delay {
		t.Fatalf("recorded latency %s, want at least the %s the body took to stream", got, delay)
	}
}

func TestProxyRecordsBodylessResponsesExactlyOnce(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/empty" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	for _, tc := range []struct {
		name, method, path string
		status             int
	}{
		{name: "HEAD", method: http.MethodHead, path: "/item", status: http.StatusOK},
		{name: "204 No Content", method: http.MethodGet, path: "/empty", status: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder, proxy := startCapture(t, backend.URL)
			resp, _ := fetch(t, tc.method, proxy.URL+tc.path)
			proxy.Close()

			if resp.StatusCode != tc.status {
				t.Fatalf("client got status %d, want %d", resp.StatusCode, tc.status)
			}
			entry := onlyEntry(t, recorder)
			if entry.Status != tc.status || entry.Method != tc.method {
				t.Fatalf("recorded %s %d, want %s %d", entry.Method, entry.Status, tc.method, tc.status)
			}
			if entry.Bytes != 0 {
				t.Fatalf("recorded %d bytes for a response without a body, want 0", entry.Bytes)
			}
		})
	}
}

func TestNewProxyRejectsUnusableTargets(t *testing.T) {
	for _, tc := range []struct{ target, want string }{
		{target: "localhost:3000", want: `invalid target "localhost:3000": include the scheme, for example http://localhost:3000`},
		{target: "ftp://x", want: `invalid target "ftp://x": the scheme must be http or https`},
		{target: "http://", want: `invalid target "http://": missing host`},
		{target: "http://localhost:0", want: `invalid target "http://localhost:0": the port must be between 1 and 65535`},
		{target: "http://localhost:99999", want: `invalid target "http://localhost:99999": the port must be between 1 and 65535`},
	} {
		_, err := NewProxy(tc.target, NewRecorder(nil))
		if err == nil {
			t.Errorf("NewProxy(%q) succeeded, want an error", tc.target)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("NewProxy(%q) error = %q, want it to contain %q", tc.target, err, tc.want)
		}
	}
	if _, err := NewProxy("http://localhost:3000", NewRecorder(nil)); err != nil {
		t.Fatalf("NewProxy rejected a valid target: %v", err)
	}
}

func TestCheckTargetMatchesNewProxy(t *testing.T) {
	// config --target uses CheckTarget, so it must refuse exactly what
	// NewProxy refuses, with the same message.
	for _, target := range []string{"localhost:3000", "ftp://x", "http://", "http://localhost:0", "http://localhost:99999", "%zz"} {
		_, proxyErr := NewProxy(target, NewRecorder(nil))
		checkErr := CheckTarget(target)
		if proxyErr == nil || checkErr == nil || proxyErr.Error() != checkErr.Error() {
			t.Errorf("target %q: CheckTarget = %v, NewProxy = %v; want the same error", target, checkErr, proxyErr)
		}
	}
	for _, target := range []string{"http://localhost:3000", "https://api.local", "http://127.0.0.1:8080/api"} {
		if err := CheckTarget(target); err != nil {
			t.Errorf("CheckTarget(%q) = %v, want nil", target, err)
		}
	}
}

func TestProxyRecordsTheClientRequestURI(t *testing.T) {
	upstreamURI := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamURI <- r.URL.RequestURI()
	}))
	defer backend.Close()
	recorder, proxy := startCapture(t, backend.URL+"/api")

	fetch(t, http.MethodGet, proxy.URL+"/users?x=1")
	proxy.Close()

	if got := <-upstreamURI; got != "/api/users?x=1" {
		t.Fatalf("upstream received %q, want the target path prefix applied", got)
	}
	if got := onlyEntry(t, recorder).Path; got != "/users?x=1" {
		t.Fatalf("recorded path %q, want the client's request URI /users?x=1", got)
	}
}

func TestProxyRecordsTheRequestTargetAsSent(t *testing.T) {
	// The path used to be rebuilt from the parsed URL, which re-encodes
	// characters browsers send raw: /a|b^c was recorded as /a%7Cb%5Ec.
	// Targets that are not origin-form keep only their path and query, so
	// every recorded path still begins with "/" or is "*", as
	// devpulse validate requires.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()
	for _, tc := range []struct{ line, want string }{
		{"GET /a|b^c?q=é HTTP/1.1", "/a|b^c?q=é"},
		{"GET /a#frag HTTP/1.1", "/a#frag"},
		{"GET //twice HTTP/1.1", "//twice"},
		{"GET * HTTP/1.1", "*"},
		// JSON cannot hold invalid UTF-8, so keep the percent-encoded form.
		{"GET /\xff HTTP/1.1", "/%FF"},
		// Absolute-form, authority-form and opaque targets.
		{"GET http://example.com/x?y=1 HTTP/1.1", "/x?y=1"},
		{"GET http://example.com HTTP/1.1", "/"},
		{"GET h:80 HTTP/1.1", "/"},
		{"GET http:foo HTTP/1.1", "/"},
		{"CONNECT example.com:443 HTTP/1.1", "/"},
	} {
		recorder, proxy := startCapture(t, backend.URL)
		conn, err := net.Dial("tcp", proxy.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(conn, tc.line+"\r\nHost: example.com\r\nConnection: close\r\n\r\n")
		if resp, err := http.ReadResponse(bufio.NewReader(conn), nil); err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		conn.Close()
		proxy.Close()
		entries := recorder.Snapshot()
		if len(entries) != 1 {
			t.Errorf("%q: recorded %d requests, want 1", tc.line, len(entries))
			continue
		}
		got := entries[0].Path
		if got != tc.want {
			t.Errorf("%q: recorded path %q, want %q", tc.line, got, tc.want)
		}
		if !strings.HasPrefix(got, "/") && got != "*" {
			t.Errorf("%q: recorded path %q would fail session validation", tc.line, got)
		}
	}
}

func TestProxyRecordsClientCancellationAs499(t *testing.T) {
	arrived := make(chan struct{}, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case arrived <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer backend.Close()
	recorder, proxy := startCapture(t, backend.URL)

	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previous)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, proxy.URL+"/slow", nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		done <- err
	}()
	select {
	case <-arrived:
	case <-time.After(5 * time.Second):
		t.Fatal("request never reached the backend")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("client error = %v, want context.Canceled", err)
	}
	proxy.Close()

	if got := onlyEntry(t, recorder).Status; got != 499 {
		t.Fatalf("recorded status %d for a cancelled request, want 499", got)
	}
	if strings.Contains(logs.String(), "proxy error") {
		t.Fatalf("client cancellation was logged as a proxy error: %q", logs.String())
	}
}

func TestProxyRecordsFailedUpgradeOnlyOnce(t *testing.T) {
	// A raw backend that answers a plain GET with an upgrade the client never
	// asked for. ReverseProxy passes the 101 to ModifyResponse and then fails
	// the switch through ErrorHandler.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	release := make(chan struct{})
	defer close(release)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		_, _ = io.WriteString(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: unrequested\r\nConnection: Upgrade\r\n\r\n")
		<-release
	}()
	recorder, proxy := startCapture(t, "http://"+listener.Addr().String())

	resp, _ := fetch(t, http.MethodGet, proxy.URL+"/socket")
	proxy.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("client got status %d, want 502 from the failed switch", resp.StatusCode)
	}
	if got := onlyEntry(t, recorder).Status; got != http.StatusSwitchingProtocols {
		t.Fatalf("recorded status %d, want the 101 seen by ModifyResponse", got)
	}
}

func TestProxyAnswersARequestThatAlreadyPassedADevPulseProxyWith508(t *testing.T) {
	hits := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.WriteString(w, "upstream")
	}))
	defer backend.Close()
	recorder, proxy := startCapture(t, backend.URL)

	req, err := http.NewRequest(http.MethodGet, proxy.URL+"/again", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(ForwardedHeader, "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	proxy.Close()

	if resp.StatusCode != http.StatusLoopDetected {
		t.Fatalf("client got %d %q, want 508", resp.StatusCode, body)
	}
	if hits != 0 {
		t.Fatalf("a looped request was forwarded upstream %d times", hits)
	}
	entry := onlyEntry(t, recorder)
	if entry.Status != http.StatusLoopDetected || entry.Path != "/again" || entry.Bytes != int64(len(body)) {
		t.Fatalf("recorded %+v, want a 508 for /again with %d bytes", entry, len(body))
	}
}

func TestProxyWhoseTargetIsItselfStopsAfterOneHop(t *testing.T) {
	// --target pointing at --listen used to recurse until the process ran
	// out of memory: one client request became thousands.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	recorder := NewRecorder(nil)
	proxy, err := NewProxy("http://"+listener.Addr().String(), recorder)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(Handler(proxy))
	server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(server.URL + "/loop")
	if err != nil {
		t.Fatalf("request through a self-targeting proxy failed: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	server.Close()

	if resp.StatusCode != http.StatusLoopDetected {
		t.Fatalf("client got %d, want 508", resp.StatusCode)
	}
	// The client's request and the one copy the proxy sent to itself.
	if entries := recorder.Snapshot(); len(entries) != 2 {
		t.Fatalf("recorded %d requests for one looped request, want 2", len(entries))
	}
}

func TestProxyRecordsAnUpstreamStatusBelow100As502(t *testing.T) {
	// Go's client accepts any three-digit status, but ResponseWriter panics
	// on codes below 100, so a bogus "000" used to crash the handler and the
	// request was never recorded.
	for _, code := range []string{"000", "099"} {
		t.Run(code, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
					return
				}
				_, _ = io.WriteString(conn, "HTTP/1.1 "+code+" Bogus\r\nContent-Length: 2\r\n\r\nhi")
			}()
			recorder := NewRecorder(nil)
			proxy, err := NewProxy("http://"+listener.Addr().String(), recorder)
			if err != nil {
				t.Fatal(err)
			}
			var serverLog, proxyLog bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&proxyLog)
			defer log.SetOutput(previous)
			server := httptest.NewUnstartedServer(Handler(proxy))
			server.Config.ErrorLog = log.New(&serverLog, "", 0)
			server.Start()
			defer server.Close()

			resp, err := http.Get(server.URL + "/bogus")
			if err != nil {
				t.Fatalf("client error %v; server log: %s", err, serverLog.String())
			}
			resp.Body.Close()
			server.Close()

			if resp.StatusCode != http.StatusBadGateway {
				t.Fatalf("client got %d, want 502", resp.StatusCode)
			}
			if strings.Contains(serverLog.String(), "panic") {
				t.Fatalf("handler panicked: %s", serverLog.String())
			}
			if got := onlyEntry(t, recorder).Status; got != http.StatusBadGateway {
				t.Fatalf("recorded status %d, want 502", got)
			}
		})
	}
}
