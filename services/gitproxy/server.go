// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitproxy

import (
	"bufio"
	"cmp"
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
	"os"
	"strings"
	"time"

	"gitea.dev/modules/egress"
	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"github.com/Azure/go-ntlmssp"
	"golang.org/x/net/proxy"
)

const (
	proxyURLEnv  = "GITEA_GIT_PROXY" // tells this binary it runs as git's GIT_PROXY_COMMAND
	directHeader = "X-Gitea-Direct"  // asks for a CONNECT tunnel that skips the operator's proxy, as git:// remotes never used one
)

// proxyDialer reaches the operator's proxies, which are configuration rather than user input
var proxyDialer = &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}

// server is a forward proxy for git's remotes that enforces an egress policy on its direct dials.
type server struct {
	auth         string // the Proxy-Authorization header git must send
	policy       *policy.Policy
	dial         func(ctx context.Context, network, addr string) (net.Conn, error)
	reverseProxy *httputil.ReverseProxy
	proxyTLS     *tls.Config
	proxyNTLM    bool // CONNECT only, the transport can't pin the connection NTLM authenticates
}

func newServer(p *policy.Policy, auth string, proxyTLS *tls.Config) *server {
	s := &server{auth: auth, policy: p, dial: p.NewDialContext(), proxyTLS: cmp.Or(proxyTLS, &tls.Config{})}
	transport := p.NewHTTPTransport()
	transport.Proxy = s.upstreamProxy
	transport.TLSClientConfig = s.proxyTLS.Clone() // the transport adds its ALPN protocols to the config it gets
	s.reverseProxy = &httputil.ReverseProxy{
		Rewrite:       func(*httputil.ProxyRequest) {},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler:  func(w http.ResponseWriter, _ *http.Request, err error) { writeUpstreamError(w, err) },
	}
	return s
}

// Run routes git's network remotes through a proxy on a random loopback port until ctx is done.
func Run(ctx context.Context) error {
	gitPolicy, err := egress.NewGitPolicy()
	if err != nil {
		return fmt.Errorf("git proxy: %w", err)
	}
	gitOption := func(key, env string) string { return cmp.Or(os.Getenv(env), setting.GitConfig.GetOption(key)) }
	proxyTLS, err := proxyTLSConfig(gitOption("http.proxySSLCAInfo", "GIT_PROXY_SSL_CAINFO"),
		gitOption("http.proxySSLCert", "GIT_PROXY_SSL_CERT"), gitOption("http.proxySSLKey", "GIT_PROXY_SSL_KEY"))
	if err != nil {
		return fmt.Errorf("git proxy: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("git proxy: %w", err)
	}
	user := url.UserPassword("gitea", rand.Text())
	handler := newServer(gitPolicy, basicAuth(user), proxyTLS)
	handler.proxyNTLM = strings.EqualFold(gitOption("http.proxyAuthMethod", "GIT_HTTP_PROXY_AUTHMETHOD"), "ntlm")
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	context.AfterFunc(ctx, func() { _ = srv.Close() })
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Error("git proxy: %v", err)
		}
	}()
	gitcmd.SetExtraEnvs(gitEnvs((&url.URL{Scheme: "http", User: user, Host: ln.Addr().String()}).String()))
	return nil
}

// gitEnvs route git's http(s) remotes through proxyURL and its git:// remotes through MaybeTunnel, command scope config beats every config file and keeps the credentials out of process listings
func gitEnvs(proxyURL string) []string {
	envs := []string{
		"GIT_CONFIG_PARAMETERS=" + strings.TrimSpace(os.Getenv("GIT_CONFIG_PARAMETERS")+" 'http.proxy="+proxyURL+"'"),
		"GIT_HTTP_PROXY_AUTHMETHOD=basic",
		"no_proxy=", "NO_PROXY=", // git honors no_proxy even for a configured proxy
	}
	if setting.GitConfig.GetOption("core.gitProxy") == "" { // the operator's own git:// proxy command stays in charge
		envs = append(envs, "GIT_PROXY_COMMAND="+setting.AppPath, proxyURLEnv+"="+proxyURL)
	}
	return envs
}

