// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/log"
	"gitea.dev/modules/process"
	"gitea.dev/modules/setting"

	"golang.org/x/net/proxy"
)

const connectOK = "HTTP/1.1 200 Connection Established\r\nVia: 1.1 gitea-gitproxy\r\n\r\n"

// gitProxyServer is the in-process loopback HTTP proxy that sits between git
// subprocesses and their http(s) remotes. Egress enforcement (SSRF protection
// via the migration allow/deny lists) happens in the dialer's Control hook:
// every dialed ip:port is policy-checked at connect time. Targets chain through
// the operator proxy (setting.Proxy.ProxyURLFixed) when one is configured.
type gitProxyServer struct {
	// operatorProxy is the upstream proxy URL, or nil for direct dial.
	operatorProxy *url.URL
	// operatorDialAddr is operatorProxy's dial address with the scheme default
	// port applied, or "" for a direct dial.
	operatorDialAddr string
	// dial is the policy's own dialer: every connection is validated against the
	// resolved ip:port in the Control hook at connect time (the operator proxy's
	// host:port is exempted inside), so direct targets need no pre-check.
	dial func(ctx context.Context, network, addr string) (net.Conn, error)
	// operatorTLS is non-nil when the operator proxy speaks TLS (https scheme):
	// the chained CONNECT handshakes with the proxy before sending CONNECT.
	operatorTLS *tls.Config
	// transport serves absolute-URI (http remote) requests: the policy's dialer,
	// the loopback diversion and the operator-proxy chain are already wired in.
	transport *http.Transport
	// socksDialer tunnels to a target through a SOCKS5 operator proxy (nil unless
	// one is configured). It is built once when the server is constructed: the
	// proxy address, credentials and forward dialer are all fixed.
	socksDialer proxy.ContextDialer
}

// serverConfig is everything newServer needs, already loaded, so tests can build it
// directly without touching settings.
type serverConfig struct {
	policy *policy.Policy
	// operatorTLS is non-nil when the operator proxy speaks TLS (https scheme), so the
	// chained CONNECT handshakes with the proxy before sending CONNECT.
	operatorTLS *tls.Config
}

// newServer builds the git proxy handler from cfg and wraps it in an http.Server. It does
// not listen, publish an address, or serve; Run does that.
func newServer(cfg serverConfig) (*gitProxyServer, error) {
	operatorProxy := cfg.policy.ProxyURL()
	dial := cfg.policy.NewDialContext()

	srv := &gitProxyServer{
		operatorProxy:    operatorProxy,
		operatorDialAddr: policy.ProxyDialAddr(operatorProxy),
		operatorTLS:      cfg.operatorTLS,
		dial:             dial,
		transport:        cfg.policy.NewHTTPTransport(),
	}

	socksDialer, err := newSocksDialer(operatorProxy, dial)
	if err != nil {
		return nil, err
	}
	srv.socksDialer = socksDialer

	return srv, nil
}

// Run binds the internal git proxy on setting.Egress.GitProxyListenAddr and registers its
// bound address via egress.SetGitProxyURL (activating git env injection). It blocks until
// the address is published, so git subprocesses spawned after it returns are proxied.
func Run(ctx context.Context) error {
	return run(ctx, egress.GetMigrationPolicy())
}

// run is Run's testable core: everything but the policy, which is fetched from settings by Run
// and injected here.
func run(ctx context.Context, policy *policy.Policy) error {
	cfg := serverConfig{policy: policy}
	if u := policy.ProxyURL(); u != nil && strings.EqualFold(u.Scheme, "https") {
		// verify the proxy like any other TLS peer, using its hostname for SNI and
		// certificate checks
		cfg.operatorTLS = &tls.Config{ServerName: u.Hostname()}
	}

	proxyHandler, err := newServer(cfg)
	if err != nil {
		return fmt.Errorf("egress: init git proxy: %w", err)
	}

	httpServer := &http.Server{
		Handler:           proxyHandler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		// WriteTimeout is intentionally 0 (unbounded) for long Git operations
	}
	started := make(chan error, 1)

	go func() {
		_, _, finished := process.GetManager().AddTypedContext(ctx, "Internal Git Proxy", process.SystemProcessType, true)
		defer finished()
		listen(httpServer, started)
	}()
	return <-started
}

