// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const internalSecret = "internal-only-secret"

// newServerOn starts a test server bound to a specific loopback address so allow/block lists can
// tell "external" (127.0.0.1) and "internal" (127.0.0.2) targets apart.
func newServerOn(t *testing.T, addr string, handler http.Handler, useTLS bool) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp", addr+":0")
	require.NoError(t, err)
	srv := httptest.NewUnstartedServer(handler)
	require.NoError(t, srv.Listener.Close())
	srv.Listener = listener
	if useTLS {
		srv.StartTLS()
	} else {
		srv.Start()
	}
	t.Cleanup(srv.Close)
	return srv
}

// newTestProxy starts a ProxyServer that blocks 127.0.0.2 and returns a client routed through it.
func newTestProxy(t *testing.T) (*ProxyServer, *http.Client) {
	t.Helper()
	blockList := ParseHostMatchList("test.BLOCKED", "127.0.0.2")
	allowList := ParseHostMatchList("test.ALLOWED", "")
	ps, err := NewProxyServer("test", allowList, blockList, nil, nil, &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ps.Close() })

	proxyURL, err := url.Parse(ps.URL())
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	return ps, client
}

func TestProxyServerAllowsPermittedTarget(t *testing.T) {
	external := newServerOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}), false)
	_, client := newTestProxy(t)

	resp, err := client.Get(external.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", string(body))
}

func TestProxyServerBlocksForbiddenTarget(t *testing.T) {
	internal := newServerOn(t, "127.0.0.2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(internalSecret))
	}), false)
	_, client := newTestProxy(t)

	// negative control: the target is reachable, so a refusal can only come from the block list
	direct, err := http.Get(internal.URL)
	require.NoError(t, err)
	directBody, err := io.ReadAll(direct.Body)
	require.NoError(t, err)
	require.NoError(t, direct.Body.Close())
	require.Equal(t, internalSecret, string(directBody))

	resp, err := client.Get(internal.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.NotContains(t, string(body), internalSecret)
	// the reason is logged, not returned: it would tell the caller how an internal name resolves
	assert.Contains(t, string(body), errDenied)
	assert.NotContains(t, string(body), "127.0.0.2")
}

// TestProxyServerRevalidatesRedirectTarget is the SSRF case: the first hop is permitted, and the
// permitted host redirects to a blocked internal address. The redirect must not smuggle the caller
// past the allow/block lists.
func TestProxyServerRevalidatesRedirectTarget(t *testing.T) {
	internal := newServerOn(t, "127.0.0.2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(internalSecret))
	}), false)
	external := newServerOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}), false)
	_, client := newTestProxy(t)

	resp, err := client.Get(external.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.NotContains(t, string(body), internalSecret)
}

// TestProxyServerAllowsPermittedRedirect keeps the behaviour that broke the previous fix: a
// redirect to a permitted target still resolves, so remotes that answer 301 (e.g. GitLab pointing
// at the .git URL) keep working.
func TestProxyServerAllowsPermittedRedirect(t *testing.T) {
	var externalURL string
	external := newServerOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repo" {
			http.Redirect(w, r, externalURL+"/repo.git", http.StatusMovedPermanently)
			return
		}
		_, _ = w.Write([]byte("clone-me"))
	}), false)
	externalURL = external.URL
	_, client := newTestProxy(t)

	resp, err := client.Get(external.URL + "/repo")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "clone-me", string(body))
}

func TestProxyServerBlocksForbiddenConnectTarget(t *testing.T) {
	internal := newServerOn(t, "127.0.0.2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(internalSecret))
	}), true)
	_, client := newTestProxy(t)

	resp, err := client.Get(internal.URL)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.NotContains(t, string(body), internalSecret)
		return
	}
	assert.Contains(t, err.Error(), "Forbidden")
}

func TestProxyServerAllowsPermittedConnectTarget(t *testing.T) {
	external := newServerOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tunnelled"))
	}), true)
	_, client := newTestProxy(t)

	resp, err := client.Get(external.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "tunnelled", string(body))
}

func TestRemoveHopHeaders(t *testing.T) {
	header := http.Header{}
	header.Set("Connection", "X-Custom-Hop")
	header.Set("X-Custom-Hop", "drop-me")
	header.Set("Proxy-Authorization", "drop-me")
	header.Set("X-Keep", "keep-me")
	removeHopHeaders(header)
	assert.Empty(t, header.Get("Connection"))
	assert.Empty(t, header.Get("X-Custom-Hop"))
	assert.Empty(t, header.Get("Proxy-Authorization"))
	assert.Equal(t, "keep-me", header.Get("X-Keep"))
}

