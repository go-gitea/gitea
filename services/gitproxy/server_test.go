// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/graceful"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Helpers
// ============================================================================

// testPolicy builds a policy that grants loopback (so tests can dial httptest
// backends) and chains the transport through the given operator proxy.
func testPolicy(t *testing.T, operatorProxy *url.URL) *policy.Policy {
	t.Helper()
	return policy.NewPolicy("test",
		policy.WithAllow("loopback", "test.ALLOWED"),
		policy.WithProxy(operatorProxy, testProxyFunc(operatorProxy)),
	)
}

// testBlockedPolicy builds a policy that denies every loopback target, so a
// request to an httptest backend is refused by the dialer's Control hook.
func testBlockedPolicy(t *testing.T) *policy.Policy {
	t.Helper()
	return policy.NewPolicy("test", policy.WithBlock("loopback", "test.BLOCKED"))
}

// testProxyFunc mirrors Run's fixed selector so the policy transport chains
// through the operator proxy.
func testProxyFunc(u *url.URL) func(*http.Request) (*url.URL, error) {
	if u == nil {
		return nil
	}
	return func(*http.Request) (*url.URL, error) { return u, nil }
}

// mustNewServer builds the proxy through newServer, so the tests exercise the
// real policy wiring (dialer, transport, operator address, SOCKS dialer).
func mustNewServer(t *testing.T, cfg serverConfig) *gitProxyServer {
	t.Helper()
	srv, err := newServer(cfg)
	require.NoError(t, err)
	return srv
}

// newTestProxy serves a proxy built from the given policy on a random loopback
// port.
func newTestProxy(t *testing.T, p *policy.Policy) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mustNewServer(t, serverConfig{policy: p}))
	t.Cleanup(srv.Close)
	return srv
}

// dialProxy connects to the proxy's listener and returns the raw connection
// with a buffered reader.
func dialProxy(t *testing.T, proxyURL string) (net.Conn, *bufio.Reader) {
	t.Helper()
	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn, bufio.NewReader(conn)
}

// connectTunnel sends a CONNECT request for target and requires a 200, then
// returns the established connection for the caller to read.
func connectTunnel(t *testing.T, proxyURL, target string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, br := dialProxy(t, proxyURL)
	_, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	require.NoError(t, err)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "CONNECT %s failed: %s", target, resp.Status)
	return conn, br
}

// connectRequest sends a CONNECT request for target and returns the proxy's
// response without asserting its status.
func connectRequest(t *testing.T, proxyURL, target string) *http.Response {
	t.Helper()
	conn, br := dialProxy(t, proxyURL)
	_, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	require.NoError(t, err)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	return resp
}

// testCertPool trusts the certificate of an httptest TLS server.
func testCertPool(srv *httptest.Server) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return pool
}

// startSOCKS5 runs a minimal SOCKS5 operator that tunnels every request to
// backendAddr, requiring user/password auth when they are non-empty. It
// returns its listen address.
func startSOCKS5(t *testing.T, backendAddr, user, pass string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveSOCKS5(conn, backendAddr, user, pass)
		}
	}()
	return ln.Addr().String()
}

