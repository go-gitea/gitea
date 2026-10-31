// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net/netip"
	"slices"
)

type addrClass uint8

const (
	classPublic addrClass = iota
	classRestricted
	classReserved
)

var cgnatRange = netip.MustParsePrefix("100.64.0.0/10") // RFC 6598

// reservedRanges are never dialable, based on https://microsoft.github.io/AntiSSRF/ipaddressranges.html
var reservedRanges = func() (ranges []netip.Prefix) {
	for _, cidr := range []string{
		"0.0.0.0/8",        // "this network"
		"168.63.129.16/32", // Azure WireServer
		"192.88.99.0/24",   // 6to4 relay anycast
		"224.0.0.0/4",      // multicast
		"240.0.0.0/4",      // reserved, incl. limited broadcast
		"::/96",            // IPv4-compatible, embeds IPv4
		"::ffff:0:0:0/96",  // IPv4-translated, embeds IPv4
		"64:ff9b::/96",     // wkp NAT64
		"64:ff9b:1::/48",   // local-use NAT64
		"2001::/32",        // Teredo, embeds IPv4
		"2002::/16",        // 6to4, embeds IPv4
		"ff00::/8",         // multicast
	} {
		ranges = append(ranges, netip.MustParsePrefix(cidr))
	}
	return ranges
}()

// restrictedRanges are dialable if they have been explicitly allowed.
var restrictedRanges = func() (ranges []netip.Prefix) {
	for _, cidr := range []string{
		"192.0.0.0/24",      // IETF protocol assignments
		"192.0.2.0/24",      // TEST-NET-1
		"192.31.196.0/24",   // AS112
		"192.52.193.0/24",   // AMT
		"192.175.48.0/24",   // AS112
		"198.18.0.0/15",     // benchmarking
		"198.51.100.0/24",   // TEST-NET-2
		"203.0.113.0/24",    // TEST-NET-3
		"100::/64",          // discard-only
		"100:0:0:1::/64",    // dummy
		"2001::/23",         // IETF protocol assignments
		"2001:db8::/32",     // documentation
		"2620:4f:8000::/48", // AS112
		"3fff::/20",         // documentation
		"5f00::/16",         // SRv6 SIDs
		"fec0::/10",         // site-local
	} {
		ranges = append(ranges, netip.MustParsePrefix(cidr))
	}
	return ranges
}()

// classifyAddr reports the class of a canonical address, and for reserved addresses the matching range.
func classifyAddr(ip netip.Addr) (addrClass, netip.Prefix) {
	if ip.Zone() != "" {
		return classReserved, netip.Prefix{}
	}
	if !ip.IsLoopback() {
		for _, prefix := range reservedRanges {
			if prefix.Contains(ip) {
				return classReserved, prefix
			}
		}
	}
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || cgnatRange.Contains(ip) || inRange(restrictedRanges, ip) {
		return classRestricted, netip.Prefix{}
	}
	return classPublic, netip.Prefix{}
}

func inRange(p []netip.Prefix, ip netip.Addr) bool {
	return slices.ContainsFunc(p, func(p netip.Prefix) bool { return p.Contains(ip) })
}
