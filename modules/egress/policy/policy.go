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
	"sync"
	"syscall"
	"time"
)

// ErrDenied wraps a policy rejection, so callers can tell it from a network failure.
var ErrDenied = errors.New("denied by egress policy")

type Policy struct {
	usage              string
	allow, block       *HostMatchList
	allowKey, blockKey string // the settings the lists were read from, named in rejections
	localNeedsIPAllow  bool
	proxyFunc          func(*http.Request) (*url.URL, error)
	proxyAddrs         sync.Map // dial addresses proxyFunc returned, the operator's proxies are exempt from the lists
}

type Option func(*Policy)

// WithAllow sets the allow list from the setting named by key, an empty list allows every target.
func WithAllow(hostList, key string) Option {
	return func(p *Policy) {
		p.allow, p.allowKey = ParseHostMatchList(hostList), key
	}
}

func WithBlock(hostList, key string) Option {
	return func(p *Policy) {
		p.block, p.blockKey = ParseHostMatchList(hostList), key
	}
}

// WithLocalNeedsIPAllow requires private, loopback and CGNAT targets to match a builtin or CIDR allow entry, a host name match is not enough.
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

// NewPolicy compiles a policy enforced on every outbound dial, usage names the caller in rejections.
func NewPolicy(usage string, opts ...Option) *Policy {
	p := &Policy{usage: usage, allow: &HostMatchList{}, block: &HostMatchList{}}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ProxyDialAddr returns the address the transport dials for proxy URL u, with net/http's default port for its scheme.
func ProxyDialAddr(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func (p *Policy) blockedError(target string) error {
	return fmt.Errorf("%s can not call blocked HTTP servers (check your %s setting), deny '%s'", p.usage, p.blockKey, target)
}

func (p *Policy) notAllowedError(target string) error {
	return fmt.Errorf("%s can only call allowed HTTP servers (check your %s setting), deny '%s'", p.usage, p.allowKey, target)
}

// checkAddr reports whether host, resolved to ip, may be called.
func (p *Policy) checkAddr(host string, ip netip.Addr) error {
	ip = ip.Unmap()
	netIP := net.IP(ip.AsSlice())
	class := classifyAddr(ip)
	target := fmt.Sprintf("%s(%s)", host, ip)
	switch {
	case class == classReserved && !p.allow.matchesIP(netIP):
		return fmt.Errorf("%s can not call reserved addresses, deny '%s'", p.usage, target)
	case p.block.MatchHostOrIP(host, netIP):
		return p.blockedError(target)
	case p.localNeedsIPAllow && class == classRestricted && !p.allow.matchesIP(netIP),
		!p.allow.IsEmpty() && !p.allow.MatchHostOrIP(host, netIP):
		return p.notAllowedError(target)
	}
	return nil
}

// CheckHost pre-screens an unresolved host name or IP literal.
func (p *Policy) CheckHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		return p.checkAddr(host, ip)
	}
	if p.block.MatchHostName(host) {
		return p.blockedError(host)
	}
	if !p.allow.IsEmpty() && !p.allow.MatchHostName(host) {
		return p.notAllowedError(host)
	}
	return nil
}

// CheckHostIPs reports whether host, resolved to ips, may be called, every address must pass as the dialer may pick any.
func (p *Policy) CheckHostIPs(host string, ips []net.IP) error {
	if len(ips) == 0 {
		return p.CheckHost(host)
	}
	for _, ip := range ips {
		addr, _ := netip.AddrFromSlice(ip)
		if err := p.checkAddr(host, addr); err != nil {
			return err
		}
	}
	return nil
}

// NewDialContext returns a dial function that checks the resolved address at connect time, so DNS rebinding can't bypass it.
func (p *Policy) NewDialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		if _, isProxy := p.proxyAddrs.Load(addr); !isProxy {
			host, _, err := net.SplitHostPort(addr) // the requested host, also on redirects where the request's Host is empty
			if err != nil {
				return nil, err
			}
			dialer.Control = func(_, ipAddr string, _ syscall.RawConn) error {
				addrPort, err := netip.ParseAddrPort(ipAddr)
				if err != nil {
					return fmt.Errorf("%s can only call HTTP servers via TCP, deny '%s(%s)': %w", p.usage, host, ipAddr, err)
				}
				if err := p.checkAddr(host, addrPort.Addr()); err != nil {
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
		p.proxyAddrs.LoadOrStore(ProxyDialAddr(proxyURL), struct{}{})
	}
	return proxyURL, err
}

func (p *Policy) NewHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 p.Proxy,
		DialContext:           p.NewDialContext(),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
