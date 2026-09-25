// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// ErrDenied wraps a policy rejection, so callers can tell it from a network failure.
var ErrDenied = errors.New("denied by egress policy")

type Policy struct {
	usage string
	allow *HostMatchList
	block *HostMatchList
	// allowKey/blockKey name the settings the lists were read from; a rejection reports them so an
	// operator can find the setting to fix. They live here rather than on the lists, which are an
	// implementation detail of this phase and carry no setting provenance of their own.
	allowKey string
	blockKey string
	// proxy is the fixed operator proxy, exempted on the dial path; proxyHost/proxyPort
	// are its host and scheme-defaulted port, so the exemption matches the port the
	// transport actually dials.
	proxy     *url.URL
	proxyHost string
	proxyPort string
	// proxyPreScreen enables the proxy pre-screening. It does not protect against TOCTOU attacks
	// but it acts as a low effort pre-screening, helpful is the proxy does not screen the requests.
	proxyPreScreen bool
	// proxyFunc selects the proxy per request (http.Transport.Proxy; e.g. proxy.Proxy()).
	proxyFunc func(*http.Request) (*url.URL, error)
}

// Option configures a Policy built by NewPolicy.
type Option func(*Policy)

// WithAllow sets the allow list from the raw comma-separated entries of the setting named by key
// (e.g. "security.ALLOWED_HOST_LIST"), which a rejection reports. An empty list allows every target.
func WithAllow(hostList, key string) Option {
	return func(p *Policy) {
		p.allow = ParseHostMatchList(key, hostList)
		p.allowKey = key
	}
}

// WithBlock sets the block list from the raw comma-separated entries of the setting named by key,
// which a rejection reports. A target matching the list is always rejected.
func WithBlock(hostList, key string) Option {
	return func(p *Policy) {
		p.block = ParseHostMatchList(key, hostList)
		p.blockKey = key
	}
}

// WithProxy sets the fixed operator proxy exempted on the dial path and the per-request proxy
// selector (http.Transport.Proxy, e.g. proxy.Proxy()); either may be nil.
func WithProxy(fixed *url.URL, perRequest func(*http.Request) (*url.URL, error)) Option {
	return func(p *Policy) {
		p.proxy, p.proxyFunc = fixed, perRequest
		if fixed == nil {
			return
		}
		p.proxyHost = fixed.Hostname()
		p.proxyPort = fixed.Port()
		if p.proxyPort == "" {
			p.proxyPort = schemePort(fixed.Scheme)
		}
	}
}

func WithProxyPreScreen(enabled bool) Option {
	return func(p *Policy) {
		p.proxyPreScreen = enabled
	}
}

// NewPolicy compiles a policy that enforces the given options on every outbound dial. usage names
// the caller in rejections ("webhook", "git-proxy"). Without WithAllow or WithBlock the
// corresponding list is empty.
func NewPolicy(usage string, opts ...Option) *Policy {
	p := &Policy{usage: usage}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ProxyURL returns the fixed operator proxy URL, or nil when none is configured.
func (p *Policy) ProxyURL() *url.URL { return p.proxy }

// schemePort mirrors the port net/http assumes for a proxy URL without one; httpproxy's
// portMap omits socks5h, which is never consulted for the proxy address.
func schemePort(scheme string) string {
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	case "socks5", "socks5h":
		return "1080"
	default:
		return ""
	}
}

