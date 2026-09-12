package network

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// The resource transport keeps its Chrome TLS profile to the origin. This dialer
// only establishes the proxy tunnel. HTTPS proxy TLS uses system trust, and is
// independently cancellable; disabling origin verification never disables it.
type profileProxyDialer struct {
	address   *url.URL
	tlsConfig *tls.Config
}

func newProfileProxyDialer(raw string, config *tls.Config) (*profileProxyDialer, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid proxy address")
	}
	switch u.Scheme {
	case "http", "https", "socks5":
	default:
		return nil, fmt.Errorf("unsupported proxy scheme")
	}
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "https" {
			port = "443"
		}
		if u.Scheme == "socks5" {
			port = "1080"
		}
		u.Host = net.JoinHostPort(u.Hostname(), port)
	}
	return &profileProxyDialer{u, config}, nil
}
func (d *profileProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	if d.address.Scheme == "socks5" {
		var auth *proxy.Auth
		if d.address.User != nil {
			password, _ := d.address.User.Password()
			auth = &proxy.Auth{User: d.address.User.Username(), Password: password}
		}
		p, err := proxy.SOCKS5("tcp", d.address.Host, auth, dialer)
		if err != nil {
			return nil, err
		}
		return p.(proxy.ContextDialer).DialContext(ctx, network, address)
	}
	conn, err := dialer.DialContext(ctx, "tcp", d.address.Host)
	if err != nil {
		return nil, err
	}
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	success := false
	defer func() {
		stop()
		if !success {
			conn.Close()
		}
	}()
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}
	if d.address.Scheme == "https" {
		config := &tls.Config{MinVersion: tls.VersionTLS12}
		if d.tlsConfig != nil {
			config = d.tlsConfig.Clone()
		}
		config.ServerName = d.address.Hostname()
		config.NextProtos = []string{"http/1.1"}
		secure := tls.Client(conn, config)
		if err := secure.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		conn = secure
	}
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address, Header: make(http.Header)}
	if d.address.User != nil {
		password, _ := d.address.User.Password()
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(d.address.User.Username()+":"+password)))
	}
	if err := req.Write(conn); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("proxy CONNECT status %d", response.StatusCode)
	}
	if !stop() {
		return nil, ctx.Err()
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	success = true
	return &bufferedProxyConn{Conn: conn, reader: reader}, nil
}

type bufferedProxyConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedProxyConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

// Preserve actionable failure classes without copying credentials, upstream
// status text or arbitrary proxy response bodies into browser diagnostics.
func proxyRequestError(err error) error {
	var dns *net.DNSError
	var cert x509.UnknownAuthorityError
	var op *net.OpError
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.As(err, &dns):
		return fmt.Errorf("proxy transport: DNS lookup failed")
	case errors.As(err, &cert):
		return fmt.Errorf("proxy transport: certificate authority is untrusted")
	case strings.Contains(err.Error(), "status 407"), strings.Contains(err.Error(), "authentication"):
		return fmt.Errorf("proxy transport: authentication rejected")
	case errors.As(err, &op):
		if op.Timeout() {
			return fmt.Errorf("proxy transport: %s timed out", op.Op)
		}
		return fmt.Errorf("proxy transport: %s failed", op.Op)
	default:
		return fmt.Errorf("proxy transport: tunnel or upstream protocol failed")
	}
}
