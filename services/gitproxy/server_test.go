// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// testPolicy builds a policy that grants loopback (so tests can dial httptest
// backends) and chains the transport through the given operator proxy.
func testPolicy(t *testing.T, operatorProxy *url.URL) *policy.Policy {
	t.Helper()
	return policy.NewPolicy("test",
		policy.WithAllow("loopback", "test.ALLOWED"),
		policy.WithProxy(operatorProxy, testProxyFunc(operatorProxy)))
}

// newTestServer builds the proxy server for the given operator proxy.
func newTestServer(t *testing.T, operatorProxy *url.URL) *http.Server {
	t.Helper()
	srv, err := newServer(serverConfig{policy: testPolicy(t, operatorProxy)})
	require.NoError(t, err)
	return srv
}

// testProxyFunc mirrors Run's fixed selector so the policy transport chains
// through the operator proxy.
func testProxyFunc(u *url.URL) func(*http.Request) (*url.URL, error) {
	if u == nil {
		return nil
	}
	return func(*http.Request) (*url.URL, error) { return u, nil }
}

// startProxy serves srv on a random loopback port and returns its base URL. The
// server shuts down when ctx is cancelled.
func startProxy(t *testing.T, ctx context.Context, srv *http.Server) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go serve(ctx, ln, srv)
	return "http://" + ln.Addr().String()
}

// connectTunnel sends a CONNECT request to the proxy for the given target and
// returns the established connection (after the 200) for the caller to read.
func connectTunnel(t *testing.T, proxyURL, target string) net.Conn {
	t.Helper()
	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return conn
}

func TestGitProxyDirectDial(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-from-backend")
	}))
	t.Cleanup(backend.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	conn := connectTunnel(t, proxyURL, backend.Listener.Addr().String())
	// After the tunnel is established, send a plain HTTP request through it.
	_, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backend.Listener.Addr().String())
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello-from-backend", string(body))
}

func TestGitProxyPolicyDeny(t *testing.T) {
	reached := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	t.Cleanup(backend.Close)

	// loopback is on the block list, so the backend must never be reached
	pol := policy.NewPolicy("test", policy.WithBlock("loopback", "test.BLOCKED"))
	srv, err := newServer(serverConfig{policy: pol})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, srv)

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", backend.Listener.Addr().String(), backend.Listener.Addr().String())
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	_ = resp.Body.Close()
	assert.False(t, reached, "backend must not be reached when policy denies")
}

func TestGitProxyChainThroughOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "hello-through-chain")
	}))
	t.Cleanup(backend.Close)

	// Mock operator proxy: it records the CONNECT target and relays to the
	// backend. The target name is deliberately not dialled, so nothing needs DNS
	// for it — only the loopback backend does.
	chained := make(chan string, 1)
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		client, clientBuf, err := hj.Hijack()
		if err != nil {
			_ = targetConn.Close()
			return
		}
		_, _ = clientBuf.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = clientBuf.Flush()
		relay(&bufferedConn{Conn: client, r: clientBuf}, targetConn)
	}))
	t.Cleanup(operator.Close)

	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	// non-loopback target: a loopback one bypasses the operator by design
	target := "registry.example.com:443"
	conn := connectTunnel(t, proxyURL, target)
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello-through-chain", string(body))
	assert.Equal(t, target, <-chained, "the operator must have been asked to CONNECT the target")
}

func TestGitProxyNoPanicOnBadTarget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)

	// First: malformed target (no port).
	conn1, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	_, err = fmt.Fprintf(conn1, "CONNECT badhost HTTP/1.1\r\nHost: badhost\r\n\r\n")
	require.NoError(t, err)
	br1 := bufio.NewReader(conn1)
	resp1, err := http.ReadResponse(br1, nil)
	require.NoError(t, err)
	assert.True(t, resp1.StatusCode == http.StatusBadRequest || resp1.StatusCode == http.StatusBadGateway, "expected error status, got %d", resp1.StatusCode)
	_ = resp1.Body.Close()
	_ = conn1.Close()

	// Second: empty port must be rejected as malformed, never dialed or relayed.
	conn1b, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	_, err = fmt.Fprintf(conn1b, "CONNECT badhost: HTTP/1.1\r\nHost: badhost:\r\n\r\n")
	require.NoError(t, err)
	resp1b, err := http.ReadResponse(bufio.NewReader(conn1b), nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp1b.StatusCode)
	_ = resp1b.Body.Close()
	_ = conn1b.Close()

	// Third: unresolvable hostname.
	conn2, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	_, err = fmt.Fprintf(conn2, "CONNECT this-host-does-not-exist.invalid:443 HTTP/1.1\r\nHost: this-host-does-not-exist.invalid:443\r\n\r\n")
	require.NoError(t, err)
	br2 := bufio.NewReader(conn2)
	resp2, err := http.ReadResponse(br2, nil)
	require.NoError(t, err)
	assert.True(t, resp2.StatusCode >= 400, "expected error status for unresolvable host, got %d", resp2.StatusCode)
	_ = resp2.Body.Close()
	_ = conn2.Close()

	// Fourth: server must still serve a valid request. Use a loopback backend.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "still-alive")
	}))
	t.Cleanup(backend.Close)

	conn3 := connectTunnel(t, proxyURL, backend.Listener.Addr().String())
	_, err = fmt.Fprintf(conn3, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backend.Listener.Addr().String())
	require.NoError(t, err)
	br3 := bufio.NewReader(conn3)
	resp3, err := http.ReadResponse(br3, nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp3.Body)
	require.NoError(t, err)
	assert.Equal(t, "still-alive", string(body))
}

