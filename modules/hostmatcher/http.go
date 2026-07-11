// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// NewDialContext returns a DialContext for Transport, the DialContext will do allow/block list check
func NewDialContext(usage string, allowList, blockList *HostMatchList, proxy *url.URL) func(ctx context.Context, network, addr string) (net.Conn, error) {
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
				if proxy != nil {
					// Always allow the host of the proxy, but only on the specified port.
					if host == proxy.Hostname() && port == proxy.Port() {
						return nil
					}
				}

				// in Control func, the addr was already resolved to IP:PORT format, there is no cost to do ResolveTCPAddr here
				tcpAddr, err := net.ResolveTCPAddr(network, ipAddr)
				if err != nil {
					return fmt.Errorf("%s can only call HTTP servers via TCP, deny '%s(%s:%s)', err=%w", usage, host, network, ipAddr, err)
				}

				var blockedError error
				if blockList.MatchHostOrIP(host, tcpAddr.IP) {
					blockedError = fmt.Errorf("%s can not call blocked HTTP servers (check your %s setting), deny '%s(%s)'", usage, blockList.SettingKeyHint, host, ipAddr)
				}

				// if we have an allow-list, check the allow-list first
				if !allowList.IsEmpty() {
					if !allowList.MatchHostOrIP(host, tcpAddr.IP) {
						return fmt.Errorf("%s can only call allowed HTTP servers (check your %s setting), deny '%s(%s)'", usage, allowList.SettingKeyHint, host, ipAddr)
					}
				}
				// otherwise, we always follow the blocked list
				return blockedError
			},
		}
		return dialer.DialContext(ctx, network, addrOrHost)
	}
}

// AllowListOrExternal returns value, or the built-in "external" set when value is empty. An empty
// host-match list disables the allow-list check entirely (allow any host, including loopback/private),
// so a caller building an SSRF-protected client must apply this fallback rather than pass an empty
// allow-list string through.
func AllowListOrExternal(value string) string {
	if value == "" {
		return MatchBuiltinExternal
	}
	return value
}

// NewHTTPTransport builds an http.Transport whose request target is validated against the allow/block
// lists on BOTH the direct-dial path (DialContext) and the proxy path (Proxy). Pairing the two is the
// whole point: NewDialContext alone validates only the proxy's own address, leaving a configured proxy
// free to dial an otherwise-forbidden target (SSRF). base is the underlying proxy selector (e.g.
// proxy.Proxy()); proxyURLFixed is the fixed proxy address the dialer must always permit; tlsConfig may
// be nil. blockList may be nil for callers that only maintain an allow-list.
func NewHTTPTransport(usage string, allowList, blockList *HostMatchList, base func(*http.Request) (*url.URL, error), proxyURLFixed *url.URL, tlsConfig *tls.Config) *http.Transport {
	return &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           NewProxyFunc(usage, allowList, blockList, base),
		DialContext:     NewDialContext(usage, allowList, blockList, proxyURLFixed),
	}
}

// NewProxyFunc wraps a base proxy selector so a request whose target host is not allowed is refused
// before a proxy is used. Otherwise a configured proxy would dial on the target's behalf while the
// DialContext check only validates the proxy address, leaving the real target unconfined (SSRF).
// blockList may be nil for callers that only maintain an allow-list.
func NewProxyFunc(usage string, allowList, blockList *HostMatchList, base func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		proxyURL, err := base(req)
		if err != nil || proxyURL == nil {
			return proxyURL, err // direct connection: DialContext validates the target
		}
		if err := checkProxyTarget(usage, allowList, blockList, req.URL.Host); err != nil {
			return nil, err
		}
		return proxyURL, nil
	}
}

// checkProxyTarget applies the allow/block lists to a proxied request's target. Unlike the DialContext
// path, the proxy resolves DNS itself, so the target IP is not known here. We match wildcard patterns and
// IP literals directly, and for a bare hostname resolve it so IP-based builtins (loopback, private,
// external) and CIDRs can be evaluated. This is best-effort: the proxy re-resolves the name, so a
// rebinding TOCTOU remains, but it prevents a proxy from reaching an obviously disallowed target.
func checkProxyTarget(usage string, allowList, blockList *HostMatchList, hostPort string) error {
	host := hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		// a bracketed IPv6 literal without a port (e.g. "[::1]") reaches here; ParseIP/LookupIP
		// do not accept the brackets, so strip them before matching, otherwise the allow/block
		// lists silently fail to match this target.
		host = host[1 : len(host)-1]
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else if resolved, err := net.LookupIP(host); err == nil {
		ips = resolved
	}

	if !blockList.IsEmpty() {
		blocked := blockList.MatchHostName(host)
		for _, ip := range ips {
			blocked = blocked || blockList.MatchHostOrIP(host, ip)
		}
		if blocked {
			return fmt.Errorf("%s can not call blocked HTTP servers (check your %s setting), deny '%s'", usage, blockList.SettingKeyHint, host)
		}
	}

	if allowList.IsEmpty() || allowList.MatchHostName(host) {
		return nil
	}
	for _, ip := range ips {
		if allowList.MatchHostOrIP(host, ip) {
			return nil
		}
	}
	return fmt.Errorf("%s can only call allowed HTTP servers (check your %s setting), deny '%s'", usage, allowList.SettingKeyHint, host)
}
