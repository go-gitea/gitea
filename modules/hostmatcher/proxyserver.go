// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

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
	"net/url"
	"strings"
	"sync"
	"time"

	"gitea.dev/modules/log"
)

// hopHeaders are the connection-scoped headers a proxy must not forward (RFC 9110 section 7.6.1).
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

// errDenied is the body returned to the caller when a target is refused. The reason is logged
// server-side instead of being returned: a caller that can read it learns how an internal name
// resolves, which is the very thing the allow/block lists exist to withhold.
const errDenied = "target refused by the allow/block list"

// ProxyServer is a loopback HTTP proxy that validates every target against the allow/block lists
// before connecting, including targets reached through an HTTP redirect.
//
// It exists for subprocesses we cannot instrument. Gitea's own HTTP clients get connect-time
// enforcement from NewDialContext, but `git` performs its own DNS resolution and follows HTTP
// redirects itself, so a URL that passes validation can still land on an internal address once the
// remote answers with a 30x (SSRF). Pointing the subprocess at this proxy with `http.proxy` puts
// every hop, initial request and redirect alike, back under the same allow/block policy: the
// forwarded request is dialled through NewDialContext, and a redirect is returned to the caller
// unfollowed so its target arrives as a fresh, separately validated request.
//
// The listener is bound to loopback only. It is not authenticated: it applies exactly the policy
// Gitea's own migration client applies, so a local process gains nothing by using it.
type ProxyServer struct {
	usage         string
	allowList     *HostMatchList
	blockList     *HostMatchList
	proxyFunc     func(*http.Request) (*url.URL, error)
	proxyURLFixed *url.URL

	listener  net.Listener
	server    *http.Server
	transport *http.Transport
}

// NewProxyServer starts a validating proxy on a random loopback port and serves it in the
// background. Close stops it. The arguments mirror NewHTTPTransport: proxyFunc selects the upstream
// proxy per request (e.g. proxy.Proxy()), proxyURLFixed is the upstream address the dialler must
// always permit, and tlsConfig may be nil.
//
// allowList and blockList must not both be nil: nil lists match nothing, which would make the proxy
// permit every target and silently turn the guard into a no-op.
func NewProxyServer(usage string, allowList, blockList *HostMatchList, proxyFunc func(*http.Request) (*url.URL, error), proxyURLFixed *url.URL, tlsConfig *tls.Config) (*ProxyServer, error) {
	if allowList.IsEmpty() && blockList.IsEmpty() {
		return nil, fmt.Errorf("%s proxy: refusing to start with no allow or block list, it would permit every target", usage)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("%s proxy: unable to listen on loopback: %w", usage, err)
	}

	ps := &ProxyServer{
		usage:         usage,
		allowList:     allowList,
		blockList:     blockList,
		proxyFunc:     proxyFunc,
		proxyURLFixed: proxyURLFixed,
		listener:      listener,
		transport:     NewHTTPTransport(usage, allowList, blockList, proxyFunc, proxyURLFixed, tlsConfig),
	}
	ps.server = &http.Server{
		Handler:           ps,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		_ = ps.server.Serve(listener)
	}()
	return ps, nil
}

// URL returns the proxy address to hand to a subprocess, e.g. `git -c http.proxy=<URL>`.
func (ps *ProxyServer) URL() string {
	return "http://" + ps.listener.Addr().String()
}

// Close stops the proxy and releases its listener and pooled connections.
func (ps *ProxyServer) Close() error {
	ps.transport.CloseIdleConnections()
	return ps.server.Close()
}

func (ps *ProxyServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		ps.handleConnect(w, req)
		return
	}
	ps.handleForward(w, req)
}

// deny refuses a target, logging why and telling the caller only that it was refused.
func (ps *ProxyServer) deny(w http.ResponseWriter, target string, err error) {
	log.Debug("%s proxy: refused %q: %v", ps.usage, target, err)
	http.Error(w, errDenied, http.StatusForbidden)
}

