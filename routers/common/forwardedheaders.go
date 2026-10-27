// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"net/http"
	"net/netip"
	"slices"
	"strconv"
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
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
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
				if client := forwardedClient(req.Header, limit); client.IsValid() {
					req.RemoteAddr = netip.AddrPortFrom(client.Addr().Unmap().WithZone(""), client.Port()).String()
				}
			}
			h.ServeHTTP(resp, req)
		})
	}
}

// forwardedClient reads Forwarded only without X-Forwarded-For, so proxies passing a client-sent Forwarded through stay safe
func forwardedClient(header http.Header, limit int) netip.AddrPort {
	if realIPs := header["X-Real-Ip"]; len(realIPs) > 0 && realIPs[len(realIPs)-1] != "" {
		client := parseNode(realIPs[len(realIPs)-1]) // the closest hop wrote the last value
		if realPorts := header["X-Real-Port"]; client.Port() == 0 && len(realPorts) > 0 {
			if port, err := strconv.ParseUint(realPorts[len(realPorts)-1], 10, 16); err == nil {
				client = netip.AddrPortFrom(client.Addr(), uint16(port))
			}
		}
		return client
	}
	if xff := header["X-Forwarded-For"]; len(xff) > 0 {
		return parseNode(elementFromEnd(xff, limit))
	}
	return parseNode(forwardedFor(elementFromEnd(header["Forwarded"], limit)))
}

// elementFromEnd skips empty elements, which RFC 9110 doesn't count
func elementFromEnd(values []string, n int) string {
	list, entry := strings.Join(values, ","), ""
	for list != "" && n > 0 {
		var element string
		list, element = cutLast(list, ',')
		if element = strings.TrimSpace(element); element != "" {
			entry, n = element, n-1
		}
	}
	return entry
}

// forwardedFor returns nothing on a repeated "for", which RFC 7239 forbids
func forwardedFor(element string) string {
	node, found := "", false
	for element != "" {
		var pair string
		element, pair = cutLast(element, ';')
		key, value, _ := strings.Cut(strings.TrimSpace(pair), "=")
		if !strings.EqualFold(key, "for") {
			continue
		}
		if found {
			return ""
		}
		node, found = unquote(value), true
	}
	return node
}

// cutLast scans from the right so a malformed client-written prefix can't shift proxy-written elements
func cutLast(s string, sep byte) (before, after string) {
	if i := strings.LastIndexByte(s, sep); !strings.Contains(s[i+1:], `"`) {
		return s[:max(i, 0)], s[i+1:]
	}
	quoted := false
	for i := len(s) - 1; i >= 0; i-- {
		switch s[i] {
		case '"':
			quoted = !quoted || (i-len(strings.TrimRight(s[:i], `\`)))%2 == 1
		case sep:
			if !quoted {
				return s[:i], s[i+1:]
			}
		}
	}
	return "", s
}

func unquote(s string) string {
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return s
	}
	s = s[1 : len(s)-1]
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseNode keeps numeric ports and drops RFC 7239 brackets and obfuscated ports
func parseNode(node string) netip.AddrPort {
	if node == "" {
		return netip.AddrPort{}
	}
	if addr, err := netip.ParseAddr(node); err == nil {
		return netip.AddrPortFrom(addr, 0)
	}
	if addrPort, err := netip.ParseAddrPort(node); err == nil {
		return addrPort
	}
	if strings.HasPrefix(node, "[") {
		node, _, _ = strings.Cut(node[1:], "]")
	} else {
		node, _, _ = strings.Cut(node, ":")
	}
	addr, _ := netip.ParseAddr(node)
	return netip.AddrPortFrom(addr, 0)
}