// TestSplitHostPortPort pins the port guard: only a dialable numeric port is
// accepted, so an empty, zero, out-of-range, or non-numeric port is a 400 rather
// than being relayed to an operator proxy verbatim.
func TestSplitHostPortPort(t *testing.T) {
	for _, target := range []string{"example.com:", "example.com:0", "example.com:65536", "example.com:abc", "example.com:-1"} {
		_, _, ok := splitHostPort(&http.Request{Host: target})
		assert.False(t, ok, target)
	}
	host, port, ok := splitHostPort(&http.Request{Host: "example.com:443"})
	require.True(t, ok)
	assert.Equal(t, "example.com", host)
	assert.Equal(t, "443", port)
}

// TestIsLoopbackHost pins the DNS-free predicate the chained CONNECT path uses to decide a
// target is local: literals, the RFC 6761 localhost/*.localhost names, and the trailing root dot.
func TestIsLoopbackHost(t *testing.T) {
	for _, host := range []string{
		"localhost", "LOCALHOST", "localhost.", "sub.localhost", "a.b.localhost.",
		"127.0.0.1", "127.5.6.7", "::1", "::ffff:127.0.0.1",
	} {
		assert.True(t, IsLoopbackHost(host), host)
	}
	for _, host := range []string{
		"", "notlocalhost", "localhost.evil.com", "127.0.0.2.example.com", "10.0.0.1", "example.com",
	} {
		assert.False(t, IsLoopbackHost(host), host)
	}
}

func TestRunIntegration(t *testing.T) {
	// only the bind address is read from settings; the policy is injected, fetched by Run
	t.Cleanup(test.MockVariableValue(&setting.Egress.GitProxyListenAddr, "127.0.0.1:0"))
	t.Cleanup(func() { egress.SetGitProxyURL("") })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, run(ctx, testPolicy(t, nil)))
	addr := egress.GitProxyURL()
	require.NotEmpty(t, addr)
	assert.Regexp(t, `^http://127\.0\.0\.1:\d+$`, addr)

	// Shut down the graceful goroutine so it doesn't leak into other tests.
	t.Cleanup(func() { graceful.GetManager().DoGracefulShutdown() })

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "init-works")
	}))
	t.Cleanup(backend.Close)

	conn := connectTunnel(t, addr, backend.Listener.Addr().String())
	_, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backend.Listener.Addr().String())
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "init-works", string(body))
}

func TestGitProxyForward(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "forward-ok")
	}))
	t.Cleanup(backend.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// absolute-URI request, as a proxy client sends it (Host is the origin)
	_, err = fmt.Fprintf(conn, "GET %s/ HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backend.URL, backend.Listener.Addr().String())
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "forward-ok", string(body))
}

func TestGitProxyLoopbackBypassesOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "direct-localhost")
	}))
	t.Cleanup(backend.Close)

	used := make(chan struct{}, 1)
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		used <- struct{}{}
		http.Error(w, "must not be used", http.StatusBadGateway)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	conn := connectTunnel(t, proxyURL, backend.Listener.Addr().String())
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", backend.Listener.Addr().String())
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "direct-localhost", string(body))
	select {
	case <-used:
		t.Fatal("a loopback target must not be chained to the operator")
	default:
	}
}

func TestGitProxyEarlyTunnelBytes(t *testing.T) {
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT", http.StatusMethodNotAllowed)
			return
		}
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
		// tunnel bytes written in the same segment as the 200 must survive
		_, _ = buf.WriteString("HTTP/1.1 200 Connection Established\r\n\r\nEARLYDATA")
		_ = buf.Flush()
		time.Sleep(100 * time.Millisecond)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	raw, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	_, err = fmt.Fprintf(raw, "CONNECT registry.example.com:443 HTTP/1.1\r\nHost: registry.example.com:443\r\n\r\n")
	require.NoError(t, err)
	br := bufio.NewReader(raw)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	_ = raw.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len("EARLYDATA"))
	_, err = io.ReadFull(br, got)
	require.NoError(t, err)
	assert.Equal(t, "EARLYDATA", string(got))
}

