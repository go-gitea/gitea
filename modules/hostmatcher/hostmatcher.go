// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hostmatcher

import (
	"net"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// HostMatchList is used to check if a host or IP is in a list.
type HostMatchList struct {
	SettingKeyHint string
	SettingValue   string

	// builtins networks
	builtins []string
	// patterns for host names (with wildcard support)
	patterns []string
	// ipNets is the CIDR network list
	ipNets []*net.IPNet
}

// MatchBuiltinExternal A valid global-unicast IP that is neither private (see MatchBuiltinPrivate)
// nor a reserved special-purpose range (see reservedIPNets); i.e. a routable host on the public internet.
const MatchBuiltinExternal = "external"

// reservedIPNets are special-purpose ranges that net.IP.IsPrivate omits but that must not be
// treated as public/external destinations (CGNAT, cloud metadata, IPv6 transition, etc.). We layer
// these on top of net.IP.IsPrivate (RFC 1918 / RFC 4193) so future additions to Go's IsPrivate are
// picked up automatically, while still covering the ranges it leaves out; otherwise the default
// allow-list would let authenticated users reach cloud metadata, internal, and IPv6 transition
// endpoints (SSRF), and a "private" block-list would fail to catch them.
var reservedIPNets = sync.OnceValue(func() []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range []string{
		// IPv4
		"100.64.0.0/10",    // RFC 6598 Carrier-Grade NAT
		"168.63.129.16/32", // Azure WireServer metadata endpoint
		"192.0.0.0/24",     // RFC 6890 IETF protocol assignments
		"192.0.2.0/24",     // RFC 5737 TEST-NET-1
		"192.88.99.0/24",   // RFC 7526 6to4 relay anycast (deprecated)
		"198.18.0.0/15",    // RFC 2544 benchmarking
		"198.51.100.0/24",  // RFC 5737 TEST-NET-2
		"203.0.113.0/24",   // RFC 5737 TEST-NET-3
		// IPv6
		"100::/64",       // RFC 6666 discard-only
		"64:ff9b::/96",   // RFC 6052 NAT64 (can embed IPv4 such as 169.254.169.254)
		"64:ff9b:1::/48", // RFC 8215 local-use NAT64
		"2001::/32",      // RFC 4380 Teredo tunneling (embeds IPv4)
		"2001:10::/28",   // RFC 4843 ORCHID (deprecated)
		"2001:20::/28",   // RFC 7343 ORCHIDv2
		"2001:db8::/32",  // RFC 3849 documentation
		"2002::/16",      // RFC 3056 6to4 (embeds IPv4)
	} {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			panic("hostmatcher: invalid reserved CIDR " + cidr + ": " + err.Error())
		}
		nets = append(nets, ipNet)
	}
	return nets
})

// isReservedIP reports whether ip falls in reserved special-purpose
// range (see reservedIPNets) that must not be considered a public/external destination.
func isReservedIP(ip net.IP) bool {
	for _, ipNet := range reservedIPNets() {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// MatchBuiltinPrivate RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) and RFC 4193 (FC00::/7),
// plus the reserved special-purpose ranges in reservedIPNets (CGNAT, NAT64, cloud metadata, etc.).
// Also called LAN/Intranet.
const MatchBuiltinPrivate = "private"

// MatchBuiltinLoopback 127.0.0.0/8 for IPv4 and ::1/128 for IPv6, localhost is included.
const MatchBuiltinLoopback = "loopback"

func isBuiltin(s string) bool {
	return s == MatchBuiltinExternal || s == MatchBuiltinPrivate || s == MatchBuiltinLoopback
}

// ParseHostMatchList parses the host list HostMatchList
func ParseHostMatchList(settingKeyHint, hostList string) *HostMatchList {
	hl := &HostMatchList{SettingKeyHint: settingKeyHint, SettingValue: hostList}
	for s := range strings.SplitSeq(hostList, ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(s)
		if err == nil {
			hl.ipNets = append(hl.ipNets, ipNet)
		} else if isBuiltin(s) {
			hl.builtins = append(hl.builtins, s)
		} else {
			hl.patterns = append(hl.patterns, s)
		}
	}
	return hl
}

// ParseSimpleMatchList parse a simple matchlist (no built-in networks, no CIDR support, only wildcard pattern match)
func ParseSimpleMatchList(settingKeyHint, matchList string) *HostMatchList {
	hl := &HostMatchList{
		SettingKeyHint: settingKeyHint,
		SettingValue:   matchList,
	}
	for s := range strings.SplitSeq(matchList, ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		// we keep the same result as old `matchlist`, so no builtin/CIDR support here, we only match wildcard patterns
		hl.patterns = append(hl.patterns, s)
	}
	return hl
}

// AppendBuiltin appends more builtins to match
func (hl *HostMatchList) AppendBuiltin(builtin string) {
	hl.builtins = append(hl.builtins, builtin)
}

// IsEmpty checks if the checklist is empty
func (hl *HostMatchList) IsEmpty() bool {
	return hl == nil || (len(hl.builtins) == 0 && len(hl.patterns) == 0 && len(hl.ipNets) == 0)
}

func (hl *HostMatchList) checkPattern(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, pattern := range hl.patterns {
		if matched, _ := filepath.Match(pattern, host); matched {
			return true
		}
	}
	return false
}

// matchesIP determines if the given IP matches any of the configured rules
func (hl *HostMatchList) matchesIP(ip net.IP) bool {
	if slices.Contains(hl.patterns, "*") {
		return true
	}
	for _, builtin := range hl.builtins {
		switch builtin {
		case MatchBuiltinExternal:
			// External address must be a global unicast, must not be in reserved range and must not be in private range
			if ip.IsGlobalUnicast() && !isReservedIP(ip) && !ip.IsPrivate() {
				return true
			}
		case MatchBuiltinPrivate:
			// Private address must be global unicast, must not be in range we explicitly exclude for security reasons
			// and must be in private range
			if ip.IsGlobalUnicast() && !isReservedIP(ip) && ip.IsPrivate() {
				return true
			}
		case MatchBuiltinLoopback:
			if ip.IsLoopback() {
				return true
			}
		}
	}
	for _, ipNet := range hl.ipNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// MatchHostName checks if the host matches an allow/deny(block) list
func (hl *HostMatchList) MatchHostName(host string) bool {
	if hl == nil {
		return false
	}

	hostname, _, err := net.SplitHostPort(host)
	if err != nil {
		hostname = host
	}
	if hl.checkPattern(hostname) {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil {
		return hl.matchesIP(ip)
	}
	return false
}

// MatchIPAddr checks if the IP matches an allow/deny(block) list, it's safe to pass `nil` to `ip`
func (hl *HostMatchList) MatchIPAddr(ip net.IP) bool {
	if hl == nil {
		return false
	}
	host := ip.String() // nil-safe, we will get "<nil>" if ip is nil
	return hl.checkPattern(host) || hl.matchesIP(ip)
}

// MatchHostOrIP checks if the host or IP matches an allow/deny(block) list
func (hl *HostMatchList) MatchHostOrIP(host string, ip net.IP) bool {
	return hl.MatchHostName(host) || hl.MatchIPAddr(ip)
}