// MaybeTunnel serves as git's GIT_PROXY_COMMAND for git:// remotes when git runs this binary with host and port, it returns otherwise
func MaybeTunnel() {
	proxyURL := os.Getenv(proxyURLEnv)
	if proxyURL == "" || len(os.Args) != 3 {
		return
	}
	if err := tunnel(proxyURL, net.JoinHostPort(os.Args[1], os.Args[2])); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func tunnel(proxyURL, target string) error {
	gitProxy, err := url.Parse(proxyURL)
	if err != nil {
		return err
	}
	conn, err := net.Dial("tcp", gitProxy.Host)
	if err != nil {
		return err
	}
	if conn, err = openTunnel(context.Background(), conn, gitProxy, target, http.Header{directHeader: {"1"}}, false); err != nil {
		return err
	}
	go func() {
		_, _ = io.Copy(conn, os.Stdin)
		closeWrite(conn)
	}()
	_, err = io.Copy(os.Stdout, conn)
	return err
}

// proxyTLSConfig loads the files of git's http.proxySSL* options, like curl the CA file replaces the system roots
func proxyTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cfg := &tls.Config{}
	if caFile != "" {
		pemData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		cfg.RootCAs = x509.NewCertPool()
		if !cfg.RootCAs.AppendCertsFromPEM(pemData) {
			return nil, fmt.Errorf("no certificates in %s", caFile)
		}
	}
	if certFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, cmp.Or(keyFile, certFile))
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	return cfg, nil
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
	dial := s.dialUpstream
	if r.Header.Get(directHeader) != "" {
		dial = func(ctx context.Context, target string) (net.Conn, error) { return s.dial(ctx, "tcp", target) }
	}
	upstream, err := dial(ctx, r.URL.Host)
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
		dialer, err := proxy.FromURL(proxyURL, proxyDialer)
		if err != nil {
			return nil, err
		}
		ctxDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return nil, errors.New("egress: socks dialer lacks context support")
		}
		return ctxDialer.DialContext(ctx, "tcp", target)
	default:
		return s.connectVia(ctx, proxyURL, target)
	}
}

func (s *server) connectVia(ctx context.Context, proxyURL *url.URL, target string) (net.Conn, error) {
	conn, err := proxyDialer.DialContext(ctx, "tcp", policy.ProxyDialAddr(proxyURL))
	if err != nil {
		return nil, err
	}
	if proxyURL.Scheme == "https" {
		cfg := s.proxyTLS.Clone()
		cfg.ServerName = proxyURL.Hostname()
		conn = tls.Client(conn, cfg)
	}
	return openTunnel(ctx, conn, proxyURL, target, http.Header{}, s.proxyNTLM)
}

// openTunnel opens a CONNECT tunnel to target over conn to the proxy at proxyURL, closing conn on failure
func openTunnel(ctx context.Context, conn net.Conn, proxyURL *url.URL, target string, header http.Header, ntlm bool) (_ net.Conn, err error) {
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()
	switch {
	case ntlm:
		negotiate, _ := ntlmssp.NewNegotiateMessage("", "")
		header.Set("Proxy-Authorization", "NTLM "+base64.StdEncoding.EncodeToString(negotiate))
	case proxyURL.User != nil:
		header.Set("Proxy-Authorization", basicAuth(proxyURL.User))
	}
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: target}, Host: target, Header: header}
	reader := bufio.NewReader(conn)
	resp, err := roundTrip(conn, reader, req)
	if err != nil {
		return nil, err
	}
	if ntlm && resp.StatusCode == http.StatusProxyAuthRequired {
		_ = resp.Body.Close() // drains it for the next request on the connection
		authenticate, err := ntlmAuthenticate(resp.Header, proxyURL.User)
		if err != nil {
			return nil, err
		}
		header.Set("Proxy-Authorization", authenticate)
		if resp, err = roundTrip(conn, reader, req); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("egress: proxy refused CONNECT: %s", strings.TrimSpace(resp.Status+" "+string(body)))
	}
	return withBuffered(conn, reader), nil
}

func roundTrip(conn net.Conn, reader *bufio.Reader, req *http.Request) (*http.Response, error) {
	if err := req.Write(conn); err != nil {
		return nil, err
	}
	return http.ReadResponse(reader, req)
}

func ntlmAuthenticate(header http.Header, user *url.Userinfo) (string, error) {
	for _, value := range header.Values("Proxy-Authenticate") {
		if encoded, ok := strings.CutPrefix(value, "NTLM "); ok {
			challenge, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return "", err
			}
			password, _ := user.Password()
			authenticate, err := ntlmssp.NewAuthenticateMessage(challenge, user.Username(), password, nil)
			if err != nil {
				return "", err
			}
			return "NTLM " + base64.StdEncoding.EncodeToString(authenticate), nil
		}
	}
	return "", errors.New("egress: proxy sent no NTLM challenge")
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

func (c *bufferedConn) CloseWrite() error {
	closeWrite(c.Conn)
	return nil
}

func withBuffered(conn net.Conn, r *bufio.Reader) net.Conn {
	if r.Buffered() == 0 {
		return conn // a bare socket lets io.Copy splice
	}
	return &bufferedConn{Conn: conn, r: r}
}

// relay copies both ways and passes each end of stream on, the git:// protocol needs the half-close
func relay(client, upstream net.Conn) {
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(upstream, client)
		closeWrite(upstream)
		close(done)
	}()
	_, _ = io.Copy(client, upstream)
	closeWrite(client)
	<-done
	_ = client.Close()
	_ = upstream.Close()
}

func closeWrite(conn net.Conn) {
	if cw, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	} else {
		_ = conn.Close()
	}
}