func TestGitProxyOperatorAuth(t *testing.T) {
	auth := make(chan string, 1)
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth <- r.Header.Get("Proxy-Authorization")
		http.Error(w, "refused", http.StatusForbidden)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)
	opURL.User = url.UserPassword("user", "pass")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	raw, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	_, err = fmt.Fprintf(raw, "CONNECT registry.example.com:443 HTTP/1.1\r\nHost: registry.example.com:443\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(raw), nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode, "the operator refusal surfaces as a bad gateway")

	select {
	case got := <-auth:
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		assert.Equal(t, want, got)
	default:
		t.Fatal("the operator never received the CONNECT")
	}
}

func TestGitProxyRejectsOriginForm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// origin-form request: a proxy client must send an absolute URI
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", u.Host)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGitProxyStripsHopHeaders(t *testing.T) {
	seen := make(chan http.Header, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Clone()
		w.Header().Set("Proxy-Authenticate", `Basic realm="backend"`)
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(backend.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	raw := "GET " + backend.URL + "/ HTTP/1.1\r\n" +
		"Host: " + backend.Listener.Addr().String() + "\r\n" +
		"Connection: keep-alive, X-Drop\r\n" +
		"X-Drop: 1\r\n" +
		"Proxy-Authorization: Basic Zm9v\r\n" +
		"\r\n"
	_, err = conn.Write([]byte(raw))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.Empty(t, resp.Header.Get("Proxy-Authenticate"), "hop-by-hop response header must not reach the client")

	got := <-seen
	assert.Empty(t, got.Get("X-Drop"), "Connection-named header must not reach the origin")
	assert.Empty(t, got.Get("Proxy-Authorization"), "hop-by-hop request header must not reach the origin")
}

// testCertPool trusts an httptest TLS server's certificate.
func testCertPool(srv *httptest.Server) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return pool
}

func TestGitProxyHTTPSOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "through-tls-operator")
	}))
	t.Cleanup(backend.Close)

	// TLS operator proxy: record the CONNECT target and relay to the backend.
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
		hj, ok := w.(http.Hijacker)
		if !ok {
			_ = targetConn.Close()
			http.Error(w, "hijack unsupported", http.StatusInternalServerError)
			return
		}
		client, clientBuf, err := hj.Hijack()
		if err != nil {
			_ = targetConn.Close()
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

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Run builds the operator TLS config from the system roots; the test trusts the
	// test cert instead
	srv, err := newServer(serverConfig{
		policy:      testPolicy(t, opURL),
		operatorTLS: &tls.Config{ServerName: opURL.Hostname(), RootCAs: testCertPool(operator)},
	})
	require.NoError(t, err)
	proxyURL := startProxy(t, ctx, srv)

	target := "registry.example.com:443"
	conn := connectTunnel(t, proxyURL, target)
	_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "through-tls-operator", string(body))
	assert.Equal(t, target, <-chained, "the operator must have been asked to CONNECT the target")
}

func TestGitProxyUnsupportedOperatorScheme(t *testing.T) {
	opURL, err := url.Parse("ftp://127.0.0.1:1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = fmt.Fprintf(conn, "CONNECT registry.example.com:443 HTTP/1.1\r\nHost: registry.example.com:443\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Contains(t, string(body), "unsupported operator proxy scheme")
}

// startSOCKS5 runs a minimal SOCKS5 server (RFC 1928, with the RFC 1929 user/pass
// sub-negotiation when the client offers it) that relays every CONNECT to
// backendAddr. Enough to exercise the chained SOCKS path.
func startSOCKS5(t *testing.T, backendAddr string) string {
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
			go serveSOCKS5(conn, backendAddr)
		}
	}()
	return ln.Addr().String()
}

