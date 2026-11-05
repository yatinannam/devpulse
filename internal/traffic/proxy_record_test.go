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
