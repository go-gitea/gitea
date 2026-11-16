// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package policy

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// matchList keeps a list of IPs and hostnames
type matchList struct {
	patterns           []domainRule
	ipv6List, ipv4List []prefixRule
	rejected           []string
}

type portRange struct {
	start uint16
	end   uint16
}

func (p *portRange) Contains(port uint16) bool {
	return port >= p.start && port <= p.end
}

type prefixRule struct {
	prefix     netip.Prefix
	portRanges []portRange
}

func (p *prefixRule) Contains(port netip.AddrPort) bool {
	return p.prefix.Contains(port.Addr()) && slices.ContainsFunc(p.portRanges, func(r portRange) bool {
		return r.Contains(port.Port())
	})
}

type domainRule struct {
	pattern    string
	portRanges []portRange
}

func (p *domainRule) Contains(hostname string, port uint16) bool {
	return matchDomain(p.pattern, hostname) && slices.ContainsFunc(p.portRanges, func(r portRange) bool {
		return r.Contains(port)
	})
}

const (
	AliasPrivate  = "private"
	AliasLoopback = "loopback"
)

var namedRanges = sync.OnceValue(func() map[string][]netip.Prefix {
	base := map[string][]string{
		// private ranges and CGNAT
		AliasPrivate:  {"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7", "100.64.0.0/10"},
		AliasLoopback: {"127.0.0.0/8", "::1/128"},
	}
	out := make(map[string][]netip.Prefix, 2)
	for name, ranges := range base {
		out[name] = make([]netip.Prefix, 0, len(ranges))
		for _, r := range ranges {
			out[name] = append(out[name], netip.MustParsePrefix(r))
		}
	}
	return out
})

type (
	AllowList struct{ matchList }
	BlockList struct{ matchList }
)

type listEntry interface {
	addTo(*matchList)
}

func (p prefixRule) addTo(m *matchList) {
	if p.prefix.Addr().Is4() {
		m.ipv4List = append(m.ipv4List, p)
	} else {
		m.ipv6List = append(m.ipv6List, p)
	}
}

func (p domainRule) addTo(m *matchList) { m.patterns = append(m.patterns, p) }

type aliasExpansion []prefixRule

func (r aliasExpansion) addTo(m *matchList) {
	for _, pr := range r {
		pr.addTo(m)
	}
}

func parseList(hostlist string, mode Mode, isBlocklist bool) (list matchList) {
	for entry := range strings.SplitSeq(hostlist, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		rule, err := parseRule(entry, mode, isBlocklist)
		if err != nil {
			list.rejected = append(list.rejected, err.Error())
			continue
		}
		rule.addTo(&list)
	}
	return list
}

func splitEntry(entry string) (target, portSpec string, err error) {
	if strings.HasPrefix(entry, "[") {
		end := strings.IndexByte(entry, ']')
		if end < 0 {
			return "", "", errors.New("missing closing bracket")
		}
		target = entry[1:end]
		if target == "" {
			return "", "", errors.New("empty host")
		}
		rest := entry[end+1:]
		if rest == "" {
			return target, "", nil
		}
		if !strings.HasPrefix(rest, ":") {
			return "", "", fmt.Errorf("unexpected %q after bracketed target", rest)
		}
		portSpec = rest[1:]
		if portSpec == "" {
			return "", "", errors.New("empty port")
		}
		return target, portSpec, nil
	}

	// check for host:port
	if strings.Count(entry, ":") == 1 {
		i := strings.IndexByte(entry, ':')
		target, portSpec = entry[:i], entry[i+1:]
		switch {
		case target == "":
			return "", "", fmt.Errorf("empty host: '%s'", entry)
		case portSpec == "":
			return "", "", fmt.Errorf("empty port: '%s'", entry)
		}
		return target, portSpec, nil
	}

	// Portless: patterns, IPs, CIDRs - and multi-colon forms
	return entry, "", nil
}

func parseRule(entry string, mode Mode, isBlocklist bool) (listEntry, error) {
	target, portSpec, err := splitEntry(entry)
	if err != nil {
		return nil, fmt.Errorf("failed to split entry %s: %w", entry, err)
	}
	portRanges, err := parsePortSpec(portSpec, mode, isBlocklist)
	if err != nil {
		return nil, fmt.Errorf("invalid port syntax on entry %s: %w", entry, err)
	}
	return classifyTarget(target, portRanges)
}