func serveSOCKS5(conn net.Conn, backendAddr string) {
	defer conn.Close()
	br := bufio.NewReader(conn)

	greeting := make([]byte, 2)
	if _, err := io.ReadFull(br, greeting); err != nil {
		return
	}
	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(br, methods); err != nil {
		return
	}
	useAuth := false
	for _, m := range methods {
		if m == 0x02 {
			useAuth = true
		}
	}
	method := byte(0x00)
	if useAuth {
		method = 0x02
	}
	if _, err := conn.Write([]byte{0x05, method}); err != nil {
		return
	}
	if useAuth {
		head := make([]byte, 2)
		if _, err := io.ReadFull(br, head); err != nil {
			return
		}
		user := make([]byte, int(head[1]))
		if _, err := io.ReadFull(br, user); err != nil {
			return
		}
		plen, err := br.ReadByte()
		if err != nil {
			return
		}
		pass := make([]byte, int(plen))
		if _, err := io.ReadFull(br, pass); err != nil {
			return
		}
		if string(user) != "user" || string(pass) != "pass" {
			_, _ = conn.Write([]byte{0x01, 0x01})
			return
		}
		if _, err := conn.Write([]byte{0x01, 0x00}); err != nil {
			return
		}
	}

	// request: ver, cmd, rsv, atyp, addr, port
	req := make([]byte, 4)
	if _, err := io.ReadFull(br, req); err != nil {
		return
	}
	switch req[3] {
	case 0x01:
		if _, err := io.ReadFull(br, make([]byte, 4)); err != nil {
			return
		}
	case 0x03:
		l, err := br.ReadByte()
		if err != nil {
			return
		}
		if _, err := io.ReadFull(br, make([]byte, int(l))); err != nil {
			return
		}
	default:
		return
	}
	if _, err := io.ReadFull(br, make([]byte, 2)); err != nil {
		return
	}

	target, err := net.DialTimeout("tcp", backendAddr, 5*time.Second)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(target, br); done <- struct{}{} }()
	go func() { _, _ = io.Copy(conn, target); done <- struct{}{} }()
	<-done
}

func TestGitProxySocksOperator(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "through-socks")
	}))
	t.Cleanup(backend.Close)

	socksAddr := startSOCKS5(t, backend.Listener.Addr().String())

	for _, tc := range []struct {
		name, userinfo string
	}{
		{name: "no auth"},
		{name: "user and password", userinfo: "user:pass@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opURL, err := url.Parse("socks5://" + tc.userinfo + socksAddr)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

			target := "registry.example.com:443"
			conn := connectTunnel(t, proxyURL, target)
			_, err = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", target)
			require.NoError(t, err)
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			require.NoError(t, err)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "through-socks", string(body))
		})
	}
}

// TestGitProxyHalfClose checks that a client which half-closes after its request
// still receives the response: the relay must not tear the upstream down when the
// client's read direction ends first.
func TestGitProxyHalfClose(t *testing.T) {
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = backendLn.Close() })

	go func() {
		backendConn, err := backendLn.Accept()
		if err != nil {
			return
		}
		defer backendConn.Close()
		// answer only once the client has half-closed
		_, _ = io.Copy(io.Discard, backendConn)
		_, _ = backendConn.Write([]byte("late-reply"))
	}()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	target := backendLn.Addr().String()
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	_, err = conn.Write([]byte("x"))
	require.NoError(t, err)
	tcp, ok := conn.(*net.TCPConn)
	require.True(t, ok)
	require.NoError(t, tcp.CloseWrite())

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len("late-reply"))
	_, err = io.ReadFull(br, got)
	require.NoError(t, err)
	assert.Equal(t, "late-reply", string(got))
}

// TestGitProxyConnectEchoesVersion checks the tunnel's 200 line carries the
// client's HTTP version instead of a hardcoded 1.1.
func TestGitProxyConnectEchoesVersion(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(backend.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	target := backend.Listener.Addr().String()
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.0\r\n\r\n", target)
	require.NoError(t, err)

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "HTTP/1.0 200 Connection Established\r\n", line)
}

// TestGitProxyOperatorRefusalReason checks a refused upstream CONNECT surfaces
// the operator's explanation, not just its status.
func TestGitProxyOperatorRefusalReason(t *testing.T) {
	operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked by policy", http.StatusForbidden)
	}))
	t.Cleanup(operator.Close)
	opURL, err := url.Parse(operator.URL)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, opURL))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = fmt.Fprintf(conn, "CONNECT registry.example.com:443 HTTP/1.1\r\nHost: registry.example.com:443\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Contains(t, string(body), "blocked by policy")
}

// TestGitProxyForwardClearsClose checks the client's Connection: close stops at
// the proxy instead of being forwarded to the origin.
func TestGitProxyForwardClearsClose(t *testing.T) {
	originClose := make(chan bool, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originClose <- r.Close
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(backend.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	proxyURL := startProxy(t, ctx, newTestServer(t, nil))

	u, err := url.Parse(proxyURL)
	require.NoError(t, err)
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	raw := "GET " + backend.URL + "/ HTTP/1.1\r\n" +
		"Host: " + backend.Listener.Addr().String() + "\r\n" +
		"Connection: close\r\n" +
		"\r\n"
	_, err = conn.Write([]byte(raw))
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))
	assert.False(t, <-originClose, "the origin request must not inherit the client's close")
}