func listen(server *http.Server, started chan<- error) {
	// capture the configured address up front: this goroutine outlives Run's
	// caller and must not re-read mutable global settings
	listenAddr := setting.Egress.GitProxyListenAddr
	gracefulServer := graceful.NewServer("tcp", listenAddr, "GitProxy")
	gracefulServer.OnShutdown = func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
	serve := func(ln net.Listener) error {
		// Captures the real port (even if configured with ":0") and injects into egress
		boundAddr := ln.Addr().String()
		egress.SetGitProxyURL("http://" + boundAddr)
		log.Info("egress: internal git proxy listening on %s", boundAddr)
		// Signal that the server is ready to accept connections
		started <- nil

		return server.Serve(ln)
	}

	err := gracefulServer.ListenAndServe(serve, false)
	if err != nil {
		select {
		case started <- err:
			// Sent successfully
		default:
			// Dropped because nobody is listening
		}
		select {
		case <-graceful.GetManager().IsShutdown():
			log.Error("Failed to start git proxy server: %v", err)
		default:
			// If not shutting down, a bind/startup failure is fatal to Gitea
			log.Fatal("Failed to start git proxy server: %v", err)
		}
	}

	log.Info("Git Proxy Listener: %s Closed", listenAddr)
}

// ServeHTTP dispatches CONNECT (https remotes) and absolute-URI GET/POST (http
// remotes). The deferred recover is the safety net: even if a bug in validation
// or relay panics, the handler returns an error response to git instead of
// killing the server goroutine.
func (s *gitProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodConnect:
		s.handleCONNECT(w, r)
	default:
		s.handleHTTP(w, r)
	}
}

// handleCONNECT tunnels a CONNECT request to the target. Git sends CONNECT
// host:port for https remotes.
//
// The direct leg is enforced by the dialer's Control hook, at connect time. When
// an operator proxy is configured it dials the destination instead, so it owns
// the target policy there and the target is passed through unchecked. The one
// target never handed over is a loopback literal: that would mean the operator's
// own localhost, not the requested one, so it is dialed directly and gated
// normally.
//
// Returns true if the connection was hijacked (the recover net must not touch
// the response writer afterwards).
func (s *gitProxyServer) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	host, port, err := splitHostPort(r)
	if err != nil {
		http.Error(w, "egress: "+err.Error(), http.StatusBadRequest)
		return
	}

	upstream, err := s.dialUpstream(r.Context(), host, port)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "egress: hijack unsupported", http.StatusInternalServerError)
		return
	}
	raw, clientBuf, err := hj.Hijack()
	if err != nil {
		_ = upstream.Close()
		http.Error(w, "egress: hijack failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// 200 establishes the tunnel; the hijack buffer may already hold bytes read
	// past the request, so it feeds the relay through the wrapper
	_, err = clientBuf.WriteString(connectOK)
	if err != nil {
		upstream.Close()
		raw.Close()
		return
	}
	err = clientBuf.Flush()
	if err != nil {
		upstream.Close()
		raw.Close()
		return
	}

	relay(&bufferedConn{Conn: raw, r: clientBuf}, upstream)
}

// dialUpstream opens the connection to a CONNECT target: through the operator
// proxy when one is configured, else directly. A loopback literal is never handed
// to the operator (that would be the operator's own localhost, not the requested
// one), so it is dialed directly and gated normally.
func (s *gitProxyServer) dialUpstream(ctx context.Context, host, port string) (net.Conn, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	if s.operatorProxy != nil && !IsLoopbackHost(host) {
		return s.upstreamTunnel(ctx, host, port)
	}
	return s.dial(ctx, "tcp", net.JoinHostPort(host, port))
}

// IsLoopbackHost reports whether host names this machine's loopback: "localhost" or any
// "*.localhost" name (RFC 6761), or an IP literal in 127.0.0.0/8 or ::1. A trailing root dot
// is ignored. It involves no resolution.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Unmap().IsLoopback()
}

// dialFunc adapts the policy dialer to x/net/proxy's Dialer (and ContextDialer),
// so a SOCKS handshake dials the proxy through the policy too.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f dialFunc) Dial(network, addr string) (net.Conn, error) {
	return f(context.Background(), network, addr)
}

func (f dialFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

// isSocksScheme reports whether scheme names a SOCKS5 operator proxy.
func isSocksScheme(scheme string) bool {
	scheme = strings.ToLower(scheme)
	return scheme == "socks5" || scheme == "socks5h"
}

// newSocksDialer builds the target dialer for a SOCKS5 operator proxy, or nil
// when proxyURL is nil or not a SOCKS5 URL. forward dials the proxy hop, so the
// policy's exemption for the proxy host:port applies. The target itself is not
// validated here: the operator dials it, and an operator proxy is assumed to
// screen the targets it is asked to reach.
func newSocksDialer(proxyURL *url.URL, forward func(context.Context, string, string) (net.Conn, error)) (proxy.ContextDialer, error) {
	if proxyURL == nil || !isSocksScheme(proxyURL.Scheme) {
		return nil, nil
	}
	dialer, err := proxy.SOCKS5("tcp", policy.ProxyDialAddr(proxyURL), socksAuth(proxyURL), dialFunc(forward))
	if err != nil {
		return nil, fmt.Errorf("egress: operator proxy: %w", err)
	}
	ctxDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("egress: operator proxy dialer does not support contexts")
	}
	return ctxDialer, nil
}

