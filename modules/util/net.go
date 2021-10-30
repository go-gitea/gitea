// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net"
	"path/filepath"
	"strings"
)

//HostListBuiltinAll all hosts are matched
const HostListBuiltinAll = "all"

//HostListBuiltinExternal A valid non-private unicast IP, all hosts on public internet are matched
const HostListBuiltinExternal = "external"

//HostListBuiltinPrivate RFC 1918 (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) and RFC 4193 (FC00::/7). Also called LAN/Intranet.
const HostListBuiltinPrivate = "private"

//HostListBuiltinLoopback 127.0.0.0/8 for IPv4 and ::1/128 for IPv6, localhost is included.
const HostListBuiltinLoopback = "loopback"

// IsIPPrivate for net.IP.IsPrivate. TODO: replace with `ip.IsPrivate()` if min go version is bumped to 1.17
func IsIPPrivate(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 ||
			(ip4[0] == 172 && ip4[1]&0xf0 == 16) ||
			(ip4[0] == 192 && ip4[1] == 168)
	}
	return len(ip) == net.IPv6len && ip[0]&0xfe == 0xfc
}

// ParseHostMatchList parses the host list for HostOrIPMatchesList
func ParseHostMatchList(hostListStr string) (hostList []string, ipNets []*net.IPNet) {
	for _, s := range strings.Split(hostListStr, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(s)
		if err == nil {
			ipNets = append(ipNets, ipNet)
		} else {
			hostList = append(hostList, s)
		}
	}
	return
}

// HostOrIPMatchesList checks if the host or IP matches an allow/deny(block) list
func HostOrIPMatchesList(host string, ip net.IP, hostList []string, ipNets []*net.IPNet) bool {
	var matched bool
	ipStr := ip.String()
loop:
	for _, hostInList := range hostList {
		switch hostInList {
		case "":
			continue
		case HostListBuiltinAll:
			matched = true
			break loop
		case HostListBuiltinExternal:
			if matched = ip.IsGlobalUnicast() && !IsIPPrivate(ip); matched {
				break loop
			}
		case HostListBuiltinPrivate:
			if matched = IsIPPrivate(ip); matched {
				break loop
			}
		case HostListBuiltinLoopback:
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
		for _, ipNet := range ipNets {
			if matched = ipNet.Contains(ip); matched {
				break
			}
		}
	}
	return matched
}
