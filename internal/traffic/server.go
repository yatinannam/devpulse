package traffic

import (
	"context"
	"net/http"
)

func Start(proxy http.Handler, addr string) *http.Server {
	server := &http.Server{Addr: addr, Handler: proxy}
	go func() { _ = server.ListenAndServe() }()
	return server
}

func Shutdown(server *http.Server) error {
	return server.Shutdown(context.Background())
}