func classifyTarget(target string, ranges []portRange) (listEntry, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	target = strings.TrimSuffix(target, ".")

	if expanded, ok := newNamedRanges(target, ranges); ok {
		return expanded, nil
	}
	if prefix, err := newPrefixRule(target, ranges); !errors.Is(err, errNotIP) {
		return prefix, err
	}
	return newDomainRule(target, ranges)
}

var errNotIP = errors.New("not an IP address")

func newPrefixRule(target string, ranges []portRange) (prefixRule, error) {
	if strings.ContainsRune(target, '/') {
		prefix, err := netip.ParsePrefix(target)
		if err != nil {
			if strings.ContainsRune(target, ':') {
				return prefixRule{}, fmt.Errorf("invalid IPv6 CIDR %q (unbracketed IPv6 with port? use [addr] or [addr]:port): %w", target, err)
			}
			return prefixRule{}, fmt.Errorf("invalid CIDR %q: %w", target, err)
		}
		if prefix.Bits() == 0 {
			return prefixRule{}, fmt.Errorf("catch-all CIDR %q covers every address and is not allowed", target)
		}
		if masked := prefix.Masked(); masked != prefix {
			return prefixRule{}, fmt.Errorf("invalid CIDR %q: host bits must be zero, use %q", target, masked)
		}
		return prefixRule{prefix: prefix, portRanges: ranges}, nil
	}

	addr, addrErr := netip.ParseAddr(target)
	if addrErr == nil {
		if addr.Zone() != "" {
			return prefixRule{}, fmt.Errorf("invalid address %q: address zone is not dialable", target)
		}
		addr = addr.Unmap()
		return prefixRule{prefix: netip.PrefixFrom(addr, addr.BitLen()), portRanges: ranges}, nil
	}

	if !ipShaped(target) {
		return prefixRule{}, errNotIP
	}
	return prefixRule{}, fmt.Errorf("target %q looks like an IP address but is not a valid one: %w", target, addrErr)
}

func ipShaped(target string) bool {
	if strings.ContainsRune(target, ':') {
		return true
	}
	return strings.ContainsRune(target, '.') && !strings.ContainsFunc(target, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
}

// newNamedRanges expands a named range alias into one prefixRule per CIDR. ok
// is false when target is not an alias.
func newNamedRanges(target string, ranges []portRange) (aliasExpansion, bool) {
	prefixes, ok := namedRanges()[target]
	if !ok {
		return nil, false
	}
	expanded := make(aliasExpansion, len(prefixes))
	for i, prefix := range prefixes {
		expanded[i] = prefixRule{prefix: prefix, portRanges: ranges}
	}
	return expanded, true
}

// newDomainRule classifies a hostname pattern.
func newDomainRule(target string, ranges []portRange) (domainRule, error) {
	if target == "*" {
		return domainRule{}, fmt.Errorf("catch-all host pattern %q matches every host and is not allowed", target)
	}
	if target == "external" {
		return domainRule{}, errors.New(`the "external" builtin was replaced by EGRESS_MODE = lax`)
	}
	if !validDomainPattern(target) {
		return domainRule{}, fmt.Errorf("target %q is not an IP, CIDR, named range, or valid hostname pattern", target)
	}
	return domainRule{pattern: target, portRanges: ranges}, nil
}

// parsePortSpec parses a port spec: "" means the context default per defaultPorts, "*"
// all ports, otherwise a port, a "lo-hi" range, or a bracketed "[p|p-p|...]" set.
func parsePortSpec(spec string, mode Mode, isBlocklist bool) ([]portRange, error) {
	if spec == "" {
		return defaultPorts(mode, isBlocklist), nil
	}
	if spec == "*" {
		return []portRange{{start: 0, end: 65535}}, nil
	}
	if strings.HasPrefix(spec, "[") && strings.HasSuffix(spec, "]") {
		inner := spec[1 : len(spec)-1]
		if inner == "" {
			return nil, fmt.Errorf("empty port set %q", spec)
		}
		var ranges []portRange
		for item := range strings.SplitSeq(inner, "|") {
			r, err := parsePortItem(item)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, r)
		}
		return ranges, nil
	}
	r, err := parsePortItem(spec)
	if err != nil {
		return nil, err
	}
	return []portRange{r}, nil
}

