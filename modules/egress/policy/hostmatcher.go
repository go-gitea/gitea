// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"
)

// HostMatchList is used to check if a host or IP is in a list.
type HostMatchList struct {
	builtins []string
	patterns []string
	ipNets   []*net.IPNet
}

// MatchBuiltinExternal A valid global-unicast IP that is neither private nor reserved, i.e. a routable host on the public internet.
const MatchBuiltinExternal = "external"

// MatchBuiltinPrivate RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16), RFC 4193 (FC00::/7) and RFC 6598 CGNAT (100.64.0.0/10).
// Also called LAN/Intranet.
const MatchBuiltinPrivate = "private"

// MatchBuiltinLoopback 127.0.0.0/8 for IPv4 and ::1/128 for IPv6, localhost is included.
const MatchBuiltinLoopback = "loopback"

func isBuiltin(s string) bool {
	return s == MatchBuiltinExternal || s == MatchBuiltinPrivate || s == MatchBuiltinLoopback
}

// ParseHostMatchList parses the host list HostMatchList
func ParseHostMatchList(hostList string) *HostMatchList {
	hl := &HostMatchList{}
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

// IsEmpty checks if the checklist is empty
func (hl *HostMatchList) IsEmpty() bool {
	return len(hl.builtins) == 0 && len(hl.patterns) == 0 && len(hl.ipNets) == 0
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

// matchesIP reports whether ip matches a builtin or CIDR entry, host name patterns are not consulted
func (hl *HostMatchList) matchesIP(ip net.IP) bool {
	addr, _ := netip.AddrFromSlice(ip)
	class := classifyAddr(addr)
	for _, builtin := range hl.builtins {
		switch builtin {
		case MatchBuiltinExternal:
			if ip.IsGlobalUnicast() && class == classPublic {
				return true
			}
		case MatchBuiltinPrivate:
			if ip.IsGlobalUnicast() && class == classRestricted {
				return true
			}
		case MatchBuiltinLoopback:
			if ip.IsLoopback() {
				return true
			}
		}
	}
	return slices.ContainsFunc(hl.ipNets, func(ipNet *net.IPNet) bool { return ipNet.Contains(ip) })
}

// MatchHostName checks if the host matches an allow/deny(block) list
func (hl *HostMatchList) MatchHostName(host string) bool {
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
	host := ip.String() // nil-safe, we will get "<nil>" if ip is nil
	return hl.checkPattern(host) || hl.matchesIP(ip)
}

// MatchHostOrIP checks if the host or IP matches an allow/deny(block) list
func (hl *HostMatchList) MatchHostOrIP(host string, ip net.IP) bool {
	return hl.MatchHostName(host) || hl.MatchIPAddr(ip)
}