func serveSOCKS5(conn net.Conn, backendAddr, user, pass string) {
	defer conn.Close()
	br := bufio.NewReader(conn)

	// greeting: version, method count, methods
	head := make([]byte, 2)
	if _, err := io.ReadFull(br, head); err != nil {
		return
	}
	methods := make([]byte, head[1])
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	wantAuth := user != ""
	offeredAuth := false
	for _, m := range methods {
		if m == 0x02 {
			offeredAuth = true
		}
	}
	if wantAuth && !offeredAuth {
		return
	}
	selected := byte(0x00)
	if wantAuth {
		selected = 0x02
	}
	if _, err := conn.Write([]byte{0x05, selected}); err != nil {
		return
	}
	if wantAuth {
		// RFC 1929: version, ulen, user, plen, pass
		ver := make([]byte, 2)
		if _, err := io.ReadFull(br, ver); err != nil {
			return
		}
		creds := make([]byte, int(ver[1])+1) // user plus the password length byte
		if _, err := io.ReadFull(br, creds); err != nil {
			return
		}
		ulen := int(ver[1])
		passBuf := make([]byte, int(creds[ulen]))
		if _, err := io.ReadFull(br, passBuf); err != nil {
			return
		}
		if string(creds[:ulen]) != user || string(passBuf) != pass {
			_, _ = conn.Write([]byte{0x01, 0x01}) // auth failure
			return
		}
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return
		}
	}

	// request: version, cmd, rsv, atyp, addr, port; the requested target is
	// ignored and everything is tunneled to backendAddr
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return
	}
	switch req[3] {
	case 0x01: // IPv4
		_, _ = io.ReadFull(br, make([]byte, 4))
	case 0x03: // domain
		l := make([]byte, 1)
		if _, err := io.ReadFull(br, l); err != nil {
			return
		}
		_, _ = io.ReadFull(br, make([]byte, int(l[0])))
	case 0x04: // IPv6
		_, _ = io.ReadFull(br, make([]byte, 16))
	default:
		return
	}
	_, _ = io.ReadFull(br, make([]byte, 2)) // port

	// success reply: version, ok, rsv, atyp IPv4, zero addr, zero port
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	backend, err := net.Dial("tcp", backendAddr)
	if err != nil {
		return
	}
	defer backend.Close()
	go func() { _, _ = io.Copy(backend, conn) }()
	_, _ = io.Copy(conn, backend)
}

// ============================================================================
// 1. Unit tests: parsing and header handling
// ============================================================================

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"localhost.", true},
		{"foo.localhost", true},
		{"sub.domain.localhost.", true},
		{"127.0.0.1", true},
		{"127.0.1.1", true},
		{"127.255.255.255", true},
		{"::1", true},
		{"::ffff:127.0.0.1", true},

		{"example.com", false},
		{"notlocalhost", false},
		{"localhost.com", false},
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"8.8.8.8", false},
		{"2001:db8::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			assert.Equal(t, tt.want, IsLoopbackHost(tt.host), "IsLoopbackHost(%q)", tt.host)
		})
	}
}

func TestRemoveHopHeaders(t *testing.T) {
	header := http.Header{}
	header.Add("Connection", "X-Custom-Remove, X-Another-Remove")
	header.Add("Connection", "X-Third-Remove") // multiple Connection headers
	header.Add("Keep-Alive", "timeout=5")
	header.Add("Proxy-Connection", "keep-alive")
	header.Add("X-Custom-Remove", "should-be-deleted")
	header.Add("X-Another-Remove", "should-be-deleted")
	header.Add("X-Third-Remove", "should-be-deleted")
	header.Add("X-Preserve-Me", "survivor")

	removeHopHeaders(header)

	for _, hop := range hopHeaders {
		assert.Empty(t, header.Get(hop), "hop header %q must be deleted", hop)
	}
	assert.Empty(t, header.Get("X-Custom-Remove"))
	assert.Empty(t, header.Get("X-Another-Remove"))
	assert.Empty(t, header.Get("X-Third-Remove"))
	assert.Equal(t, "survivor", header.Get("X-Preserve-Me"))
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name    string
		urlHost string
		wantErr string
		want    string
	}{
		{name: "host and port", urlHost: "example.com:443", want: "example.com:443"},
		{name: "ipv6 literal", urlHost: "[2001:db8::1]:443", want: "[2001:db8::1]:443"},
		{name: "missing port", urlHost: "example.com", wantErr: "missing port"},
		{name: "port zero", urlHost: "example.com:0", wantErr: "invalid port"},
		{name: "port out of range", urlHost: "example.com:65536", wantErr: "invalid port"},
		{name: "empty host", urlHost: ":443", wantErr: "empty host"},
		// net/http moves userinfo into URL.User, so a '@' in URL.Host cannot
		// arrive from a real server; the check guards direct handler use
		{name: "userinfo", urlHost: "user@example.com:443", wantErr: "userinfo"},
		{name: "unspecified ipv4", urlHost: "0.0.0.0:80", wantErr: "unspecified"},
		{name: "unspecified ipv6", urlHost: "[::]:80", wantErr: "unspecified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Method: http.MethodConnect, URL: &url.URL{Host: tt.urlHost}}
			host, port, err := splitHostPort(r)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, net.JoinHostPort(host, port))
		})
	}

	t.Run("nil URL", func(t *testing.T) {
		_, _, err := splitHostPort(&http.Request{Method: http.MethodConnect})
		require.ErrorContains(t, err, "URL is nil")
	})
}

