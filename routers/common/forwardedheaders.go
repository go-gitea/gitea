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
	trustAll := slices.Contains(trustedProxies, "*")
	var trusted []netip.Prefix
	for _, s := range trustedProxies {
		if s == "*" {
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
			if trustAll || slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(peer.Addr().Unmap()) }) {
				if addr, err := parseForwardedAddr(forwardedClientIP(req.Header, limit)); err == nil {
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
	xff := strings.Join(header.Values("X-Forwarded-For"), ",")
	for range limit - 1 {
		if comma := strings.LastIndexByte(xff, ','); comma >= 0 {
			xff = xff[:comma]
		}
	}
	return strings.TrimSpace(xff[strings.LastIndexByte(xff, ',')+1:])
}

func parseForwardedAddr(s string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr, nil
	}
	addrPort, err := netip.ParseAddrPort(s)
	return addrPort.Addr(), err
}
