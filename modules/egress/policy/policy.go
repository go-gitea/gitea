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
	allow              *AllowList
	block              *BlockList
	allowKey, blockKey string // the settings the lists were read from, named in rejections
	localNeedsIPAllow  bool
	proxyFunc          func(*http.Request) (*url.URL, error)
	proxyAddrs         sync.Map // dial addresses proxyFunc returned, the operator's proxies are exempt from the lists
}

type Option func(*Policy)

// WithAllow sets the allow list from the setting named by key
func WithAllow(hostList, key string) Option {
	return func(p *Policy) {
		p.allow, p.allowKey = NewAllowList(hostList), key
	}
}

func WithBlock(hostList, key string) Option {
	return func(p *Policy) {
		p.block, p.blockKey = NewBlockList(hostList), key
	}
}

// WithLocalNeedsIPAllow requires private, loopback and CGNAT targets to match an IP allow entry (CIDR or named range), a host name match is not enough.
func WithLocalNeedsIPAllow() Option {
	return func(p *Policy) {
		p.localNeedsIPAllow = true
	}
}

func WithProxy(proxyFunc func(*http.Request) (*url.URL, error)) Option {
	return func(p *Policy) {
		p.proxyFunc = proxyFunc
	}
}

// WithMode sets the policy mode, default is Lax
func WithMode(mode Mode) Option {
	return func(p *Policy) {
		p.mode = mode
	}
}

// NewPolicy compiles a policy enforced on every outbound dial, usage names the caller in rejections.
func NewPolicy(usage string, opts ...Option) *Policy {
	p := &Policy{usage: usage, block: NewBlockList(""), allow: NewAllowList("")}
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

func (p *Policy) blockedError(target string) error {
	return fmt.Errorf("%s can not call blocked HTTP servers (check your %s setting), deny '%s'", p.usage, p.blockKey, target)
}

func (p *Policy) notAllowedError(target string) error {
	return fmt.Errorf("%s can only call allowed HTTP servers (check your %s setting), deny '%s'", p.usage, p.allowKey, target)
}

func (p *Policy) checkAddr(host string, ip netip.AddrPort) error {
	ip = netip.AddrPortFrom(ip.Addr().Unmap(), ip.Port())
	class := classifyAddr(ip.Addr())
	if class == classReserved {
		return fmt.Errorf("%s can not call reserved addresses, deny '%s'", p.usage, denyTarget(host, ip))
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
	// with localNeedsIPAllow a restricted target needs an IP entry, a hostname match is not enough
	hostnameOk := !p.localNeedsIPAllow || class != classRestricted
	if hostnameOk && p.allow.MatchHostname(host, ip.Port()) {
		return nil
	}
	if p.mode == Strict {
		return p.notAllowedError(denyTarget(host, ip))
	}
	if !hostnameOk {
		return fmt.Errorf("%s needs an explicit IP allow entry (private/loopback/CGNAT)", denyTarget(host, ip))
	}
	return fmt.Errorf("%s needs an explicit allow entry (private/loopback/CGNAT)", denyTarget(host, ip))
}

func (p *Policy) blockReason(host string, ip netip.AddrPort) error {
	if p.block.MatchHostname(host, ip.Port()) || p.block.MatchIPAddr(ip) {
		return p.blockedError(denyTarget(host, ip))
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

// CheckHostIPs reports whether u's host may be called, it resolves the host and every address must pass as the dialer may pick any.
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
	for _, ip := range ips {
		addr, _ := netip.AddrFromSlice(ip)
		addrPort := netip.AddrPortFrom(addr, port)
		if err := p.checkAddr(host, addrPort); err != nil {
			return err
		}
	}
	return nil
}

// NewDialContext returns a dial function that checks the resolved address at connect time, so DNS rebinding can't bypass it.
func (p *Policy) NewDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return p.dialContext(false)
}

// dialContext can let through the proxies the selector returned, for a transport dialing proxies and targets alike
func (p *Policy) dialContext(allowProxies bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		if _, isProxy := p.proxyAddrs.Load(addr); !allowProxies || !isProxy {
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
					return fmt.Errorf("%w: %w", ErrDenied, err)
				}
				return nil
			}
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

// Proxy selects the proxy for req and lets the dialer reach it.
func (p *Policy) Proxy(req *http.Request) (proxyURL *url.URL, err error) {
	if p.proxyFunc != nil {
		proxyURL, err = p.proxyFunc(req)
	}
	if proxyURL != nil {
		if _, ok := proxyPorts[proxyURL.Scheme]; !ok {
			return nil, fmt.Errorf("unsupported proxy scheme %q, use http, https or socks5", proxyURL.Scheme)
		}
		p.proxyAddrs.LoadOrStore(ProxyDialAddr(proxyURL), struct{}{})
	}
	return proxyURL, err
}

func (p *Policy) NewHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 p.Proxy,
		DialContext:           p.dialContext(true),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