// ============================================================================
// 2. Relay and half-close
// ============================================================================

// TestRelay_HalfClose verifies that when the client finishes writing and
// half-closes its write side, the relay does not tear the upstream connection
// down before the upstream finishes transmitting its response.
func TestRelay_HalfClose(t *testing.T) {
	lnUpstream, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lnUpstream.Close() })

	const payloadSize = 64 * 1024
	expectedClientData := strings.Repeat("C", payloadSize)
	expectedUpstreamResponse := strings.Repeat("U", payloadSize)

	go func() {
		conn, err := lnUpstream.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		received, _ := io.ReadAll(conn)
		if string(received) != expectedClientData {
			t.Errorf("upstream did not receive expected client data")
			return
		}
		_, _ = conn.Write([]byte(expectedUpstreamResponse))
	}()

	upstreamConn, err := net.Dial("tcp", lnUpstream.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = upstreamConn.Close() })

	lnClient, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = lnClient.Close() })

	var clientServerConn net.Conn
	var acceptErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		clientServerConn, acceptErr = lnClient.Accept()
	}()

	clientConn, err := net.Dial("tcp", lnClient.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientConn.Close() })
	wg.Wait()
	require.NoError(t, acceptErr)
	t.Cleanup(func() { _ = clientServerConn.Close() })

	relayDone := make(chan struct{})
	go func() {
		relay(clientServerConn, upstreamConn)
		close(relayDone)
	}()

	_, err = io.WriteString(clientConn, expectedClientData)
	require.NoError(t, err)
	require.NoError(t, clientConn.(*net.TCPConn).CloseWrite())

	response, err := io.ReadAll(clientConn)
	require.NoError(t, err)
	assert.Equal(t, expectedUpstreamResponse, string(response))

	select {
	case <-relayDone:
	case <-time.After(3 * time.Second):
		t.Fatal("relay did not terminate cleanly within timeout")
	}
}

// mockGracefulWrapper simulates a net.Conn wrapper like graceful's wrappedConn.
// Note: the real graceful wrappedConn does not implement Unwrap, so in
// production socketOf stops there and the client leg cannot half-close; this
// mock validates the unwrap mechanism itself.
type mockGracefulWrapper struct {
	net.Conn
}

func (g *mockGracefulWrapper) Unwrap() net.Conn { return g.Conn }

func TestSocketOf_Unwrapping(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	done := make(chan struct{})
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			defer conn.Close()
		}
		<-done
	}()
	t.Cleanup(func() { close(done) })

	rawTCP, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = rawTCP.Close() })

	// the wrapper hides CloseWrite, but socketOf must unwrap down to the socket
	wrapped := &bufferedConn{Conn: &mockGracefulWrapper{Conn: rawTCP}, r: strings.NewReader("")}
	_, ok := any(wrapped).(writeCloser)
	require.False(t, ok, "the wrapper chain must hide CloseWrite from a direct assertion")

	_, ok = socketOf(wrapped).(writeCloser)
	require.True(t, ok, "socketOf must unwrap the chain down to the TCP socket")
}

