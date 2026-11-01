// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"sync"
	"syscall"
	"time"

	"gitea.dev/modules/log"
)

type Mode uint8

const (
	// Lax mode allows public and unresolved targets by default, restricted ones need an allow entry
	Lax Mode = iota
	// Strict mode requires every target to match an allow entry
	Strict
)

// ErrDenied wraps a policy rejection, so callers can tell it from a network failure.
var ErrDenied = errors.New("denied by egress policy")

type Policy struct {
	usage              string
	mode               Mode
	allow              AllowList
	block              BlockList
	allowKey, blockKey string // the settings the lists were read from, named in rejections
	internalRanges     []netip.Prefix
	internalRangesKey  string // the setting the ranges were read from, named in rejections
	proxyFunc          func(*http.Request) (*url.URL, error)
}

type Option func(*Policy)

// WithAllow sets the allow list and reference to key responsible for it
func WithAllow(hostList, key string) Option {
	return func(p *Policy) {
		p.allow, p.allowKey = *NewAllowList(hostList, p.mode), key
	}
}

// WithBlock sets the blocklist and reference to key responsible for it
func WithBlock(hostList, key string) Option {
	return func(p *Policy) {
		p.block, p.blockKey = *NewBlockList(hostList), key
	}
}

// WithInternalRanges sets the ranges hostname entries may resolve into for non-public targets.
// Invalid entries are reported at startup by the settings loader (checkInternalRanges).
func WithInternalRanges(hostList, key string) Option {
	return func(p *Policy) {
		p.internalRanges, _ = ParseInternalRanges(hostList)
		p.internalRangesKey = key
	}
}

// WithProxy sets the proxyfunc in the policy
func WithProxy(proxyFunc func(*http.Request) (*url.URL, error)) Option {
	return func(p *Policy) {
		p.proxyFunc = proxyFunc
	}
}