// socksAuth maps the operator URL's userinfo to SOCKS5 credentials.
func socksAuth(u *url.URL) *proxy.Auth {
	if u.User == nil {
		return nil
	}
	password, _ := u.User.Password()
	return &proxy.Auth{User: u.User.Username(), Password: password}
}

// upstreamTunnel returns a connection already tunnelled to host:port through the
// configured operator proxy. The target is not resolved or validated here: the
// operator dials it. The scheme picks the protocol, matching what the forward
// path's transport already supports.
func (s *gitProxyServer) upstreamTunnel(ctx context.Context, host, port string) (net.Conn, error) {
	target := net.JoinHostPort(host, port)
	switch strings.ToLower(s.operatorProxy.Scheme) {
	case "socks5", "socks5h":
		conn, err := s.socksDialer.DialContext(ctx, "tcp", target)
		if err != nil {
			return nil, fmt.Errorf("egress: dial %s through operator: %w", target, err)
		}
		return conn, nil
	case "http", "https":
		return s.upstreamCONNECT(ctx, host, port)
	default:
		return nil, fmt.Errorf("egress: unsupported operator proxy scheme %q", s.operatorProxy.Scheme)
	}
}

// maxUpstreamErrorBody bounds how much of a refused upstream CONNECT response is
// kept for the error message.
const maxUpstreamErrorBody = 512

// upstreamCONNECT opens a tunnel to host:port through the configured operator
// proxy. The dial to the proxy itself goes through the policy dialer, so its
// host:port exemption applies; the target is not resolved or validated here —
// the operator dials it. The returned connection reads any bytes the operator
// sent past its CONNECT response.
func (s *gitProxyServer) upstreamCONNECT(ctx context.Context, host, port string) (net.Conn, error) {
	uc, err := s.dial(ctx, "tcp", s.operatorDialAddr)
	if err != nil {
		return nil, fmt.Errorf("egress: dial operator proxy: %w", err)
	}

	// ensure the connection is closed if the context is canceled
	stop := context.AfterFunc(ctx, func() { _ = uc.Close() })
	defer stop()

	if s.operatorTLS != nil {
		tlsConn := tls.Client(uc, s.operatorTLS)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = uc.Close()
			return nil, fmt.Errorf("egress: TLS handshake with operator proxy: %w", err)
		}
		uc = tlsConn
	}

	target := net.JoinHostPort(host, port)
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: target},
		Host:   target,
		Header: http.Header{},
	}
	if user := s.operatorProxy.User; user != nil {
		password, _ := user.Password()
		req.Header.Set("Proxy-Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user.Username()+":"+password)))
	}
	if err := req.Write(uc); err != nil {
		_ = uc.Close()
		return nil, fmt.Errorf("egress: send CONNECT to operator: %w", err)
	}

	reader := bufio.NewReader(uc)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = uc.Close()
		return nil, fmt.Errorf("egress: read operator CONNECT response: %w", err)
	}

	// RFC 9110 §9.3.6: Any 2xx code confirms tunnel establishment
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// a refused CONNECT usually carries the reason in the body (a 407 challenge,
		// "blocked by policy"); keep a bounded slice so the caller sees why
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamErrorBody))
		_ = resp.Body.Close()
		_ = uc.Close()
		if reason := strings.TrimSpace(string(detail)); reason != "" {
			return nil, fmt.Errorf("egress: operator proxy refused: %s: %s", resp.Status, reason)
		}
		return nil, fmt.Errorf("egress: operator proxy refused: %s", resp.Status)
	}
	_ = resp.Body.Close()
	return &bufferedConn{Conn: uc, r: reader}, nil
}

// handleHTTP forwards an absolute-URI request (http remotes) to the target. The
// policy's transport does the work: it dials directly when no operator proxy
// applies (enforced by the Control hook at connect time) and chains through the
// operator proxy otherwise, where the operator dials and therefore polices the
// destination. A loopback target is never chained — the transport's guard drops
// it to a direct, gated dial.
func (s *gitProxyServer) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if !r.URL.IsAbs() || r.URL.Host == "" {
		http.Error(w, "egress: absolute request URI required", http.StatusBadRequest)
		return
	}
	if r.URL.Scheme != "http" && r.URL.Scheme != "https" {
		http.Error(w, "egress: unsupported request scheme", http.StatusBadRequest)
		return
	}

	req := r.Clone(r.Context())
	req.RequestURI = "" // server-bound request must not carry RequestURI
	req.Close = false   // the client's close applies to its hop, not the one to the origin
	req.Host = req.URL.Host

	removeHopHeaders(req.Header)
	req.Header.Add("Via", "1.1 gitea-gitproxy")

	resp, err := s.transport.RoundTrip(req)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	defer resp.Body.Close()

	removeHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.Header().Add("Via", "1.1 gitea-gitproxy")
	w.WriteHeader(resp.StatusCode)

	flusher, _ := w.(http.Flusher)
	// Streams and flushes after every chunk to prevent git waiting for replies.
	_, _ = io.Copy(flushWriter{w: w, f: flusher}, resp.Body)
}