// TestRelay_FullDuplexThroughWrappers simulates a git push through the wrapper
// chain: the client sends its pack, half-closes, and still receives the
// server's reply.
func TestRelay_FullDuplexThroughWrappers(t *testing.T) {
	serverLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverLn.Close() })

	serverReceivedPayload := make(chan []byte, 1)
	go func() {
		originConn, err := serverLn.Accept()
		if err != nil {
			return
		}
		defer originConn.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, originConn)
		serverReceivedPayload <- buf.Bytes()
		_, _ = originConn.Write([]byte("UNPACK_OK_REFS_UPDATED"))
	}()

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = proxyLn.Close() })

	clientRawConn, err := net.Dial("tcp", proxyLn.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientRawConn.Close() })

	proxyClientConn, err := proxyLn.Accept()
	require.NoError(t, err)
	t.Cleanup(func() { _ = proxyClientConn.Close() })

	proxyOriginConn, err := net.Dial("tcp", serverLn.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = proxyOriginConn.Close() })

	wrappedProxyClient := &bufferedConn{Conn: &mockGracefulWrapper{Conn: proxyClientConn}, r: proxyClientConn}
	wrappedProxyOrigin := &bufferedConn{Conn: &mockGracefulWrapper{Conn: proxyOriginConn}, r: proxyOriginConn}

	relayDone := make(chan struct{})
	go func() {
		relay(wrappedProxyClient, wrappedProxyOrigin)
		close(relayDone)
	}()

	clientTCP := clientRawConn.(*net.TCPConn)
	payload := bytes.Repeat([]byte("A"), 10*1024)
	_, err = clientTCP.Write(payload)
	require.NoError(t, err)
	require.NoError(t, clientTCP.CloseWrite())

	select {
	case received := <-serverReceivedPayload:
		assert.Len(t, received, len(payload))
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to receive payload")
	}

	clientResponse := make([]byte, len("UNPACK_OK_REFS_UPDATED"))
	_, err = io.ReadFull(clientTCP, clientResponse)
	require.NoError(t, err)
	assert.Equal(t, "UNPACK_OK_REFS_UPDATED", string(clientResponse))

	require.NoError(t, clientTCP.Close())

	select {
	case <-relayDone:
	case <-time.After(2 * time.Second):
		t.Fatal("relay failed to shut down cleanly")
	}
}

// ============================================================================
// 3. CONNECT: direct dial
// ============================================================================

func TestProxy_CONNECT_Success(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	go func() {
		for {
			c, err := backend.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()

	proxySrv := newTestProxy(t, testPolicy(t, nil))

	conn, br := connectTunnel(t, proxySrv.URL, backend.Addr().String())
	testMsg := "Hello through the CONNECT tunnel!\n"
	_, err = conn.Write([]byte(testMsg))
	require.NoError(t, err)
	reply, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, testMsg, reply)
}

func TestProxy_CONNECT_PolicyDenied(t *testing.T) {
	// a blocked policy denies the loopback target in the dialer's Control hook
	proxySrv := newTestProxy(t, testBlockedPolicy(t))

	resp := connectRequest(t, proxySrv.URL, "127.0.0.1:9")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestProxy_CONNECT_MalformedTargets(t *testing.T) {
	proxySrv := httptest.NewServer(&gitProxyServer{})
	t.Cleanup(proxySrv.Close)

	for _, target := range []string{
		"127.0.0.1",       // missing port
		"127.0.0.1:0",     // port zero
		"127.0.0.1:65536", // port out of range
		":443",            // empty host
	} {
		t.Run(target, func(t *testing.T) {
			resp := connectRequest(t, proxySrv.URL, target)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "target %q", target)
		})
	}
}

func TestProxy_CONNECT_UnspecifiedTargets(t *testing.T) {
	// neither the operator nor the direct dialer may be reached: the target is
	// rejected during validation
	directDialed := make(chan struct{}, 1)
	operatorDialed := make(chan struct{}, 1)
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operatorDialed <- struct{}{}
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	proxySrv := httptest.NewServer(&gitProxyServer{
		operatorProxy:    opURL,
		operatorDialAddr: opURL.Host,
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			directDialed <- struct{}{}
			return nil, errors.New("no dial in this test")
		},
	})
	t.Cleanup(proxySrv.Close)

	for _, target := range []string{"0.0.0.0:3000", "[::]:3000"} {
		resp := connectRequest(t, proxySrv.URL, target)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "target %q", target)
		select {
		case <-operatorDialed:
			t.Fatal("unspecified target reached the operator proxy")
		default:
		}
		select {
		case <-directDialed:
			t.Fatal("unspecified target reached the direct dialer")
		default:
		}
	}
}

