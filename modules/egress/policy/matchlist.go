// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"net/netip"
	"slices"
)

// addrClass is the intrinsic class of a dial target, before any list lookup.
type addrClass uint8

const (
	classPublic addrClass = iota
	classRestricted
	classReserved
)

var cgnatRange = netip.MustParsePrefix("100.64.0.0/10") // RFC 6598

// reservedRanges are never dialable unless an allow list names them by CIDR, based on https://microsoft.github.io/AntiSSRF/ipaddressranges.html
var reservedRanges = func() (ranges []netip.Prefix) {
	for _, cidr := range []string{
		"0.0.0.0/8",          // "this network"
		"100.100.100.200/32", // Alibaba Cloud metadata
		"168.63.129.16/32",   // Azure WireServer
		"169.254.0.0/16",     // link-local, cloud metadata endpoints
		"192.0.0.0/24",       // IETF protocol assignments
		"192.0.2.0/24",       // TEST-NET-1
		"192.31.196.0/24",    // AS112
		"192.52.193.0/24",    // AMT
		"192.88.99.0/24",     // 6to4 relay anycast
		"192.175.48.0/24",    // AS112
		"198.18.0.0/15",      // benchmarking
		"198.51.100.0/24",    // TEST-NET-2
		"203.0.113.0/24",     // TEST-NET-3
		"224.0.0.0/4",        // multicast
		"240.0.0.0/4",        // reserved, incl. limited broadcast
		"::/96",              // IPv4-compatible, embeds IPv4
		"::ffff:0:0:0/96",    // IPv4-translated, embeds IPv4
		"64:ff9b::/96",       // NAT64, can embed any IPv4
		"64:ff9b:1::/48",     // local-use NAT64
		"100::/64",           // discard-only
		"100:0:0:1::/64",     // dummy
		"2001::/23",          // IETF protocol assignments, incl. Teredo and ORCHID
		"2001:db8::/32",      // documentation
		"2002::/16",          // 6to4, embeds IPv4
		"2620:4f:8000::/48",  // AS112
		"3fff::/20",          // documentation
		"5f00::/16",          // SRv6 SIDs
		"fd00:ec2::254/128",  // AWS IMDS
		"fe80::/10",          // link-local
		"fec0::/10",          // site-local
		"ff00::/8",           // multicast
	} {
		ranges = append(ranges, netip.MustParsePrefix(cidr))
	}
	return ranges
}()

// classifyAddr reports ip's class, judging an IPv4-mapped address by its IPv4 payload.
func classifyAddr(ip netip.Addr) addrClass {
	ip = ip.Unmap()
	switch {
	case ip.Zone() != "" || !ip.IsLoopback() && slices.ContainsFunc(reservedRanges, func(p netip.Prefix) bool { return p.Contains(ip) }):
		return classReserved
	case ip.IsPrivate() || ip.IsLoopback() || cgnatRange.Contains(ip):
		return classRestricted
	}
	return classPublic
}