// handleForward relays a plain HTTP request. The response, redirect or not, is returned to the
// caller as-is: http.Transport.RoundTrip does not follow redirects, so the caller re-requests the
// new location through this proxy and that hop is validated in its own right.
func (ps *ProxyServer) handleForward(w http.ResponseWriter, req *http.Request) {
	if !req.URL.IsAbs() || req.URL.Host == "" {
		http.Error(w, ps.usage+" proxy: absolute request URI required", http.StatusBadRequest)
		return
	}

	// when an upstream proxy carries the request, it dials the target and NewDialContext never sees
	// it, so the target has to be checked here instead
	addr := targetAddr(req.URL)
	if ps.upstreamProxyFor(req.URL) != nil {
		if err := ps.checkTarget(req.Context(), addr); err != nil {
			ps.deny(w, addr, err)
			return
		}
	}

	outReq := req.Clone(req.Context())
	outReq.RequestURI = ""
	outReq.Close = false
	removeHopHeaders(outReq.Header)

	resp, err := ps.transport.RoundTrip(outReq)
	if err != nil {
		// the deny reason from NewDialContext's Control func surfaces here
		ps.deny(w, addr, err)
		return
	}
	defer resp.Body.Close()

	removeHopHeaders(resp.Header)
	dst := w.Header()
	for key, values := range resp.Header {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleConnect validates the CONNECT target and, if permitted, tunnels bytes to it. A caller that
// follows a redirect to a different host opens a new CONNECT, so each tunnel is validated
// separately.
func (ps *ProxyServer) handleConnect(w http.ResponseWriter, req *http.Request) {
	addr := req.Host
	if addr == "" {
		addr = req.URL.Host
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}

	targetConn, targetReader, err := ps.dialTarget(req, addr)
	if err != nil {
		ps.deny(w, addr, err)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		targetConn.Close()
		http.Error(w, ps.usage+" proxy: connection hijacking unsupported", http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		targetConn.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// whichever direction ends first tears down both, otherwise a caller that walks away leaves the
	// other copy parked on a remote that never sends EOF, holding two sockets and a goroutine
	closeBoth := sync.OnceFunc(func() {
		_ = clientConn.Close()
		_ = targetConn.Close()
	})
	defer closeBoth()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		return
	}

	go func() {
		defer closeBoth()
		// clientBuf may already hold bytes read past the CONNECT header, so copy from the reader
		_, _ = io.Copy(targetConn, clientBuf)
	}()
	_, _ = io.Copy(clientConn, targetReader)
}

// upstreamProxyFor asks proxyFunc which upstream proxy, if any, should carry a request to target.
func (ps *ProxyServer) upstreamProxyFor(target *url.URL) *url.URL {
	if ps.proxyFunc == nil {
		return nil
	}
	probe := &http.Request{
		Method: http.MethodGet,
		URL:    target,
		Host:   target.Host,
		Header: http.Header{},
	}
	upstream, err := ps.proxyFunc(probe)
	if err != nil {
		return nil
	}
	return upstream
}

// dialTarget connects to addr, either directly or by asking the configured upstream proxy to open
// the tunnel. It returns the connection and the reader to consume it through, which is buffered
// when an upstream handshake has already read ahead.
func (ps *ProxyServer) dialTarget(req *http.Request, addr string) (net.Conn, io.Reader, error) {
	// the upstream selector expects a request URL, and https targets carry the default port here
	upstream := ps.upstreamProxyFor(&url.URL{Scheme: "https", Host: strings.TrimSuffix(addr, ":443")})
	if upstream == nil {
		dial := NewDialContext(ps.usage, ps.allowList, ps.blockList, ps.proxyURLFixed)
		conn, err := dial(req.Context(), "tcp", addr)
		if err != nil {
			return nil, nil, err
		}
		return conn, conn, nil
	}

	// the upstream dials the target itself, so it has to be checked before the tunnel is requested
	if err := ps.checkTarget(req.Context(), addr); err != nil {
		return nil, nil, err
	}

	// dialling the upstream proxy itself: NewDialContext permits the proxy host it is given, but
	// only on the proxy's own port, so hand it a URL with the port spelled out
	upstreamWithPort := *upstream
	upstreamWithPort.Host = upstreamAddr(upstream)
	dial := NewDialContext(ps.usage, ps.allowList, ps.blockList, &upstreamWithPort)
	conn, err := dial(req.Context(), "tcp", upstreamWithPort.Host)
	if err != nil {
		return nil, nil, err
	}
	reader, err := openUpstreamTunnel(conn, upstream, addr)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, reader, nil
}

// checkTarget applies the allow/block policy to addr without dialling it. The dialler enforces the
// same policy on the resolved IP for every direct connection; this covers the paths where an
// upstream proxy makes the outbound connection on our behalf.
func (ps *ProxyServer) checkTarget(ctx context.Context, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("%s can not resolve '%s': %w", ps.usage, host, err)
	}
	for _, ip := range ips {
		if ps.blockList.MatchHostOrIP(host, ip.IP) {
			return fmt.Errorf("%s can not call blocked HTTP servers (check your %s setting), deny '%s(%s)'", ps.usage, ps.blockList.SettingKeyHint, host, ip.IP)
		}
		if !ps.allowList.IsEmpty() && !ps.allowList.MatchHostOrIP(host, ip.IP) {
			return fmt.Errorf("%s can only call allowed HTTP servers (check your %s setting), deny '%s(%s)'", ps.usage, ps.allowList.SettingKeyHint, host, ip.IP)
		}
	}
	return nil
}

// openUpstreamTunnel performs the CONNECT handshake against an upstream proxy already dialled, and
// returns the reader to keep using: it may hold bytes read past the handshake response.
func openUpstreamTunnel(conn net.Conn, upstream *url.URL, addr string) (io.Reader, error) {
	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: http.Header{},
	}
	if user := upstream.User; user != nil {
		password, _ := user.Password()
		connectReq.Header.Set("Proxy-Authorization", "Basic "+basicAuth(user.Username(), password))
	}
	if err := connectReq.Write(conn); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, connectReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("upstream proxy refused CONNECT: " + resp.Status)
	}
	return reader, nil
}

// basicAuth encodes upstream proxy credentials for the Proxy-Authorization header.
func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// targetAddr renders a request URL as host:port, filling in the scheme default port.
func targetAddr(target *url.URL) string {
	if target.Port() != "" {
		return target.Host
	}
	if strings.EqualFold(target.Scheme, "https") {
		return net.JoinHostPort(target.Hostname(), "443")
	}
	return net.JoinHostPort(target.Hostname(), "80")
}

// upstreamAddr renders an upstream proxy URL as host:port, filling in the scheme default port.
func upstreamAddr(upstream *url.URL) string {
	return targetAddr(upstream)
}

// removeHopHeaders drops connection-scoped headers, including any the Connection header names.
func removeHopHeaders(header http.Header) {
	for _, value := range header.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}