func TestProxy_CONNECT_DialTimeoutIs504(t *testing.T) {
	proxySrv := httptest.NewServer(&gitProxyServer{
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
		},
	})
	t.Cleanup(proxySrv.Close)

	resp := connectRequest(t, proxySrv.URL, "10.0.0.1:443")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
}

func TestProxy_CONNECT_DialDeadlineInjected(t *testing.T) {
	// dialUpstream must bound a deadline-less request context so a hanging
	// target cannot hold the handler forever
	deadlineIn := make(chan time.Duration, 1)
	proxySrv := httptest.NewServer(&gitProxyServer{
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if d, ok := ctx.Deadline(); ok {
				deadlineIn <- time.Until(d)
			} else {
				deadlineIn <- 0
			}
			return nil, errors.New("stop")
		},
	})
	t.Cleanup(proxySrv.Close)

	resp := connectRequest(t, proxySrv.URL, "10.0.0.1:443")
	_ = resp.Body.Close()

	select {
	case d := <-deadlineIn:
		require.NotZero(t, d, "the dial context must carry a deadline")
		assert.InDelta(t, 30.0, d.Seconds(), 5.0, "the injected deadline must be about 30s")
	case <-time.After(2 * time.Second):
		t.Fatal("the dialer was never called")
	}
}

// TestProxy_CONNECT_EarlyTunnelBytes checks that bytes the operator sends
// past its CONNECT response in the same segment survive the tunnel setup.
func TestProxy_CONNECT_EarlyTunnelBytes(t *testing.T) {
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 200 Connection Established\r\n\r\nEARLYDATA")
		_ = buf.Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	conn, br := connectTunnel(t, proxySrv.URL, "registry.example.com:443")
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	got := make([]byte, len("EARLYDATA"))
	_, err = io.ReadFull(br, got)
	require.NoError(t, err)
	assert.Equal(t, "EARLYDATA", string(got))
}

// ============================================================================
// 4. CONNECT: operator proxy chaining
// ============================================================================

func TestProxy_CONNECT_ChainedOperatorProxy(t *testing.T) {
	var operatorReceivedAuth string
	var operatorReceivedTarget string
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operatorReceivedAuth = r.Header.Get("Proxy-Authorization")
		operatorReceivedTarget = r.Host
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buf.Flush()
		_, _ = io.Copy(conn, buf) // echo tunnel
	}))
	t.Cleanup(operator.Close)

	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)
	opURL.User = url.UserPassword("gituser", "secretpass")

	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	conn, br := connectTunnel(t, proxySrv.URL, "git.external.org:443")
	testMsg := "through the chained operator\n"
	_, err = conn.Write([]byte(testMsg))
	require.NoError(t, err)
	reply, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, testMsg, reply)

	assert.Equal(t, "git.external.org:443", operatorReceivedTarget, "the operator must receive the original target")
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("gituser:secretpass"))
	assert.Equal(t, expectedAuth, operatorReceivedAuth, "the operator must receive the configured credentials")
}

func TestProxy_CONNECT_LoopbackBypassesOperator(t *testing.T) {
	operatorDialed := make(chan struct{}, 1)
	directDialed := make(chan struct{}, 1)
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		operatorDialed <- struct{}{}
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	proxySrv := httptest.NewServer(&gitProxyServer{
		operatorProxy:    opURL,
		operatorDialAddr: opURL.Host,
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			directDialed <- struct{}{}
			return nil, errors.New("no dial in this test")
		},
	})
	t.Cleanup(proxySrv.Close)

	// a loopback literal would name the operator's own machine, not this one,
	// so it must be dialed directly and gated normally
	for _, target := range []string{"localhost:65534", "127.0.0.1:65534", "[::1]:65534"} {
		resp := connectRequest(t, proxySrv.URL, target)
		_ = resp.Body.Close()
		select {
		case <-operatorDialed:
			t.Fatalf("loopback target %q was relayed to the operator proxy", target)
		default:
		}
		select {
		case <-directDialed:
		default:
			t.Fatalf("loopback target %q did not reach the direct dialer", target)
		}
	}
}

