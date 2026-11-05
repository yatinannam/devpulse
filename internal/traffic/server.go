package traffic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DefaultShutdownTimeout is how long Shutdown waits for in-flight requests
// before it closes the connections that are still open.
const DefaultShutdownTimeout = 5 * time.Second

// readHeaderTimeout bounds how long a client may take to send its request
// headers, so a stalled connection cannot hold a server goroutine forever.
const readHeaderTimeout = 10 * time.Second

// Start listens on addr and serves proxy in the background. The returned
// channel receives the error if Serve stops for any reason other than
// Shutdown or Close, and is closed once Serve has returned.
func Start(proxy http.Handler, addr string) (*http.Server, <-chan error, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := &http.Server{Addr: listener.Addr().String(), Handler: proxy, ReadHeaderTimeout: readHeaderTimeout}
	serveErr := make(chan error, 1)
	go func() {
		defer close(serveErr)
		if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	return server, serveErr, nil
}

// Shutdown stops accepting connections and waits up to timeout
// (DefaultShutdownTimeout when timeout <= 0) for in-flight requests to finish.
// Requests that are still running at the deadline, such as a long-lived event
// stream, have their connections closed, and Shutdown reports that it did so.
func Shutdown(server *http.Server, timeout time.Duration) error {
	if server == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = DefaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		if closeErr := server.Close(); closeErr != nil {
			return fmt.Errorf("close connections after %s shutdown deadline: %w", timeout, closeErr)
		}
		return fmt.Errorf("requests still running after %s were cut off", timeout)
	}
	return err
}

func Serve(proxy http.Handler, addr string) error {
	server := &http.Server{Addr: addr, Handler: proxy, ReadHeaderTimeout: readHeaderTimeout}
	return server.ListenAndServe()
}
