package ports

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"
)

func NewProxy(port int) http.Handler {
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(port))}
	return httputil.NewSingleHostReverseProxy(target)
}

func NewProxyForHost(host string, port int) http.Handler {
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(port))}
	return httputil.NewSingleHostReverseProxy(target)
}

func Reachable(ctx context.Context, port int) bool {
	_, ok := ReachableHost(ctx, port)
	return ok
}

func ReachableHost(ctx context.Context, port int) (string, bool) {
	dialer := net.Dialer{Timeout: 250 * time.Millisecond}
	for _, host := range []string{"127.0.0.1", "::1"} {
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			continue
		}
		_ = conn.Close()
		return host, true
	}
	return "", false
}