func TestProxy_CONNECT_OperatorAny2xxAccepts(t *testing.T) {
	// RFC 9110 9.3.6: any 2xx confirms the tunnel, not only 200
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 204 No Content\r\n\r\n")
		_ = buf.Flush()
		_, _ = io.Copy(conn, buf) // echo tunnel
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	conn, br := connectTunnel(t, proxySrv.URL, "git.external.org:443")
	testMsg := "tunneled after a 204\n"
	_, err = conn.Write([]byte(testMsg))
	require.NoError(t, err)
	reply, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, testMsg, reply)
}

func TestProxy_CONNECT_OperatorRefusalReason(t *testing.T) {
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked by policy", http.StatusForbidden)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	resp := connectRequest(t, proxySrv.URL, "registry.example.com:443")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "blocked by policy", "the refusal reason must reach the client")
}

func TestProxy_CONNECT_HTTPSOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "through-tls-operator")
	}))
	t.Cleanup(backend.Close)

	chained := make(chan string, 1)
	operator := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT", http.StatusMethodNotAllowed)
			return
		}
		chained <- r.Host
		targetConn, err := net.DialTimeout("tcp", backend.Listener.Addr().String(), 5*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer targetConn.Close()
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		client, clientBuf, err := hj.Hijack()
		if err != nil {
			return
		}
		_, _ = clientBuf.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = clientBuf.Flush()
		relay(&bufferedConn{Conn: client, r: clientBuf}, targetConn)
	}))
	t.Cleanup(operator.Close)

	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)
	require.Equal(t, "https", opURL.Scheme)

	// run() builds the operator TLS config from the system roots; the test
	// trusts the operator's self-signed certificate instead
	proxySrv := httptest.NewServer(mustNewServer(t, serverConfig{
		policy:      testPolicy(t, opURL),
		operatorTLS: &tls.Config{ServerName: opURL.Hostname(), RootCAs: testCertPool(operator)},
	}))
	t.Cleanup(proxySrv.Close)

	target := "registry.example.com:443"
	conn, br := connectTunnel(t, proxySrv.URL, target)
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	require.NoError(t, err)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "through-tls-operator", string(body))
	assert.Equal(t, target, <-chained, "the operator must have been asked to CONNECT the target")
}

func TestProxy_CONNECT_SocksOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "through-socks")
	}))
	t.Cleanup(backend.Close)

	for _, tc := range []struct {
		name, user, pass string
	}{
		{name: "no auth"},
		{name: "user and password", user: "user", pass: "pass"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			socksAddr := startSOCKS5(t, backend.Listener.Addr().String(), tc.user, tc.pass)
			userinfo := ""
			if tc.user != "" {
				userinfo = tc.user + ":" + tc.pass + "@"
			}
			opURL, err := url.Parse("socks5://" + userinfo + socksAddr)
			require.NoError(t, err)

			proxySrv := newTestProxy(t, testPolicy(t, opURL))

			target := "registry.example.com:443"
			conn, br := connectTunnel(t, proxySrv.URL, target)
			_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
			require.NoError(t, err)
			resp, err := http.ReadResponse(br, nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "through-socks", string(body))
		})
	}
}

func TestProxy_CONNECT_UnsupportedOperatorScheme(t *testing.T) {
	opURL := &url.URL{Scheme: "ftp", Host: "operator.invalid:21"}
	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	resp := connectRequest(t, proxySrv.URL, "git.external.org:443")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "unsupported operator proxy scheme")
}

