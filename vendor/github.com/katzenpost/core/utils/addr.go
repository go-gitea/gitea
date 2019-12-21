// addr.go - Address validation and utilities.
// Copyright (C) 2017  Yawning Angel.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"fmt"
	"net"
)

var unsuitableNetworks []*net.IPNet

// EnsureAddrIPPort returns nil iff the address is a raw IP + Port combination.
func EnsureAddrIPPort(a string) error {
	host, _, err := net.SplitHostPort(a)
	if err != nil {
		return err
	}
	if net.ParseIP(host) == nil {
		return fmt.Errorf("address '%v' is not an IP", host)
	}
	return nil
}

// GetExternalIPv4Address attempts to guess an external IPv4 address by
// interface enumeration.
func GetExternalIPv4Address() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

addrLoop:
	for _, addr := range addrs {
		if addr.Network() != "ip+net" {
			continue
		}

		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			return nil, err
		}

		for _, n := range unsuitableNetworks {
			if n.Contains(ip) {
				continue addrLoop
			}
		}

		if ip.To4() == nil {
			continue
		}

		return ip, nil
	}

	return nil, fmt.Errorf("no globally routable IPv4 addresses found")
}

func init() {
	for _, v := range []string{
		// Loopback addresses.
		"127.0.0.0/8",
		"::1/128",

		// Local addresses.
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",

		// Link-local addresses.
		"169.254.0.0/16",
		"fe80::/10",

		// Oddities.
		"::ffff:0:0/96", // IPv4 mapped addresses
		"64:ff9b::/96",  // IPv4/IPv6 translation

		// TODO: There's more things that could be on here.
	} {
		_, n, err := net.ParseCIDR(v)
		if err != nil {
			panic("BUG: Failed to build unsuitable address list: " + err.Error())
		}
		unsuitableNetworks = append(unsuitableNetworks, n)
	}
}
