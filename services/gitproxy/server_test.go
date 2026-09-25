// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAuth = "Basic dGVzdDp0ZXN0"

var (
	allowLoopback = policy.NewPolicy("test", policy.WithAllow("loopback", ""))
	blockLoopback = policy.NewPolicy("test", policy.WithBlock("loopback", ""))
)

func listen(t *testing.T) net.Listener {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

func serveConns(t *testing.T, handle func(net.Conn)) string {
	ln := listen(t)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				handle(conn)
			}()
		}
	}()
	return ln.Addr().String()
}

func startEcho(t *testing.T) string {
	return serveConns(t, func(conn net.Conn) { _, _ = io.Copy(conn, conn) })
}

func startProxy(t *testing.T, srv *server) string {
	proxySrv := httptest.NewServer(srv)
	t.Cleanup(proxySrv.Close)
	return proxySrv.Listener.Addr().String()
}

func viaProxy(u *url.URL) *policy.Policy {
	return policy.NewPolicy("test", policy.WithAllow("loopback", ""), policy.WithProxy(http.ProxyURL(u)))
}

func serve(srv *server, method, target, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("Proxy-Authorization", auth)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func connect(t *testing.T, proxyAddr, target, auth string) (net.Conn, *bufio.Reader, int) {
	conn, err := net.Dial("tcp", proxyAddr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: %s\r\n\r\n", target, target, auth)
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	defer resp.Body.Close()
	return conn, br, resp.StatusCode
}

func assertEcho(t *testing.T, conn net.Conn, br *bufio.Reader) {
	_, err := conn.Write([]byte("ping\n"))
	require.NoError(t, err)
	reply, err := br.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "ping\n", reply)
}

func startConnectOperator(t *testing.T, useTLS bool, reply string, seen chan<- *http.Request) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString(reply)
		_ = buf.Flush()
		_, _ = io.Copy(conn, buf)
	})
	operator := httptest.NewUnstartedServer(handler)
	if useTLS {
		operator.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
		operator.StartTLS()
	} else {
		operator.Start()
	}
	t.Cleanup(operator.Close)
	return operator
}

func startSOCKS5(t *testing.T) string {
	return serveConns(t, func(conn net.Conn) {
		br := bufio.NewReader(conn)
		read := func(n int) []byte {
			buf := make([]byte, n)
			_, _ = io.ReadFull(br, buf)
			return buf
		}
		_, _ = conn.Write([]byte{5, 2})
		_ = read(int(read(2)[1]))
		gotUser := string(read(int(read(2)[1])))
		if gotPass := string(read(int(read(1)[0]))); gotUser != "user" || gotPass != "secret" {
			_, _ = conn.Write([]byte{1, 1})
			return
		}
		_, _ = conn.Write([]byte{1, 0})
		_ = read(int(read(5)[4]) + 2)
		_, _ = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
		_, _ = io.Copy(conn, br)
	})
}

func TestUpstreamProxy(t *testing.T) {
	t.Parallel()
	var seen []string
	errStop := errors.New("stop")
	s := &server{policy: policy.NewPolicy("test", policy.WithProxy(func(r *http.Request) (*url.URL, error) {
		seen = append(seen, r.URL.Host)
		return nil, errStop
	}))}
	for _, target := range []string{"github.com:443", "github.com:8443", "[2001:db8::1]:443"} {
		_, err := s.dialUpstream(t.Context(), target)
		assert.ErrorIs(t, err, errStop)
	}
	assert.Equal(t, []string{"github.com", "github.com:8443", "[2001:db8::1]"}, seen)

	for _, host := range []string{"localhost", "git.localhost.", "127.0.0.2", "::1", "::ffff:127.0.0.1", "0.0.0.0", "::"} {
		u, err := s.upstreamProxy(&http.Request{URL: &url.URL{Host: net.JoinHostPort(host, "80")}})
		assert.NoError(t, err, host)
		assert.Nil(t, u, host)
	}
}

func TestWriteUpstreamError(t *testing.T) {
	t.Parallel()
	for err, want := range map[error]int{
		fmt.Errorf("dial: %w", policy.ErrDenied):                          http.StatusForbidden,
		context.DeadlineExceeded:                                          http.StatusGatewayTimeout,
		&net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}: http.StatusGatewayTimeout,
		errors.New("refused"):                                             http.StatusBadGateway,
	} {
		rec := httptest.NewRecorder()
		writeUpstreamError(rec, err)
		assert.Equal(t, want, rec.Code, err.Error())
	}
}

func TestRelay(t *testing.T) {
	t.Parallel()
	client, clientEnd := net.Pipe()
	upstreamEnd, upstream := net.Pipe()
	done := make(chan struct{})
	go func() {
		relay(clientEnd, upstreamEnd)
		close(done)
	}()
	require.NoError(t, upstream.Close())
	_, err := client.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	<-done
}