// TestProxy_CONNECT_OperatorConnClosedOnClientGone checks that the connection
// to the operator is dropped when the client disappears while the proxy waits
// for the operator's CONNECT response.
func TestProxy_CONNECT_OperatorConnClosedOnClientGone(t *testing.T) {
	requestSeen := make(chan struct{})
	operatorClosed := make(chan struct{})
	operatorLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = operatorLn.Close() })
	go func() {
		conn, err := operatorLn.Accept()
		if err != nil {
			return
		}
		defer close(operatorClosed)
		defer conn.Close()
		br := bufio.NewReader(conn)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			if line == "\r\n" {
				break
			}
		}
		close(requestSeen)
		// block reading: the proxy must close this conn when the client goes away
		buf := make([]byte, 1)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
		}
	}()

	opURL, err := url.Parse("http://" + operatorLn.Addr().String())
	require.NoError(t, err)
	proxySrv := newTestProxy(t, testPolicy(t, opURL))

	conn, _ := dialProxy(t, proxySrv.URL)
	_, err = fmt.Fprintf(conn, "CONNECT registry.example.com:443 HTTP/1.1\r\nHost: registry.example.com:443\r\n\r\n")
	require.NoError(t, err)

	select {
	case <-requestSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("the operator never received the CONNECT request")
	}
	require.NoError(t, conn.Close())

	select {
	case <-operatorClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("the operator connection was not closed after the client went away")
	}
}

// ============================================================================
// 5. Normal (absolute-URI) HTTP forwarding
// ============================================================================

func TestProxy_HTTP_ForwardingAndHopHeaders(t *testing.T) {
	var originSawVia any
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originSawVia = r.Header.Get("Via")
		if r.Header.Get("Proxy-Connection") != "" {
			t.Error("Proxy-Connection hop header leaked to the origin")
		}
		if r.Header.Get("X-Drop-Me") != "" {
			t.Error("custom connection header leaked to the origin")
		}
		w.Header().Set("Connection", "X-Origin-Drop")
		w.Header().Set("X-Origin-Drop", "sensitive-origin-data")
		w.Header().Set("X-Normal", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("origin-content"))
	}))
	t.Cleanup(origin.Close)

	proxySrv := newTestProxy(t, testPolicy(t, nil))

	req, err := http.NewRequest(http.MethodGet, origin.URL+"/repo.git", nil)
	require.NoError(t, err)
	req.Header.Set("Connection", "X-Drop-Me")
	req.Header.Set("X-Drop-Me", "dropped")
	req.Header.Set("Proxy-Connection", "keep-alive")

	proxyURL, err := url.Parse(proxySrv.URL)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "origin-content", string(body))
	assert.Empty(t, resp.Header.Get("X-Origin-Drop"), "hop-by-hop response header leaked to the client")
	assert.Equal(t, "ok", resp.Header.Get("X-Normal"))
	assert.Equal(t, "1.1 gitea-gitproxy", resp.Header.Get("Via"), "the proxy must add a Via header")
	assert.Equal(t, "1.1 gitea-gitproxy", originSawVia, "the origin must see the proxy's Via header")
}

func TestProxy_HTTP_StreamingFlush(t *testing.T) {
	// small conversational pkt-lines must not be buffered by the proxy, or git
	// negotiation deadlocks
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("origin does not support flushing")
			return
		}
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		_, _ = w.Write([]byte("0008NAK\n"))
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("0000"))
		flusher.Flush()
	}))
	t.Cleanup(origin.Close)

	proxySrv := newTestProxy(t, testPolicy(t, nil))

	proxyURL, err := url.Parse(proxySrv.URL)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(origin.URL + "/git-upload-pack")
	require.NoError(t, err)
	defer resp.Body.Close()

	buf := make([]byte, 8)
	_, err = io.ReadFull(resp.Body, buf)
	require.NoError(t, err)
	assert.Equal(t, "0008NAK\n", string(buf), "the first pkt-line stalled or is invalid")

	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "0000", string(rest))
}

func TestProxy_HTTP_RejectsOriginForm(t *testing.T) {
	proxySrv := newTestProxy(t, testPolicy(t, nil))

	u, err := url.Parse(proxySrv.URL)
	require.NoError(t, err)
	conn, br := dialProxy(t, proxySrv.URL)
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", u.Host)
	require.NoError(t, err)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a proxy client must send an absolute URI")
}