// ProxyDialAddr returns u's dial address with the scheme default port applied, or "" for nil.
func ProxyDialAddr(u *url.URL) string {
	if u == nil {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = schemePort(u.Scheme)
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// settingHint renders the pointer a rejection carries, naming the setting the offending list was
// read from; empty when the policy was built without a key for that list.
func settingHint(key string) string {
	if key == "" {
		return ""
	}
	return fmt.Sprintf(" (check your %s setting)", key)
}

// checkTarget reports whether the target may be dialed, using the hostmatcher decision: the
// deny list rejects, then a non-empty allow list must match the host or the resolved IP. An
// empty allow list allows.
func (p *Policy) checkTarget(host string, ip netip.AddrPort) error {
	netIP := net.IP(ip.Addr().AsSlice())
	if classifyAddr(ip.Addr()) == classReserved {
		return fmt.Errorf("%s is in the mandatory reserved set", ip)
	}
	if p.block.MatchHostOrIP(host, netIP) {
		return fmt.Errorf("%s can not call blocked HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.blockKey), host, ip)
	}
	if !p.allow.IsEmpty() && !p.allow.MatchHostOrIP(host, netIP) {
		return fmt.Errorf("%s can only call allowed HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.allowKey), host, ip)
	}
	return nil
}

// CheckHost verifies that the host isn't known to be denied
// It cannot be used before external calls as it does not resolve the host or dial it, it only serves as lightweight prescreening
func (p *Policy) CheckHost(host string) error {
	ip := net.ParseIP(host)
	if ip != nil {
		if p.block.MatchIPAddr(ip) {
			return fmt.Errorf("%s can not call blocked HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.blockKey), host, ip)
		}
		if !p.allow.IsEmpty() && !p.allow.MatchIPAddr(ip) {
			return fmt.Errorf("%s can only call allowed HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.allowKey), host, ip)
		}
		return nil
	}
	if p.block.MatchHostName(host) {
		return fmt.Errorf("%s can not call blocked HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.blockKey), host, ip)
	}
	if !p.allow.IsEmpty() && !p.allow.MatchHostName(host) {
		return fmt.Errorf("%s can only call allowed HTTP servers%s, deny '%s(%s)'", p.usage, settingHint(p.allowKey), host, ip)
	}
	return nil
}

// CheckHostIPs reports whether host, resolved to ips, may be used. It applies the hostmatcher
// pre-check: the deny list rejects, and when the allow list is non-empty it must match the host
// or every resolved address.
func (p *Policy) CheckHostIPs(host string, ips []net.IP) error {
	ipAllowed := len(ips) > 0
	var ipBlocked bool
	for _, ip := range ips {
		ipAllowed = ipAllowed && p.allow.MatchIPAddr(ip)
		ipBlocked = ipBlocked || p.block.MatchIPAddr(ip)
	}
	if p.block.MatchHostName(host) || ipBlocked {
		return fmt.Errorf("%s can not call blocked HTTP servers%s, deny '%s'", p.usage, settingHint(p.blockKey), host)
	}
	if !p.allow.IsEmpty() && !p.allow.MatchHostName(host) && !ipAllowed {
		return fmt.Errorf("%s can only call allowed HTTP servers%s, deny '%s'", p.usage, settingHint(p.allowKey), host)
	}
	return nil
}

// AllowsHost reports whether host is on the allow list; an empty list allows every host. It
// answers for the name alone, for callers that must decide before the target is resolved (e.g. a
// proxy selector).
func (p *Policy) AllowsHost(host string) bool {
	return p.allow.IsEmpty() || p.allow.MatchHostName(host)
}

// NewDialContext returns a dial function that validates the already-resolved ip:port in the
// dialer's Control hook at connect time (no pre-resolution, no TOCTOU). The fixed operator
// proxy's host:port is exempt so the transport can dial it to CONNECT.
func (p *Policy) NewDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	// How Go HTTP Client works with redirection:
	//   transport.RoundTrip URL=http://domain.com, Host=domain.com
	//   transport.DialContext addrOrHost=domain.com:80
	//   dialer.Control tcp4:11.22.33.44:80
	//   transport.RoundTrip URL=http://www.domain.com/, Host=(empty here, in the direction, HTTP client doesn't fill the Host field)
	//   transport.DialContext addrOrHost=domain.com:80
	//   dialer.Control tcp4:11.22.33.44:80
	return func(ctx context.Context, network, addrOrHost string) (net.Conn, error) {
		dialer := net.Dialer{
			// default values comes from http.DefaultTransport
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,

			Control: func(network, ipAddr string, c syscall.RawConn) error {
				host, port, err := net.SplitHostPort(addrOrHost)
				if err != nil {
					return err
				}
				if p.proxyHost != "" {
					// Always allow the host of the proxy, but only on its configured port.
					if host == p.proxyHost && port == p.proxyPort {
						return nil
					}
				}

				// ipAddr is already the resolved ip:port
				addrPort, err := netip.ParseAddrPort(ipAddr)
				if err != nil {
					return fmt.Errorf("%s can only call HTTP servers via TCP, deny '%s(%s:%s)', err=%w", p.usage, host, network, ipAddr, err)
				}
				if err := p.checkTarget(host, addrPort); err != nil {
					return fmt.Errorf("%s: connection to %s: %w: %w", p.usage, host, ErrDenied, err)
				}
				return nil
			},
		}
		return dialer.DialContext(ctx, network, addrOrHost)
	}
}

// NewHTTPTransport returns a transport that enforces the policy on the direct-dial path via
// NewDialContext
func (p *Policy) NewHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 p.proxiedTargetGuard(p.proxyFunc),
		DialContext:           p.NewDialContext(),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// proxiedTargetGuard wraps the proxyFunc to apply the policy's pre-screening, when enabled.
func (p *Policy) proxiedTargetGuard(next func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	if next == nil {
		return nil
	}
	return func(req *http.Request) (*url.URL, error) {
		u, err := next(req)
		if u == nil || err != nil { // if it errored out, we can skip guards
			return u, err
		}

		if p.proxyPreScreen {
			return u, p.CheckHost(req.Host)
		}
		return u, nil
	}
}
