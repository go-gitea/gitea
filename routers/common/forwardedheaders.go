// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"gitea.dev/modules/log"
)

func ForwardedHeadersHandler(limit int, trustedProxies []string) func(h http.Handler) http.Handler {
	limit = max(limit, 1)
	var trusted []netip.Prefix
	for _, s := range trustedProxies {
		if s == "*" {
			trusted = append(trusted, netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0"))
			continue
		}
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			addr := parseClientAddr(s)
			if !addr.IsValid() {
				log.Error("Ignoring invalid trusted proxy %q", s)
				continue
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		trusted = append(trusted, prefix)
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if req.RemoteAddr == "@" { // unix socket
				req.RemoteAddr = "127.0.0.1:0"
			}
			if isTrustedProxy(req.RemoteAddr, trusted) {
				if addr := forwardedClientIP(req.Header, limit); addr.IsValid() {
					req.RemoteAddr = netip.AddrPortFrom(addr, 0).String()
				}
			}
			h.ServeHTTP(resp, req)
		})
	}
}

func isTrustedProxy(remoteAddr string, trusted []netip.Prefix) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	addr := parseClientAddr(host)
	return slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(addr) })
}

func forwardedClientIP(header http.Header, limit int) netip.Addr {
	realIPs := header.Values("X-Real-Ip")
	if len(realIPs) > 0 && realIPs[len(realIPs)-1] != "" {
		return parseClientAddr(realIPs[len(realIPs)-1]) // the closest hop wrote the last value, an empty one falls through
	}
	remaining, leftmost := limit, ""
	for _, value := range slices.Backward(header.Values("X-Forwarded-For")) { // walking from the right keeps an attacker-chosen chain length free
		for rest := value; rest != ""; {
			entry := rest
			if comma := strings.LastIndexByte(rest, ','); comma >= 0 {
				entry, rest = rest[comma+1:], rest[:comma]
			} else {
				rest = ""
			}
			if entry = strings.TrimSpace(entry); entry == "" {
				continue
			}
			leftmost = entry
			if remaining--; remaining == 0 {
				return parseClientAddr(entry)
			}
		}
	}
	return parseClientAddr(leftmost)
}

// parseClientAddr rejects non-IP input, so a header a proxy forgot to overwrite cannot put an arbitrary string into RemoteAddr.
func parseClientAddr(s string) netip.Addr {
	addr, _ := netip.ParseAddr(s)
	return addr.Unmap().WithZone("")
}