func TestProxy_HTTP_RejectsUnsupportedScheme(t *testing.T) {
	proxySrv := newTestProxy(t, testPolicy(t, nil))

	conn, br := dialProxy(t, proxySrv.URL)
	_, err := fmt.Fprintf(conn, "GET ftp://example.com/repo HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestProxy_HTTP_ForwardClearsClose(t *testing.T) {
	// the client's Connection: close applies to its hop only and must not be
	// inherited by the origin request
	originClose := make(chan bool, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originClose <- r.Close
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(origin.Close)

	proxySrv := newTestProxy(t, testPolicy(t, nil))

	conn, br := dialProxy(t, proxySrv.URL)
	raw := "GET " + origin.URL + "/ HTTP/1.1\r\n" +
		"Host: " + origin.Listener.Addr().String() + "\r\n" +
		"Connection: close\r\n" +
		"\r\n"
	_, err := conn.Write([]byte(raw))
	require.NoError(t, err)

	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.False(t, <-originClose, "the origin request must not inherit the client's close")
}

func TestProxy_HTTP_PolicyDenied(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the blocked policy must keep the origin unreachable")
	}))
	t.Cleanup(backend.Close)

	proxySrv := newTestProxy(t, testBlockedPolicy(t))

	proxyURL, err := url.Parse(proxySrv.URL)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get(backend.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestProxy_HTTP_UpstreamTimeoutIs504(t *testing.T) {
	proxySrv := httptest.NewServer(&gitProxyServer{
		transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
			},
		},
	})
	t.Cleanup(proxySrv.Close)

	proxyURL, err := url.Parse(proxySrv.URL)
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	resp, err := client.Get("http://10.0.0.1/repo.git")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode)
}

// ============================================================================
// 6. Lifecycle: run() and the graceful wiring
// ============================================================================

// TestRun_PublishesBeforeServingAndTunnels pins run()'s core contract: when
// it returns, the proxy address is already published via egress.SetGitProxyURL,
// so git subprocesses spawned afterwards are proxied. It then drives a CONNECT
// tunnel through the real graceful listener, where the hijacked connection is
// graceful's wrappedConn.
//
// This test must run before anything shuts the graceful manager down; it is
// first in the lifecycle section for that reason.
func TestRun_PublishesBeforeServingAndTunnels(t *testing.T) {
	t.Cleanup(test.MockVariableValue(&setting.Egress.GitProxyListenAddr, "127.0.0.1:0"))
	// registered before the connection cleanups so they run after them
	t.Cleanup(func() { egress.SetGitProxyURL("") })
	t.Cleanup(func() { graceful.GetManager().DoGracefulShutdown() })

	require.NoError(t, run(context.Background(), testPolicy(t, nil)))
	addr := egress.GitProxyURL()
	require.Regexp(t, `^http://127\.0\.0\.1:\d+$`, addr, "the proxy address must be published before run returns")

	// echo backend that replies after the client half-closes, like a git push
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	const response = "UNPACK_OK"
	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn) // read until the client's EOF
		_, _ = conn.Write([]byte(response))
	}()

	conn, br := connectTunnel(t, addr, backend.Addr().String())
	_, err = conn.Write(bytes.Repeat([]byte("A"), 1024))
	require.NoError(t, err)
	require.NoError(t, conn.(*net.TCPConn).CloseWrite())
	got, err := io.ReadAll(br)
	require.NoError(t, err)
	assert.Equal(t, response, string(got))
}

// TestRun_BindFailurePropagates checks that a bind failure is returned to
// run()'s caller instead of being swallowed by the serving goroutine.
func TestRun_BindFailurePropagates(t *testing.T) {
	// the manager singleton must already be shutting down: the listen
	// goroutine's failure path logs Fatal otherwise, killing the test binary
	graceful.GetManager().DoGracefulShutdown()

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = occupied.Close() })
	t.Cleanup(test.MockVariableValue(&setting.Egress.GitProxyListenAddr, occupied.Addr().String()))

	require.Error(t, run(context.Background(), testPolicy(t, nil)))
}
