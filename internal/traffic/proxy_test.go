package traffic

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyRecordsRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()
	recorder := NewRecorder(nil)
	proxy, err := NewProxy(backend.URL, recorder)
	if err != nil { t.Fatal(err) }
	server := httptest.NewServer(Handler(proxy))
	defer server.Close()
	resp, err := http.Get(server.URL + "/api/users?active=true")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { t.Fatalf("got status %d, want 200", resp.StatusCode) }
	entries := recorder.Snapshot()
	if len(entries) != 1 { t.Fatalf("got %d recorded requests, want 1", len(entries)) }
	if entries[0].Method != http.MethodGet { t.Fatalf("got method %q, want GET", entries[0].Method) }
	if entries[0].Path != "/api/users?active=true" { t.Fatalf("got path %q", entries[0].Path) }
}