func TestProxyCONNECT(t *testing.T) {
	t.Parallel()
	echo := startEcho(t)
	srv := newServer(allowLoopback, testAuth, nil)

	assert.Equal(t, http.StatusProxyAuthRequired, serve(srv, http.MethodConnect, echo, "").Code)
	assert.Equal(t, http.StatusForbidden, serve(newServer(blockLoopback, "", nil), http.MethodConnect, echo, "").Code)
	assert.Equal(t, http.StatusBadRequest, serve(srv, http.MethodConnect, "127.0.0.1", testAuth).Code)

	conn, br, status := connect(t, startProxy(t, srv), echo, testAuth)
	require.Equal(t, http.StatusOK, status)
	assertEcho(t, conn, br)
}

func TestProxyCONNECTOperator(t *testing.T) {
	t.Parallel()
	const target = "git.example.com:443"

	t.Run("HTTP", func(t *testing.T) {
		t.Parallel()
		seen := make(chan *http.Request, 1)
		operator := startConnectOperator(t, false, "HTTP/1.1 204 No Content\r\n\r\nEARLY", seen)
		opURL, err := url.Parse(operator.URL)
		require.NoError(t, err)
		opURL.User = url.UserPassword("user", "secret")

		conn, br, status := connect(t, startProxy(t, newServer(viaProxy(opURL), "", nil)), target, "")
		require.Equal(t, http.StatusOK, status)
		early := make([]byte, 5)
		_, err = io.ReadFull(br, early)
		require.NoError(t, err)
		assert.Equal(t, "EARLY", string(early))
		assertEcho(t, conn, br)
		req := <-seen
		assert.Equal(t, target, req.Host)
		assert.Equal(t, basicAuth(opURL.User), req.Header.Get("Proxy-Authorization"))
	})

	t.Run("HTTPS", func(t *testing.T) {
		t.Parallel()
		operator := startConnectOperator(t, true, "HTTP/1.1 200 OK\r\n\r\n", make(chan *http.Request, 1))
		opURL, err := url.Parse(operator.URL)
		require.NoError(t, err)
		key, err := x509.MarshalPKCS8PrivateKey(operator.TLS.Certificates[0].PrivateKey)
		require.NoError(t, err)
		pemFile := filepath.Join(t.TempDir(), "proxy.pem")
		pemData := append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: operator.Certificate().Raw}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})...)
		require.NoError(t, os.WriteFile(pemFile, pemData, 0o600))
		proxyTLS, err := proxyTLSConfig(pemFile, pemFile, "")
		require.NoError(t, err)

		conn, br, status := connect(t, startProxy(t, newServer(viaProxy(opURL), "", proxyTLS)), target, "")
		require.Equal(t, http.StatusOK, status)
		assertEcho(t, conn, br)
	})

	t.Run("SOCKS5", func(t *testing.T) {
		t.Parallel()
		opURL := &url.URL{Scheme: "socks5", User: url.UserPassword("user", "secret"), Host: startSOCKS5(t)}
		conn, br, status := connect(t, startProxy(t, newServer(viaProxy(opURL), "", nil)), target, "")
		require.Equal(t, http.StatusOK, status)
		assertEcho(t, conn, br)
	})

	t.Run("NTLM", func(t *testing.T) {
		t.Parallel()
		challenge := make([]byte, 48)
		copy(challenge, "NTLMSSP\x00")
		challenge[8] = 2
		binary.LittleEndian.PutUint32(challenge[20:], 0x201)
		auths := make(chan string, 2)
		addr := serveConns(t, func(conn net.Conn) {
			br := bufio.NewReader(conn)
			for _, reply := range []string{"HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: NTLM " + base64.StdEncoding.EncodeToString(challenge) + "\r\nContent-Length: 4\r\n\r\ndeny", "HTTP/1.1 200 OK\r\n\r\n"} {
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				auths <- req.Header.Get("Proxy-Authorization")
				_, _ = io.WriteString(conn, reply)
			}
		})
		srv := newServer(viaProxy(&url.URL{Scheme: "http", User: url.UserPassword(`CORP\alice`, "secret"), Host: addr}), "", nil)
		srv.proxyNTLM = true

		_, _, status := connect(t, startProxy(t, srv), target, "")
		require.Equal(t, http.StatusOK, status)
		assert.Regexp(t, "^NTLM TlRMTVNTUAAB", <-auths)
		assert.Regexp(t, "^NTLM TlRMTVNTUAAD", <-auths)
	})

	t.Run("Refused", func(t *testing.T) {
		t.Parallel()
		operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "blocked by policy", http.StatusForbidden)
		}))
		t.Cleanup(operator.Close)
		opURL, err := url.Parse(operator.URL)
		require.NoError(t, err)

		rec := serve(newServer(viaProxy(opURL), "", nil), http.MethodConnect, target, "")
		assert.Equal(t, http.StatusBadGateway, rec.Code)
		assert.Contains(t, rec.Body.String(), "blocked by policy")
	})

	t.Run("ClientGone", func(t *testing.T) {
		t.Parallel()
		ln := listen(t)
		operatorClosed := make(chan struct{})
		requestSeen := make(chan struct{})
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer close(operatorClosed)
			defer conn.Close()
			if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
				return
			}
			close(requestSeen)
			_, _ = io.Copy(io.Discard, conn)
		}()

		conn, err := net.Dial("tcp", startProxy(t, newServer(viaProxy(&url.URL{Scheme: "http", Host: ln.Addr().String()}), "", nil)))
		require.NoError(t, err)
		_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target)
		require.NoError(t, err)
		responded := make(chan struct{})
		go func() {
			_, _ = conn.Read(make([]byte, 1))
			close(responded)
		}()
		select {
		case <-requestSeen:
		case <-responded:
			t.Fatal("proxy answered before reaching the operator")
		}
		require.NoError(t, conn.Close())
		<-operatorClosed
	})
}