func TestUpstreamAddr(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"http://proxy.example.com", "proxy.example.com:80"},
		{"https://proxy.example.com", "proxy.example.com:443"},
		{"http://proxy.example.com:8080", "proxy.example.com:8080"},
	} {
		u, err := url.Parse(tc.raw)
		require.NoError(t, err)
		assert.Equal(t, tc.want, upstreamAddr(u), fmt.Sprintf("raw=%s", tc.raw))
	}
}

func TestNewProxyServerRejectsEmptyLists(t *testing.T) {
	// nil lists match nothing, so a proxy built from them would permit every target
	ps, err := NewProxyServer("test", nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Nil(t, ps)
	assert.Contains(t, err.Error(), "permit every target")
}

// newUpstreamProxy starts a minimal CONNECT-capable proxy and records the authorities it was asked
// to reach.
func newUpstreamProxy(t *testing.T) (*url.URL, *[]string) {
	t.Helper()
	var seen []string
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT", http.StatusNotImplemented)
			return
		}
		seen = append(seen, r.Host)
		target, err := net.Dial("tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer target.Close()
		clientConn, clientBuf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer clientConn.Close()
		_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() { _, _ = io.Copy(target, clientBuf) }()
		_, _ = io.Copy(clientConn, target)
	})}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })

	upstream, err := url.Parse("http://" + listener.Addr().String())
	require.NoError(t, err)
	return upstream, &seen
}

// TestProxyServerValidatesTargetBehindUpstreamProxy covers the path where an upstream proxy makes
// the outbound connection, so the dialler never sees the real target and the policy has to be
// applied before the tunnel is requested.
func TestProxyServerValidatesTargetBehindUpstreamProxy(t *testing.T) {
	internal := newServerOn(t, "127.0.0.2", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(internalSecret))
	}), true)
	upstream, seen := newUpstreamProxy(t)

	ps, err := NewProxyServer("test", ParseHostMatchList("test.ALLOWED", ""), ParseHostMatchList("test.BLOCKED", "127.0.0.2"),
		func(*http.Request) (*url.URL, error) { return upstream, nil }, upstream, &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ps.Close() })

	proxyURL, err := url.Parse(ps.URL())
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	resp, err := client.Get(internal.URL)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.NotContains(t, string(body), internalSecret)
	} else {
		assert.Contains(t, err.Error(), "Forbidden")
	}
	assert.Empty(t, *seen, "the upstream proxy must never be asked to reach a blocked target")
}

func TestProxyServerTunnelsPermittedTargetViaUpstreamProxy(t *testing.T) {
	external := newServerOn(t, "127.0.0.1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("via-upstream"))
	}), true)
	upstream, seen := newUpstreamProxy(t)

	ps, err := NewProxyServer("test", ParseHostMatchList("test.ALLOWED", ""), ParseHostMatchList("test.BLOCKED", "127.0.0.2"),
		func(*http.Request) (*url.URL, error) { return upstream, nil }, upstream, &tls.Config{InsecureSkipVerify: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = ps.Close() })

	proxyURL, err := url.Parse(ps.URL())
	require.NoError(t, err)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}

	resp, err := client.Get(external.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "via-upstream", string(body))
	assert.NotEmpty(t, *seen, "the permitted target must be reached through the configured upstream proxy")
}

// TestProxyServerReleasesAbandonedTunnels guards the leak: a caller that walks away must not leave
// the other copy direction parked on a remote that never sends EOF.
func TestProxyServerReleasesAbandonedTunnels(t *testing.T) {
	silent, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = silent.Close() })
	go func() {
		for {
			conn, err := silent.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	ps, _ := newTestProxy(t)
	before := runtime.NumGoroutine()
	for range 25 {
		conn, err := net.Dial("tcp", strings.TrimPrefix(ps.URL(), "http://"))
		require.NoError(t, err)
		_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", silent.Addr(), silent.Addr())
		require.NoError(t, err)
		_, _ = bufio.NewReader(conn).ReadString('\n')
		require.NoError(t, conn.Close())
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && runtime.NumGoroutine() > before+10 {
		time.Sleep(50 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), before+10, "abandoned tunnels leaked goroutines")
}
