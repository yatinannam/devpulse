package traffic

import (
	"io"
	"net/http"
	"net/http/httptest"
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
