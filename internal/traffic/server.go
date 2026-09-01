package traffic

import (
	"context"
	"fmt"
	"net"
	"net/http"
)

func Start(proxy http.Handler, addr string) (*http.Server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := &http.Server{Addr: listener.Addr().String(), Handler: proxy}
	go func() { _ = server.Serve(listener) }()
	return server, nil
}

func Shutdown(server *http.Server) error {
	if server == nil {
		return nil
	}
	return server.Shutdown(context.Background())
}

func Serve(proxy http.Handler, addr string) error {
	return http.ListenAndServe(addr, proxy)
}
