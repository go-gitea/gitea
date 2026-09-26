// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/netip"
	"slices"
	"strings"

	"gitea.dev/modules/log"
)

func ForwardedHeadersHandler(limit int, trustedProxies []string) func(h http.Handler) http.Handler {
	var trusted []netip.Prefix
	for _, s := range trustedProxies {
		if s == "*" {
			trusted = append(trusted, netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0"))
			continue
		}
		prefix, err := netip.ParsePrefix(s)
		if err != nil {
			addr, _ := netip.ParseAddr(s)
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		if prefix.Addr().Is4In6() {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		if !prefix.IsValid() {
			log.Error("Ignoring invalid trusted proxy %q", s)
			continue
		}
		trusted = append(trusted, prefix)
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if req.RemoteAddr == "@" { // unix socket
				req.RemoteAddr = "127.0.0.1:0"
			}
			peer, _ := netip.ParseAddrPort(req.RemoteAddr)
			if slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(peer.Addr().Unmap()) }) {
				if addr, err := netip.ParseAddr(forwardedClientIP(req.Header, limit)); err == nil {
					req.RemoteAddr = netip.AddrPortFrom(addr.Unmap().WithZone(""), 0).String()
				}
			}
			h.ServeHTTP(resp, req)
		})
	}
}

func forwardedClientIP(header http.Header, limit int) string {
	if realIPs := header.Values("X-Real-Ip"); len(realIPs) > 0 && realIPs[len(realIPs)-1] != "" {
		return realIPs[len(realIPs)-1] // the closest hop wrote the last value
	}
	entry := ""
	for _, value := range slices.Backward(header.Values("X-Forwarded-For")) {
		for rest := value; rest != "" && limit > 0; {
			comma := strings.LastIndexByte(rest, ',')
			if field := strings.TrimSpace(rest[comma+1:]); field != "" {
				entry, limit = field, limit-1
			}
			rest = rest[:max(comma, 0)]
		}
	}
	return entry
}
