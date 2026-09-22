package policy

import (
	"net/netip"
	"sync"
)

// cgnatRange is RFC 6598 carrier-grade NAT space.
var cgnatRange = netip.MustParsePrefix("100.64.0.0/10")

// isCGNAT reports whether ip is in cgnatRange.
func isCGNAT(ip netip.Addr) bool {
	return ip.Is4() && cgnatRange.Contains(ip)
}

// addrClass is the intrinsic class of a dial target, before any list lookup.
type addrClass uint8

const (
	classPublic addrClass = iota
	// classRestricted is private/loopback/CGNAT: reachable only via an allow entry.
	classRestricted
	// classReserved is never dialable.
	classReserved
)

// classifyAddr reports ip's class. A zone is never dialable, and an IPv4-mapped (::ffff)
// address is judged as its v4 payload.
func classifyAddr(ip netip.Addr) addrClass {
	if ip.Zone() != "" {
		return classReserved
	}
	if ip.Is4In6() {
		ip = ip.Unmap()
	}
	if ipInReservedRange(ip) {
		return classReserved
	}
	if ip.IsPrivate() || ip.IsLoopback() || isCGNAT(ip) {
		return classRestricted
	}
	return classPublic
}

// ipInReservedRange reports whether a normalized ip is in the never-dialable set.
func ipInReservedRange(ip netip.Addr) bool {
	if ip.Is4() {
		for _, cidr := range reservedV4Ranges() {
			if cidr.Contains(ip) {
				return true
			}
		}
		return false
	}
	for _, cidr := range reservedV6Ranges() {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// reservedV4Ranges / reservedV6Ranges are the two halves of the never-dialable set.
var reservedV4Ranges = sync.OnceValue(func() []netip.Prefix {
	return parseRanges([]string{
		"0.0.0.0/8",        // RFC 791 "this network" - no dial target is ever usable here
		"168.63.129.16/32", // Azure WireServer metadata endpoint
		"169.254.0.0/16",   // RFC 3927 link-local - every major cloud's IMDS lives here (AWS/GCP/Azure .169.254, ECS .170.2); rarely routed even internally
		"192.0.0.0/24",     // RFC 6890 IETF protocol assignments
		"192.0.2.0/24",     // RFC 5737 TEST-NET-1
		"192.88.99.0/24",   // RFC 7526 6to4 relay anycast (deprecated)
		"198.18.0.0/15",    // RFC 2544 benchmarking
		"198.51.100.0/24",  // RFC 5737 TEST-NET-2
		"203.0.113.0/24",   // RFC 5737 TEST-NET-3
		"224.0.0.0/4",      // RFC 1112 multicast - no dial target (also class D)
		"240.0.0.0/4",      // RFC 1112 reserved for future use (class E, incl. limited broadcast)
	})
})

var reservedV6Ranges = sync.OnceValue(func() []netip.Prefix {
	return parseRanges([]string{
		"::ffff:0:0:0/96",   // RFC 2765/6145 IPv4-translated
		"::/128",            // RFC 4291 IPv6 unspecified
		"100::/64",          // RFC 6666 discard-only
		"2001::/32",         // RFC 4380 Teredo tunneling (embeds IPv4)
		"2001:2::/48",       // RFC 5180 benchmarking (IPv6 twin of 198.18.0.0/15)
		"2001:10::/28",      // RFC 4843 ORCHID (deprecated)
		"2001:20::/28",      // RFC 7343 ORCHIDv2
		"2001:db8::/32",     // RFC 3849 documentation
		"2002::/16",         // RFC 3056 6to4 (embeds IPv4)
		"3ffe::/20",         // RFC 3701 6bone testbed - deprecated 2004, returned to IANA
		"64:ff9b::/96",      // RFC 6052 NAT64 well-known prefix - reserved wholesale, no payload decoding
		"64:ff9b:1::/48",    // RFC 8215 NAT64 local-use prefix - ditto (needs DNS64 + runtime checks to be useful)
		"fd00:ec2::254/128", // AWS IMDS IPv6-only endpoint - inside ULA (which stays restricted)
		"fe80::/10",         // RFC 4291 link-local - zoned forms were already reserved up front
		"fec0::/10",         // RFC 3879 deprecated site-local - never a dial target
		"ff00::/8",          // RFC 4291 IPv6 multicast - no dial target
	})
})

func parseRanges(cidrs []string) []netip.Prefix {
	var ranges []netip.Prefix
	for _, cidr := range cidrs {
		subnet, err := netip.ParsePrefix(cidr)
		if err != nil {
			panic("egress: invalid reserved CIDR " + cidr + ": " + err.Error())
		}
		ranges = append(ranges, subnet)
	}
	return ranges
}
