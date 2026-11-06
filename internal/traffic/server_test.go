package traffic

import (
	"net/http"
	"strings"
	"testing"
)

func TestCheckListenRequiresHostPort(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{addr: "", want: `invalid listen address "": use host:port, for example 127.0.0.1:9090`},
		{addr: "9090", want: `invalid listen address "9090": missing port, for example 127.0.0.1:9090`},
		{addr: "abc", want: `invalid listen address "abc": missing port, for example 127.0.0.1:9090`},
		{addr: "127.0.0.1:", want: `invalid listen address "127.0.0.1:": missing port, for example 127.0.0.1:9090`},
		{addr: "127.0.0.1:abc", want: `invalid listen address "127.0.0.1:abc": the port must be a number from 0 to 65535`},
		{addr: "127.0.0.1:70000", want: `invalid listen address "127.0.0.1:70000": the port must be a number from 0 to 65535`},
	} {
		err := CheckListen(tc.addr)
		if err == nil {
			t.Errorf("CheckListen(%q) succeeded, want an error", tc.addr)
			continue
		}
		if err.Error() != tc.want {
			t.Errorf("CheckListen(%q) = %q, want %q", tc.addr, err, tc.want)
		}
	}
	for _, addr := range []string{"127.0.0.1:9090", "127.0.0.1:0", ":9090", "0.0.0.0:9090", "[::1]:9090", "localhost:9090"} {
		if err := CheckListen(addr); err != nil {
			t.Errorf("CheckListen(%q) = %v, want nil", addr, err)
		}
	}
}

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
