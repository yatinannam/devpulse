package traffic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultShutdownTimeout is how long Shutdown waits for in-flight requests
// before it closes the connections that are still open.
const DefaultShutdownTimeout = 5 * time.Second

// readHeaderTimeout bounds how long a client may take to send its request
// headers, so a stalled connection cannot hold a server goroutine forever.
const readHeaderTimeout = 10 * time.Second

// CheckListen rejects listen addresses that are not host:port with a numeric
// port. net.Listen treats an empty address as ":0", which binds every
// interface on a random port, so it is refused here rather than started.
// The host may be empty (all interfaces) and the port may be 0 (any free
// port).
func CheckListen(addr string) error {
	const example = "127.0.0.1:9090"
	if addr == "" {
		return fmt.Errorf("invalid listen address %q: use host:port, for example %s", addr, example)
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return fmt.Errorf("invalid listen address %q: missing port, for example %s", addr, example)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid listen address %q: the port must be a number from 0 to 65535", addr)
	}
	return nil
}

// CheckNotSelf rejects a target that names the proxy's own listen address,
// which would make every request loop back into the proxy. It compares
// literally, without DNS: equal hosts, "localhost" against any loopback
// address, and a wildcard listen host ("", 0.0.0.0, ::) against a loopback
// target. A listen or target value that does not parse is left to
// CheckListen and CheckTarget, and port 0 never matches. Loops this cannot
// see, such as a LAN address of this machine, are still answered with 508
// by the proxy.
func CheckNotSelf(listen, target string) error {
	listenHost, listenPort, err := net.SplitHostPort(listen)
	if err != nil {
		return nil
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil
	}
	targetPort := u.Port()
	if targetPort == "" {
		targetPort = "80"
		if u.Scheme == "https" {
			targetPort = "443"
		}
	}
	lp, err1 := strconv.Atoi(listenPort)
	tp, err2 := strconv.Atoi(targetPort)
	if err1 != nil || err2 != nil || lp == 0 || lp != tp || !sameHost(listenHost, u.Hostname()) {
		return nil
	}
	return fmt.Errorf("--target %s is the proxy's own --listen address %s, so every request would loop back into the proxy; point --target at your app", target, listen)
}

// sameHost reports whether a connection to targetHost could reach a
// listener bound to listenHost.
func sameHost(listenHost, targetHost string) bool {
	if strings.EqualFold(listenHost, targetHost) {
		return true
	}
	isLocalhost := func(h string) bool { return strings.EqualFold(h, "localhost") }
	loopback := func(h string) bool {
		ip := net.ParseIP(h)
		return isLocalhost(h) || (ip != nil && (ip.IsLoopback() || ip.IsUnspecified()))
	}
	if ip := net.ParseIP(listenHost); listenHost == "" || (ip != nil && ip.IsUnspecified()) {
		return loopback(targetHost)
	}
	if isLocalhost(listenHost) || isLocalhost(targetHost) {
		return loopback(listenHost) && loopback(targetHost)
	}
	l, t := net.ParseIP(listenHost), net.ParseIP(targetHost)
	return l != nil && t != nil && l.Equal(t)
}

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
