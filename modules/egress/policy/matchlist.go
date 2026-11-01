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
	AliasCGNAT    = "cgnat"
)

var namedRanges = sync.OnceValue(func() map[string][]netip.Prefix {
	base := map[string][]string{
		// private ranges, CGNAT is its own alias so "private" never covers the carrier-NAT metadata endpoint
		AliasPrivate:  {"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7"},
		AliasLoopback: {"127.0.0.0/8", "::1/128"},
		AliasCGNAT:    {"100.64.0.0/10"}, // carrier NAT, eg Tailscale
	}
	out := make(map[string][]netip.Prefix, len(base))
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

var (
	errNotIP    = errors.New("not an IP address")
	errCatchAll = errors.New("catch-all CIDR covers every address and is not allowed")
)

func newPrefixRule(target string, ranges []portRange) (prefixRule, error) {
	if strings.ContainsRune(target, '/') {
		prefix, err := netip.ParsePrefix(target)
		if err != nil {
			if strings.ContainsRune(target, ':') {
				return prefixRule{}, fmt.Errorf("invalid IPv6 CIDR %q (unbracketed IPv6 with port? use [addr] or [addr]:port): %w", target, err)
			}
			return prefixRule{}, fmt.Errorf("invalid CIDR %q: %w", target, err)
		}
		if prefix.Addr().Is4In6() {
			// only ::ffff:0:0/96 is IPv4-mapped: normalize to the IPv4 form, ::ffff:10.0.0.0/104 -> 10.0.0.0/8
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), max(prefix.Bits()-96, 0))
		}
		if prefix.Bits() == 0 {
			return prefixRule{}, fmt.Errorf("invalid CIDR %q: %w", target, errCatchAll)
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

// ParseInternalRanges parses a comma-separated list of CIDRs, IPs and named ranges into
// prefixes, returning the entries it rejects so callers can log them at startup.
func ParseInternalRanges(hostList string) (prefixes []netip.Prefix, rejected []string) {
	rejectPorts := func(entry string) {
		rejected = append(rejected, fmt.Sprintf("invalid range %q: ports are not supported, a range covers every port (ports come from the allow entries)", entry))
	}
	for entry := range strings.SplitSeq(hostList, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		target, portSpec, err := splitEntry(entry)
		if err != nil {
			rejected = append(rejected, fmt.Sprintf("invalid range %q: %v", entry, err))
			continue
		}
		if portSpec != "" {
			rejectPorts(entry)
			continue
		}
		if expanded, ok := newNamedRanges(target, nil); ok {
			for _, rule := range expanded {
				prefixes = append(prefixes, rule.prefix)
			}
			continue
		}
		rule, err := newPrefixRule(target, nil)
		if err != nil {
			switch {
			case errors.Is(err, errNotIP):
				rejected = append(rejected, fmt.Sprintf("invalid range %q: not an IP, CIDR, or named range", entry))
			case errors.Is(err, errCatchAll):
				rejected = append(rejected, fmt.Sprintf("invalid range %q: %v", entry, err))
			case strings.Contains(target, "/") && strings.Contains(target, ":"):
				rejectPorts(entry) // an unbracketed IPv6 CIDR with a port
			default:
				rejected = append(rejected, err.Error())
			}
			continue
		}
		prefixes = append(prefixes, rule.prefix) // newPrefixRule already normalized IPv4-mapped CIDRs
	}
	return prefixes, rejected
}

// normalizeHost lowercases a host, strips a port suffix and a trailing dot
func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		host = hostname
	}
	return strings.TrimSuffix(host, ".")
}

// matchEntry returns the first hostname entry for which pred matches, scanning the
// patterns once — the single matching loop behind all hostname matchers.
func (m *matchList) matchEntry(host string, pred func(rule domainRule, host string) bool) (domainRule, bool) {
	host = normalizeHost(host)
	for _, rule := range m.patterns {
		if pred(rule, host) {
			return rule, true
		}
	}
	return domainRule{}, false
}

func (m *matchList) MatchHostname(host string, port uint16) bool {
	_, ok := m.MatchHostnameEntry(host, port)
	return ok
}

// MatchHostnameEntry returns the hostname entry matching host, if any
func (m *matchList) MatchHostnameEntry(host string, port uint16) (string, bool) {
	rule, ok := m.matchHostnameRule(host, port)
	return rule.pattern, ok
}

// matchHostnameRule returns the first hostname entry matching host on the given port
func (m *matchList) matchHostnameRule(host string, port uint16) (domainRule, bool) {
	return m.matchEntry(host, func(r domainRule, h string) bool { return r.Contains(h, port) })
}

// matchHostEntry returns the first hostname entry matching host, ignoring ports
func (m *matchList) matchHostEntry(host string) (domainRule, bool) {
	return m.matchEntry(host, func(r domainRule, h string) bool { return matchDomain(r.pattern, h) })
}

// matchDomain matches a hostname entry, like CSP source lists, CORS origins and TLS
// wildcards: a bare entry matches the exact host only, a "*." or "." prefixed entry
// matches subdomains only, the apex excluded — the two spellings are equivalent.
// List both forms for the host and its subdomains.
func matchDomain(pattern, host string) bool {
	if strings.HasPrefix(pattern, "*.") || strings.HasPrefix(pattern, ".") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	return pattern == host
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

// NeverMatching returns the IP entries that can never match a target, so callers can warn about them at startup.
// IPv4-mapped CIDRs are normalized to the IPv4 form at parse time, so they always match.
func (m *matchList) NeverMatching() []string {
	var out []string
	for _, rule := range slices.Concat(m.ipv4List, m.ipv6List) {
		if fullyReserved(rule.prefix) {
			out = append(out, fmt.Sprintf("%s is a reserved range and is never dialable", rule.prefix))
		}
	}
	return out
}

// fullyReserved reports whether every address of the prefix is reserved, so the entry never matches.
// A partially reserved prefix (e.g. 0.0.0.0/7, which includes dialable 1.0.0.1) still matches and is not flagged.
func fullyReserved(prefix netip.Prefix) bool {
	// classifyAddr exempts loopback from the reserved pool, so a prefix overlapping loopback
	// (e.g. the loopback alias: 127.0.0.0/8, ::1/128) still matches and is not flagged
	for _, loopback := range []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")} {
		if prefix.Overlaps(loopback) {
			return false
		}
	}
	for _, reserved := range reservedRanges {
		if reserved.Contains(prefix.Addr()) && prefix.Bits() >= reserved.Bits() {
			return true
		}
	}
	return false
}

// MatchIPAddr checks if the given IP is in the list.
func (m *matchList) MatchIPAddr(ip netip.AddrPort) bool {
	_, ok := m.MatchIPAddrEntry(ip)
	return ok
}

// MatchIPAddrEntry returns the IP entry matching ip, if any
func (m *matchList) MatchIPAddrEntry(ip netip.AddrPort) (netip.Prefix, bool) {
	match := m.ipv4List
	if ip.Addr().Is6() {
		match = m.ipv6List
	}
	for _, prefix := range match {
		if prefix.Contains(ip) {
			return prefix.prefix, true
		}
	}
	return netip.Prefix{}, false
}

func (m *matchList) IsEmpty() bool {
	return len(m.patterns) == 0 && len(m.ipv4List) == 0 && len(m.ipv6List) == 0
}