// parsePortItem parses "p" or "lo-hi" with 1 <= lo <= hi <= 65535. The parts
// are trimmed, so a bracketed set may be spaced ("[80 | 443]").
func parsePortItem(item string) (portRange, error) {
	lo, hi, isRange := strings.Cut(item, "-")
	start, err := parsePort(strings.TrimSpace(lo))
	if err != nil {
		return portRange{}, err
	}
	end := start
	if isRange {
		end, err = parsePort(strings.TrimSpace(hi))
		if err != nil {
			return portRange{}, err
		}
	}
	if start > end {
		return portRange{}, fmt.Errorf("reversed port range %q", item)
	}
	return portRange{start: start, end: end}, nil
}

func parsePort(s string) (uint16, error) {
	p, err := strconv.ParseUint(s, 10, 16)
	if err != nil || p == 0 {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	return uint16(p), nil
}

// defaultPorts: a portless block entry covers every port, a portless allow entry covers every port in Lax mode and only the web ports in Strict.
func defaultPorts(mode Mode, isBlocklist bool) []portRange {
	if isBlocklist || mode == Lax {
		return []portRange{{start: 0, end: 65535}}
	}
	return []portRange{{start: 80, end: 80}, {start: 443, end: 443}}
}

func NewAllowList(hostList string, mode Mode) *AllowList {
	return &AllowList{parseList(hostList, mode, false)}
}

func NewBlockList(hostList string) *BlockList {
	return &BlockList{parseList(hostList, Strict, true)}
}

func (m *matchList) MatchHostname(host string, port uint16) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	host = strings.TrimSuffix(host, ".")

	for _, pattern := range m.patterns {
		if pattern.Contains(host, port) {
			return true
		}
	}
	return false
}

// matchDomain implements the domain-matching model of x/net/http/httpproxy
// (dot-anchored suffix), like curl's and Go's NO_PROXY:
//   - "example.com" matches that host and any subdomain, dot-anchored so
//     "notexample.com" never matches
//   - "*.example.com" and ".example.com" match only subdomains, apex
//     excluded — the two spellings are equivalent, mirroring httpproxy's normalization
func matchDomain(pattern, host string) bool {
	if strings.HasPrefix(pattern, "*.") || strings.HasPrefix(pattern, ".") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return pattern == host || strings.HasSuffix(host, "."+pattern)
}

// validDomainPattern reports whether p is a usable domain pattern: any glob
// metacharacter beyond the documented forms (?, character classes, backslash
// escapes, mid-pattern *) is rejected instead of silently never matching, and
// so is any non-ASCII rune, which byte-wise matching against a resolved
// hostname could never hit (IDN names must be given as punycode).
func validDomainPattern(p string) bool {
	if strings.HasPrefix(p, "*.") || strings.HasPrefix(p, ".") {
		p = p[1:]
	}
	if p == "" || strings.ContainsFunc(p, func(r rune) bool { return r >= utf8.RuneSelf }) {
		return false
	}
	return !strings.ContainsAny(p, " *?[]/:\\")
}

// Rejected returns the entries dropped at parse, so callers can log them at startup.
func (m *matchList) Rejected() []string {
	return slices.Clone(m.rejected)
}

// MatchIPAddr checks if the given IP is in the list.
func (m *matchList) MatchIPAddr(ip netip.AddrPort) bool {
	match := m.ipv4List
	if ip.Addr().Is6() {
		match = m.ipv6List
	}
	for _, prefix := range match {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func (m *matchList) IsEmpty() bool {
	return len(m.patterns) == 0 && len(m.ipv4List) == 0 && len(m.ipv6List) == 0
}

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

// classifyAddr reports the class of a canonical address.
func classifyAddr(ip netip.Addr) addrClass {
	switch {
	case ip.Zone() != "" || !ip.IsLoopback() && inRange(reservedRanges, ip):
		return classReserved
	case ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || cgnatRange.Contains(ip) || inRange(restrictedRanges, ip):
		return classRestricted
	}
	return classPublic
}

func inRange(p []netip.Prefix, ip netip.Addr) bool {
	return slices.ContainsFunc(p, func(p netip.Prefix) bool { return p.Contains(ip) })
}
