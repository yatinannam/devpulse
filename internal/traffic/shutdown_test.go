package traffic

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestStartSetsAReadHeaderTimeout(t *testing.T) {
	server, _, err := Start(http.NotFoundHandler(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer Shutdown(server, 0)
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("ReadHeaderTimeout = %s, want 10s", server.ReadHeaderTimeout)
	}
}

func TestShutdownReturnsWithinTheDeadlineDespiteAHungHandler(t *testing.T) {
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	defer close(release)
	server, serveErr, err := Start(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release // ignores cancellation, like a stuck SSE stream
	}), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		if resp, err := http.Get("http://" + server.Addr + "/hang"); err == nil {
			resp.Body.Close()
		}
	}()
	waitFor(t, entered, "the handler to start")

	const timeout = 200 * time.Millisecond
	began := time.Now()
	err = Shutdown(server, timeout)
	if elapsed := time.Since(began); elapsed > timeout+2*time.Second {
		t.Fatalf("Shutdown took %s with a %s deadline", elapsed, timeout)
	}
	if err == nil {
		t.Fatal("Shutdown returned nil, want an error saying connections were closed at the deadline")
	}
	select {
	case err, open := <-serveErr:
		if open {
			t.Fatalf("Serve reported %v after Shutdown, want no error", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after Shutdown")
	}
}

func TestShutdownDoesNotWaitForAConnectionThatNeverSentARequest(t *testing.T) {
	// Browsers open speculative connections they may never use. net/http
	// treats such a StateNew connection as busy for its first 5 seconds,
	// so Shutdown used to wait the whole 5s deadline and then report that
	// requests were cut off although none was running.
	server, _, err := Start(http.NotFoundHandler(), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond) // let the server accept it

	began := time.Now()
	err = Shutdown(server, DefaultShutdownTimeout)
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Errorf("Shutdown took %s with only an unused connection open", elapsed)
	}
	if err != nil {
		t.Errorf("Shutdown = %v with no request in flight, want nil", err)
	}
}

func TestShutdownKeepsRequestsThatWereInFlight(t *testing.T) {
	arrived := make(chan struct{}, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrived <- struct{}{}
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, "finished")
	}))
	defer backend.Close()
	recorder := NewRecorder(nil)
	proxy, err := NewProxy(backend.URL, recorder)
	if err != nil {
		t.Fatal(err)
	}
	server, _, err := Start(Handler(proxy), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + server.Addr + "/in-flight")
		if err != nil {
			body <- err.Error()
			return
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		body <- string(data)
	}()
	waitFor(t, arrived, "the request to reach the backend")

	if err := Shutdown(server, DefaultShutdownTimeout); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	entries := recorder.Snapshot()
	if len(entries) != 1 || entries[0].Path != "/in-flight" || entries[0].Status != http.StatusOK {
		t.Fatalf("snapshot after shutdown = %+v, want the in-flight request", entries)
	}
	if got := <-body; got != "finished" {
		t.Fatalf("client got %q, want the full response", got)
	}
}