// NewPolicy compiles a policy enforced on every outbound dial, usage names the caller in rejections.
// The mode sets the policy posture and the default ports of portless allow entries.
func NewPolicy(usage string, mode Mode, opts ...Option) *Policy {
	p := &Policy{usage: usage, mode: mode} // the zero lists are valid and behave as empty ones
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// proxyPorts maps the proxy schemes net/http speaks to their default ports, it speaks HTTP to any other
var proxyPorts = map[string]string{"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080"}

// targetPorts maps target schemes to their default dial port, http and anything else dials 80
var targetPorts = map[string]uint16{"https": 443, "git": 9418}

// dialPort resolves the port a target URL is dialed on, empty port means the scheme default
func dialPort(u *url.URL) uint16 {
	if port, err := strconv.ParseUint(u.Port(), 10, 16); err == nil && port != 0 {
		return uint16(port)
	}
	return cmp.Or(targetPorts[u.Scheme], 80)
}

// ProxyDialAddr returns the address the transport dials for proxy URL u
func ProxyDialAddr(u *url.URL) string {
	return net.JoinHostPort(u.Hostname(), cmp.Or(u.Port(), proxyPorts[u.Scheme]))
}

func (p *Policy) notAllowedError(target string) error {
	return fmt.Errorf("%s can only call allowed HTTP servers (no allow entry matches, check your %s setting), deny '%s'", p.usage, p.allowKey, target)
}

func (p *Policy) checkAddr(host string, ip netip.AddrPort) error {
	ip = netip.AddrPortFrom(unmap(ip.Addr()), ip.Port())
	class, reserved := classifyAddr(ip.Addr())
	if class == classReserved {
		if reserved.IsValid() {
			return fmt.Errorf("%s can not call reserved addresses (%s), deny '%s'", p.usage, reserved, denyTarget(host, ip))
		}
		return fmt.Errorf("%s can not call reserved addresses (zoned address), deny '%s'", p.usage, denyTarget(host, ip))
	}
	return p.gate(host, ip, class)
}

// gate enforces the deny list, then the allow list, lax mode exempts public and unresolved targets, the dial-time check classifies the resolved address
func (p *Policy) gate(host string, ip netip.AddrPort, class addrClass) error {
	if err := p.blockReason(host, ip); err != nil {
		return err
	}
	if p.mode == Lax && (class == classPublic) {
		return nil
	}
	return p.allowCheck(host, ip, class)
}

// allowCheck returns nil when the allow list names the target, else an error naming the allow entry it needs
func (p *Policy) allowCheck(host string, ip netip.AddrPort, class addrClass) error {
	if p.allow.MatchIPAddr(ip) {
		return nil
	}
	if class == classRestricted {
		return p.allowRestricted(host, ip)
	}
	if p.allow.MatchHostname(host, ip.Port()) {
		return nil
	}
	return p.notAllowedError(denyTarget(host, ip))
}

// allowRestricted allows a non-public target via a hostname entry within the internal
// ranges
func (p *Policy) allowRestricted(host string, ip netip.AddrPort) error {
	if len(p.internalRanges) > 0 {
		if _, ok := p.allow.matchHostnameRule(host, ip.Port()); ok && inRange(p.internalRanges, ip.Addr()) {
			return nil
		}
		if rule, ok := p.allow.matchHostEntry(host); ok {
			if !inRange(p.internalRanges, ip.Addr()) {
				return fmt.Errorf("%s resolves outside the internal ranges allowed for hostnames (check your %s setting), deny '%s'", p.usage, p.internalRangesKey, denyTarget(host, ip))
			}
			return fmt.Errorf("%s port %d is not allowed for %q (check your %s setting), deny '%s'", p.usage, ip.Port(), rule.pattern, p.allowKey, denyTarget(host, ip))
		}
	}
	err := fmt.Errorf("%s can not call non-public addresses without an allow entry (check your %s setting), deny '%s'", p.usage, p.allowKey, denyTarget(host, ip))
	if _, ok := p.allow.matchHostEntry(host); ok {
		// invalid entries are reported at startup by the settings loader, so one hint covers both cases
		err = fmt.Errorf("%w — set or fix %s to let hostname entries cover non-public targets", err, p.internalRangesKey)
	}
	return err
}

func (p *Policy) blockReason(host string, ip netip.AddrPort) error {
	if entry, ok := p.block.MatchHostnameEntry(host, ip.Port()); ok {
		return fmt.Errorf("%s can not call blocked HTTP servers (matched hostname entry %q, check your %s setting), deny '%s'", p.usage, entry, p.blockKey, denyTarget(host, ip))
	}
	if entry, ok := p.block.MatchIPAddrEntry(ip); ok {
		return fmt.Errorf("%s can not call blocked HTTP servers (matched IP entry %s, check your %s setting), deny '%s'", p.usage, entry, p.blockKey, denyTarget(host, ip))
	}
	return nil
}

// denyTarget renders the checked address for denial messages, host is empty for an IP literal, ip is invalid for an unresolved host name
func denyTarget(host string, ip netip.AddrPort) string {
	if !ip.Addr().IsValid() {
		return host
	}
	if host == "" {
		return ip.Addr().String()
	}
	return fmt.Sprintf("%s(%s)", host, ip.Addr())
}

// CheckHost pre-screens a target URL whose host name may be unresolved or an IP literal.
func (p *Policy) CheckHost(u *url.URL) error {
	host, port := u.Hostname(), dialPort(u)
	ip, err := netip.ParseAddr(host)
	addrPort := netip.AddrPortFrom(ip, port)
	if err == nil {
		return p.checkAddr("", addrPort)
	}
	return p.checkAddr(host, addrPort) // invalid ip doesn't match anything
}

// CheckHostIPs reports whether u's host may be called, it resolves the host and at least one
// address must pass, matching the dialer which tries the resolved addresses in turn.
func (p *Policy) CheckHostIPs(u *url.URL) error {
	// hosts behind a proxy may have no DNS resolver, the name-only CheckHost screen still applies
	ips, _ := net.LookupIP(u.Hostname())
	return p.checkHostIPs(u, ips)
}

func (p *Policy) checkHostIPs(u *url.URL, ips []net.IP) error {
	if len(ips) == 0 {
		return p.CheckHost(u)
	}
	host, port := u.Hostname(), dialPort(u)
	var firstErr error
	for _, ip := range ips {
		addr, _ := netip.AddrFromSlice(ip)
		addrPort := netip.AddrPortFrom(addr, port)
		err := p.checkAddr(host, addrPort)
		if err == nil {
			return nil // the dialer falls back to the next address, one passing address is enough
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

const (
	// dialTimeout bounds a dial, dialKeepAlive probes idle connections — every egress dialer uses these
	dialTimeout   = 30 * time.Second
	dialKeepAlive = 30 * time.Second
)

// NewDialContext returns a dial function that checks the resolved address at connect time, so DNS rebinding can't bypass it.
func (p *Policy) NewDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
		host, _, err := net.SplitHostPort(addr) // the requested host, also on redirects where the request's Host is empty
		if err != nil {
			return nil, err
		}
		dialer.Control = func(_, ipAddr string, _ syscall.RawConn) error {
			addrPort, err := netip.ParseAddrPort(ipAddr)
			if err != nil {
				return fmt.Errorf("%s can only call HTTP servers via TCP, deny '%s(%s)': %w", p.usage, host, ipAddr, err)
			}
			if err := p.checkAddr(host, addrPort); err != nil {
				log.Warn("egress denied: %v", err) // audit trail for blocked connection attempts
				return fmt.Errorf("%w: %w", ErrDenied, err)
			}
			return nil
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

// Proxy selects the proxy for req.
func (p *Policy) Proxy(req *http.Request) (proxyURL *url.URL, err error) {
	if p.proxyFunc != nil {
		proxyURL, err = p.proxyFunc(req)
	}
	if proxyURL != nil {
		if _, ok := proxyPorts[proxyURL.Scheme]; !ok {
			return nil, fmt.Errorf("unsupported proxy scheme %q, use http, https or socks5", proxyURL.Scheme)
		}
	}
	return proxyURL, err
}

// ProxyDialer returns a dialer for the operator's proxies, which are configuration rather than user input.
func ProxyDialer() *net.Dialer {
	return &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
}

// proxyTransport routes each request through the dialer its traffic deserves: a request
// the selector sends to a proxy is dialed by a transport that only dials that proxy
// (operator configuration, not user input, so unchecked), every other request is dialed
// with the policy check. The split is by role, not by address: a direct request whose
// target is a proxy's address is still checked.
type proxyTransport struct {
	transport *http.Transport // direct requests: no proxy, the policy-checked dialer
	selector  func(*http.Request) (*url.URL, error)
	mu        sync.Mutex
	proxied   map[string]*http.Transport // proxy URL -> transport that only dials that proxy
}

// NewHTTPTransport returns a RoundTripper that enforces the policy on direct dials and
// lets proxy dials through. The base transport is configured for the policy: DialContext
// becomes the policy's dialer, DialTLSContext is reset so TLS dials go through the same
// policy-checked dialer, the proxy selector is captured for the proxyTransport (and cleared
// on the base transport, which must never run it again), and unset transport settings get
// their defaults; nil builds a default transport.
func (p *Policy) NewHTTPTransport(base *http.Transport) http.RoundTripper {
	if base == nil {
		base = &http.Transport{}
	}

	selector := base.Proxy
	if selector == nil {
		selector = p.Proxy
	}
	base.Proxy = nil // the proxyTransport runs the selector once per request; the direct transport must dial directly
	base.DialContext = p.NewDialContext()
	base.DialTLSContext = nil // a custom TLS dialer would bypass the policy's dial-time check
	if !base.ForceAttemptHTTP2 {
		base.ForceAttemptHTTP2 = true
	}
	base.MaxIdleConns = cmp.Or(base.MaxIdleConns, 100)
	base.IdleConnTimeout = cmp.Or(base.IdleConnTimeout, 90*time.Second)
	base.TLSHandshakeTimeout = cmp.Or(base.TLSHandshakeTimeout, 10*time.Second)
	base.ExpectContinueTimeout = cmp.Or(base.ExpectContinueTimeout, 1*time.Second)
	return &proxyTransport{transport: base, selector: selector, proxied: map[string]*http.Transport{}}
}

// RoundTrip sends req through the proxy the selector picks, or dials it directly with the policy check.
func (t *proxyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	proxyURL, err := t.selector(req)
	if err != nil {
		return nil, err
	}
	if proxyURL == nil {
		return t.transport.RoundTrip(req)
	}
	return t.proxiedTransport(proxyURL).RoundTrip(req)
}

// CloseIdleConnections closes the idle connections of the direct and all proxied transports.
func (t *proxyTransport) CloseIdleConnections() {
	t.transport.CloseIdleConnections()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, tr := range t.proxied {
		tr.CloseIdleConnections()
	}
}

// proxiedTransport returns the transport that only dials proxyURL, creating it on first use.
func (t *proxyTransport) proxiedTransport(proxyURL *url.URL) *http.Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	tr, ok := t.proxied[proxyURL.String()]
	if !ok {
		tr = newProxiedTransport(proxyURL, t.transport)
		t.proxied[proxyURL.String()] = tr
	}
	return tr
}

// newProxiedTransport returns a transport that only dials proxyURL, unchecked: the proxy is operator configuration.
func newProxiedTransport(proxyURL *url.URL, base *http.Transport) *http.Transport {
	tr := base.Clone() // carries over every setting of the direct transport
	tr.Proxy = http.ProxyURL(proxyURL)
	tr.DialContext = ProxyDialer().DialContext
	return tr
}