// writeUpstreamError maps a dial or round-trip failure to a response: a policy
// denial from the Control hook is a 403, anything else a bad gateway.
func writeUpstreamError(w http.ResponseWriter, err error) {
	if errors.Is(err, policy.ErrDenied) {
		http.Error(w, "egress: target denied by policy", http.StatusForbidden)
		return
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		http.Error(w, "egress: upstream timeout: "+err.Error(), http.StatusGatewayTimeout)
		return
	}
	http.Error(w, "egress: upstream failed: "+err.Error(), http.StatusBadGateway)
}

// splitHostPort extracts host:port from a CONNECT request
func splitHostPort(r *http.Request) (host, port string, err error) {
	if r.URL == nil {
		return "", "", errors.New("egress: CONNECT request URL is nil")
	}
	host, port, err = net.SplitHostPort(r.URL.Host)
	if err != nil {
		return host, port, err
	}
	if host == "" {
		return "", "", errors.New("egress: empty host")
	}
	// RFC 9112 §9.3.6: userinfo is forbidden
	if strings.Contains(host, "@") {
		return "", "", errors.New("userinfo '@' is not permitted in CONNECT target")
	}

	// Reject unspecified destination addresses (0.0.0.0 / ::) outright
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil && ip.Unmap().IsUnspecified() {
		return "", "", errors.New("unspecified destination address is not routable")
	}

	if p, err := strconv.ParseUint(port, 10, 16); err != nil || p == 0 {
		return "", "", fmt.Errorf("invalid port %q: must be between 1 and 65535", port)
	}
	return host, port, nil
}

// bufferedConn presents a connection whose reads come from r, so bytes buffered
// past a handshake (the client's hijack buffer, or an operator CONNECT response)
// are not lost when the tunnel starts. Writes and closes go to the underlying
// connection.
type bufferedConn struct {
	net.Conn
	r io.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }
func (c *bufferedConn) Unwrap() net.Conn           { return c.Conn }

type writeCloser interface {
	CloseWrite() error
}

type unwrapper interface {
	Unwrap() net.Conn
}

// socketOf returns the connection beneath a buffered wrapper, so the socket's own
// capabilities decide how the relay shuts down.
func socketOf(c net.Conn) net.Conn {
	for {
		if u, ok := c.(unwrapper); ok {
			c = u.Unwrap()
			continue
		}
		return c
	}
}

// relay copies bytes in both directions until both halves close. It half-closes
// on a clean EOF so a response still in flight can drain, and falls back to
// closing both at once when either side cannot half-close or a copy fails, which
// is what unblocks a peer parked on a read. Each direction runs in its own
// goroutine wrapped in a recover guard so a panic in one half never escapes to
// the runtime.
func relay(client, upstream net.Conn) {
	// sync.OnceFunc guarantees sockets are closed exactly once,
	// either on an error or after both directions finish cleanly.
	closeBoth := sync.OnceFunc(func() {
		_ = client.Close()
		_ = upstream.Close()
	})

	done := make(chan struct{}, 1)
	pipe := func(dst, src net.Conn) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("egress: relay recovered: %v", rec)
			}
		}()

		// If there is an actual read/write failure, tear down both sockets immediately.
		if _, err := io.Copy(dst, src); err != nil {
			closeBoth()
			return
		}

		// Clean EOF: inform dst that no more data is incoming.
		// If dst supports CloseWrite (e.g. TCPConn or TLS 1.23+), half-close it.
		// If it doesn't, do not closeBoth; let the other direction continue draining.
		if hc, ok := socketOf(dst).(writeCloser); ok {
			_ = hc.CloseWrite()
		}
	}

	// 1. Run client -> upstream on a background goroutine
	go func() {
		pipe(upstream, client)
		done <- struct{}{}
	}()

	// 2. Run upstream -> client directly on the current handler goroutine
	pipe(client, upstream)

	// Wait for the background direction to finish
	<-done
	closeBoth()
}

// hopHeaders are the connection-scoped headers an intermediary must not forward
// (RFC 9110 section 7.6.1).
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Proxy-Connection",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// removeHopHeaders drops connection-scoped headers, including any the Connection
// header names.
func removeHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for name := range strings.SplitSeq(value, ",") {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				header.Del(trimmed)
			}
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}

// copyHeader copies src into dst.
func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 && fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}
