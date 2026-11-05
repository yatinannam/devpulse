package traffic

import (
	"net/http"
	"strings"
	"testing"
)

func TestStartReportsBindError(t *testing.T) {
	first, _, err := Start(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer Shutdown(first, 0)

	addr := first.Addr
	second, _, err := Start(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), addr)
	if err == nil {
		_ = Shutdown(second, 0)
		t.Fatal("expected bind error")
	}
	if !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("unexpected error: %v", err)
	}
}
