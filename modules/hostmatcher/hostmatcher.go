// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package hostmatcher

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// HostMatchList is used to check if a host or IP is in a list.
// If you only need to do wildcard matching, consider to use modules/matchlist
type HostMatchList struct {
	SettingKeyHint string
	SettingValue   string

	// host name or built-in network name
	names  []string
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
func ParseHostMatchList(settingKeyHint string, hostList string) *HostMatchList {
	hl := &HostMatchList{SettingKeyHint: settingKeyHint, SettingValue: hostList}
	for _, s := range strings.Split(hostList, ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(s)
		if err == nil {
			hl.ipNets = append(hl.ipNets, ipNet)
		} else {
			hl.names = append(hl.names, s)
		}
	}
	return hl
}

// ParseSimpleMatchList parse a simple matchlist (no built-in networks, no CIDR support)
func ParseSimpleMatchList(settingKeyHint string, matchList string, includeLocalNetwork bool) *HostMatchList {
	hl := &HostMatchList{
		SettingKeyHint: settingKeyHint,
		SettingValue:   matchList + fmt.Sprintf("(local-network:%v)", includeLocalNetwork),
	}
	for _, s := range strings.Split(matchList, ",") {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if s == MatchBuiltinLoopback || s == MatchBuiltinPrivate || s == MatchBuiltinExternal {
			// for built-in names, we convert it from "private" => "[p]rivate" for internal usage and keep the same result as `matchlist`
			hl.names = append(hl.names, "["+s[:1]+"]"+s[1:])
		} else {
			// we keep the same result as `matchlist`, so no CIDR support here
			hl.names = append(hl.names, s)
		}
	}
	if includeLocalNetwork {
		hl.names = append(hl.names, MatchBuiltinPrivate)
	}
	return hl
}

// IsEmpty checks if the check list is empty
func (hl *HostMatchList) IsEmpty() bool {
	return hl == nil || (len(hl.names) == 0 && len(hl.ipNets) == 0)
}

func (hl *HostMatchList) checkNames(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, name := range hl.names {
		switch name {
		case "":
		case MatchBuiltinExternal:
		case MatchBuiltinPrivate:
		case MatchBuiltinLoopback:
			// ignore empty string or built-in network names
			continue
		case MatchBuiltinAll:
			return true
		default:
			if matched, _ := filepath.Match(name, host); matched {
				return true
			}
		}
	}
	return false
}

func (hl *HostMatchList) checkIP(ip net.IP) bool {
	for _, name := range hl.names {
		switch name {
		case "":
			continue
		case MatchBuiltinAll:
			return true
		case MatchBuiltinExternal:
			if ip.IsGlobalUnicast() && !util.IsIPPrivate(ip) {
				return true
			}
		case MatchBuiltinPrivate:
			if util.IsIPPrivate(ip) {
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
	if hl.checkNames(host) {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return hl.checkIP(ip)
	}
	return false
}

// MatchIPAddr checks if the IP matches an allow/deny(block) list, it's safe to pass `nil` to `ip`
func (hl *HostMatchList) MatchIPAddr(ip net.IP) bool {
	if hl == nil {
		return false
	}
	host := ip.String() // nil-safe, we will get "<nil>" if ip is nil
	return hl.checkNames(host) || hl.checkIP(ip)
}

// MatchHostOrIP checks if the host or IP matches an allow/deny(block) list
func (hl *HostMatchList) MatchHostOrIP(host string, ip net.IP) bool {
	return hl.MatchHostName(host) || hl.MatchIPAddr(ip)
}
