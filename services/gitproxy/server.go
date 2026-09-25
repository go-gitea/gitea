// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"

	"golang.org/x/net/proxy"
)

// server is a forward proxy for git's http(s) remotes that enforces an egress policy on its direct dials.
type server struct {
	auth         string // the Proxy-Authorization header git must send
	policy       *policy.Policy
	dial         func(ctx context.Context, network, addr string) (net.Conn, error)
	reverseProxy *httputil.ReverseProxy
	proxyRootCAs *x509.CertPool
}

func newServer(p *policy.Policy, auth string) *server {
	s := &server{auth: auth, policy: p, dial: p.NewDialContext()}
	transport := p.NewHTTPTransport()
	transport.Proxy = s.upstreamProxy
	s.reverseProxy = &httputil.ReverseProxy{
		Rewrite:       func(*httputil.ProxyRequest) {},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler:  func(w http.ResponseWriter, _ *http.Request, err error) { writeUpstreamError(w, err) },
	}
	return s
}

// Run routes git's http(s) remotes through a proxy on a random loopback port until ctx is done.
func Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("git proxy: %w", err)
	}
	user := url.UserPassword("gitea", rand.Text())
	srv := &http.Server{Handler: newServer(egress.NewGitPolicy(), basicAuth(user)), ReadHeaderTimeout: 10 * time.Second}
	context.AfterFunc(ctx, func() { _ = srv.Close() })
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Error("git proxy: %v", err)
		}
	}()
	gitcmd.SetHTTPProxy((&url.URL{Scheme: "http", User: user, Host: ln.Addr().String()}).String())
	return nil
}

func basicAuth(user *url.Userinfo) string {
	password, _ := user.Password()
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user.Username()+":"+password))
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Proxy-Authorization")), []byte(s.auth)) != 1 {
		w.Header().Set("Proxy-Authenticate", `Basic realm="gitea"`)
		http.Error(w, "egress: proxy authentication required", http.StatusProxyAuthRequired)
		return
	}
	switch {
	case r.Method == http.MethodConnect:
		s.handleConnect(w, r)
	case r.URL.Scheme == "http" && r.URL.Host != "":
		s.reverseProxy.ServeHTTP(w, r)
	default:
		http.Error(w, "egress: CONNECT or an absolute http URI required", http.StatusBadRequest)
	}
}

func (s *server) handleConnect(w http.ResponseWriter, r *http.Request) {
	if host, _, err := net.SplitHostPort(r.URL.Host); err != nil || host == "" {
		http.Error(w, "egress: invalid CONNECT target", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	upstream, err := s.dialUpstream(ctx, r.URL.Host)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	client, buffered, err := http.NewResponseController(w).Hijack()
	if err != nil {
		_ = upstream.Close()
		http.Error(w, "egress: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		_ = client.Close()
		_ = upstream.Close()
		return
	}
	relay(withBuffered(client, buffered.Reader), upstream)
}

// upstreamProxy selects the operator's proxy for req, local targets are dialed directly as they would name the proxy's own host
func (s *server) upstreamProxy(req *http.Request) (proxyURL *url.URL, err error) {
	if !isLocalHost(req.URL.Hostname()) {
		proxyURL, err = s.policy.Proxy(req)
	}
	return proxyURL, err
}

func isLocalHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && (ip.Unmap().IsLoopback() || ip.IsUnspecified())
}

func (s *server) dialUpstream(ctx context.Context, target string) (net.Conn, error) {
	proxyURL, err := s.upstreamProxy(&http.Request{URL: &url.URL{Scheme: "https", Host: strings.TrimSuffix(target, ":443")}})
	switch {
	case err != nil:
		return nil, err
	case proxyURL == nil:
		return s.dial(ctx, "tcp", target)
	case proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h":
		dialer, err := proxy.FromURL(proxyURL, dialFunc(s.dial))
		if err != nil {
			return nil, err
		}
		ctxDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, errors.New("egress: socks dialer lacks context support")
		}
		return ctxDialer.DialContext(ctx, "tcp", target)
	case proxyURL.Scheme == "http" || proxyURL.Scheme == "https":
		return s.connectVia(ctx, proxyURL, target)
	}
	return nil, fmt.Errorf("egress: unsupported proxy scheme %q", proxyURL.Scheme)
}

type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func (f dialFunc) Dial(network, addr string) (net.Conn, error) {
	return f(context.Background(), network, addr)
}

func (f dialFunc) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return f(ctx, network, addr)
}

func (s *server) connectVia(ctx context.Context, proxyURL *url.URL, target string) (_ net.Conn, err error) {
	conn, err := s.dial(ctx, "tcp", policy.ProxyDialAddr(proxyURL))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()
	if proxyURL.Scheme == "https" {
		conn = tls.Client(conn, &tls.Config{ServerName: proxyURL.Hostname(), RootCAs: s.proxyRootCAs})
	}
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: target}, Host: target, Header: http.Header{}}
	if proxyURL.User != nil {
		req.Header.Set("Proxy-Authorization", basicAuth(proxyURL.User))
	}
	if err := req.Write(conn); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("egress: proxy refused CONNECT: %s", strings.TrimSpace(resp.Status+" "+string(body)))
	}
	return withBuffered(conn, reader), nil
}

func writeUpstreamError(w http.ResponseWriter, err error) {
	var netErr net.Error
	switch {
	case errors.Is(err, policy.ErrDenied):
		http.Error(w, "egress: target denied by policy", http.StatusForbidden)
	case errors.As(err, &netErr) && netErr.Timeout():
		http.Error(w, "egress: upstream timeout: "+err.Error(), http.StatusGatewayTimeout)
	default:
		http.Error(w, "egress: upstream failed: "+err.Error(), http.StatusBadGateway)
	}
}

// bufferedConn reads the bytes a handshake left buffered before the rest of the connection
type bufferedConn struct {
	net.Conn
	r io.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func withBuffered(conn net.Conn, r *bufio.Reader) net.Conn {
	if r.Buffered() == 0 {
		return conn // a bare socket lets io.Copy splice
	}
	return &bufferedConn{Conn: conn, r: r}
}

// relay copies both ways until either side is done, git tunnels TLS which needs no half-close
func relay(client, upstream net.Conn) {
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go pipe(upstream, client)
	go pipe(client, upstream)
	<-done
	_ = client.Close()
	_ = upstream.Close()
}
