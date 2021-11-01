// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hostmatcher

import (
	"net"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// HostMatchList is used to check if a host or IP is in a list.
// If you only need to do wildcard matching, consider to use modules/matchlist
type HostMatchList struct {
	hosts  []string
	ipNets []*net.IPNet
}

// MatchBuiltinAll all hosts are matched
const MatchBuiltinAll = "*"

// MatchBuiltinExternal A valid non-private unicast IP, all hosts on public internet are matched
const MatchBuiltinExternal = "external"

// MatchBuiltinPrivate RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) and RFC 4193 (FC00::/7). Also called LAN/Intranet.
const MatchBuiltinPrivate = "private"

// MatchBuiltinLoopback 127.0.0.0/8 for IPv4 and ::1/128 for IPv6, localhost is included.
const MatchBuiltinLoopback = "loopback"

// ParseHostMatchList parses the host list HostMatchList
func ParseHostMatchList(hostList string) *HostMatchList {
	hl := &HostMatchList{}
	for _, s := range strings.Split(hostList, ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(s)
		if err == nil {
			hl.ipNets = append(hl.ipNets, ipNet)
		} else {
			hl.hosts = append(hl.hosts, s)
		}
	}
	return hl
}

// MatchesHostOrIP checks if the host or IP matches an allow/deny(block) list
func (hl *HostMatchList) MatchesHostOrIP(host string, ip net.IP) bool {
	var matched bool
	host = strings.ToLower(host)
	ipStr := ip.String()
loop:
	for _, hostInList := range hl.hosts {
		switch hostInList {
		case "":
			continue
		case MatchBuiltinAll:
			matched = true
			break loop
		case MatchBuiltinExternal:
			if matched = ip.IsGlobalUnicast() && !util.IsIPPrivate(ip); matched {
				break loop
			}
		case MatchBuiltinPrivate:
			if matched = util.IsIPPrivate(ip); matched {
				break loop
			}
		case MatchBuiltinLoopback:
			if matched = ip.IsLoopback(); matched {
				break loop
			}
		default:
			if matched, _ = filepath.Match(hostInList, host); matched {
				break loop
			}
			if matched, _ = filepath.Match(hostInList, ipStr); matched {
				break loop
			}
		}
	}
	if !matched {
		for _, ipNet := range hl.ipNets {
			if matched = ipNet.Contains(ip); matched {
				break
			}
		}
	}
	return matched
}