func TestProxyHTTP(t *testing.T) {
	t.Parallel()

	t.Run("Forward", func(t *testing.T) {
		t.Parallel()
		firstRead := make(chan struct{})
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Empty(t, r.Header.Get("X-Drop-Me"))
			assert.False(t, r.Close)
			w.Header().Set("Connection", "X-Origin-Drop")
			w.Header().Set("X-Origin-Drop", "dropped")
			_, _ = w.Write([]byte("0008NAK\n"))
			http.NewResponseController(w).Flush()
			<-firstRead
			_, _ = w.Write([]byte("0000"))
		}))
		t.Cleanup(origin.Close)

		req, err := http.NewRequest(http.MethodGet, origin.URL+"/git-upload-pack", nil)
		require.NoError(t, err)
		req.Header.Set("Connection", "close, X-Drop-Me")
		req.Header.Set("X-Drop-Me", "dropped")
		proxyURL := &url.URL{Scheme: "http", Host: startProxy(t, newServer(allowLoopback, "", nil))}
		resp, err := (&http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}).Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Empty(t, resp.Header.Get("X-Origin-Drop"))

		buf := make([]byte, 8)
		_, err = io.ReadFull(resp.Body, buf)
		require.NoError(t, err)
		assert.Equal(t, "0008NAK\n", string(buf))
		close(firstRead)
		rest, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "0000", string(rest))
	})

	t.Run("ViaOperator", func(t *testing.T) {
		t.Parallel()
		operator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(r.URL.String()))
		}))
		t.Cleanup(operator.Close)
		opURL, err := url.Parse(operator.URL)
		require.NoError(t, err)

		rec := serve(newServer(viaProxy(opURL), "", nil), http.MethodGet, "http://git.example.com/repo.git", "")
		assert.Equal(t, "http://git.example.com/repo.git", rec.Body.String())
	})

	t.Run("Denied", func(t *testing.T) {
		t.Parallel()
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("the blocked origin must not be reached")
		}))
		t.Cleanup(origin.Close)
		assert.Equal(t, http.StatusForbidden, serve(newServer(blockLoopback, "", nil), http.MethodGet, origin.URL, "").Code)
	})

	t.Run("OriginForm", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, http.StatusBadRequest, serve(newServer(allowLoopback, "", nil), http.MethodGet, "/", "").Code)
	})
}

func TestMain(m *testing.M) {
	MaybeTunnel()
	m.Run()
}

func TestRun(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	exe, err := os.Executable()
	require.NoError(t, err)
	repos, err := filepath.Abs("../../modules/git/tests/repos")
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.Git.HomePath, base)()
	defer test.MockVariableValue(&setting.AppPath, exe)()
	defer test.MockVariableValue(&setting.Migrations.AllowedHostList, "127.0.0.1/32")()
	t.Cleanup(func() { gitcmd.SetExtraEnvs(nil) })
	require.NoError(t, Run(t.Context()))

	stdout, _, runErr := gitcmd.NewCommand("config", "--get", "http.proxy").RunStdString(t.Context())
	require.NoError(t, runErr)
	assert.Contains(t, stdout, "http://gitea:")

	_, port, err := net.SplitHostPort(serveConns(t, func(conn net.Conn) {
		daemon := exec.Command("git", "daemon", "--inetd", "--export-all", "--base-path="+repos)
		daemon.Stdin, daemon.Stdout = conn, conn
		_ = daemon.Run()
	}))
	require.NoError(t, err)
	require.NoError(t, gitcmd.NewCommand("clone", "-q", "--bare").AddDynamicArguments("git://127.0.0.1:"+port+"/repo1_bare", filepath.Join(base, "allowed")).Run(t.Context()))
	_, stderr, runErr := gitcmd.NewCommand("clone", "-q", "--bare").AddDynamicArguments("git://127.0.0.2:"+port+"/repo1_bare", filepath.Join(base, "denied")).RunStdString(t.Context())
	require.Error(t, runErr)
	assert.Contains(t, stderr, "target denied by policy")
}
