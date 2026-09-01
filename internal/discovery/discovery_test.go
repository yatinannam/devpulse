package discovery

import (
	"net/http"
	"testing"
)

func TestIdentify(t *testing.T) {
	cases := []struct {
		name, server, body, want string
	}{
		{"fastapi", "uvicorn", "", "Python / FastAPI"},
		{"next", "", "_next/static/chunks", "Node / Next.js"},
		{"vite", "", "<script src=\"/@vite/client\">", "Node / Vite"},
		{"spring", "tomcat", "", "Java / Spring"},
		{"generic", "", "", "HTTP service"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := make(http.Header)
			h.Set("Server", tc.server)
			if got := identify(h, tc.body); got != tc.want {
				t.Fatalf("identify()=%q want %q", got, tc.want)
			}
		})
	}
}
