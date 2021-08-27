// Copyright 2020 Lauris BH. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package proxy

import (
	"net"
)

// ForwardedHeadersOptions represents options for forwarded header middleware
type ForwardedHeadersOptions struct {
	// ForwardLimit limits the number of entries in the headers that will be processed.
	// The default value is 1. Set to 0 to disable the limit.
	ForwardLimit int
	// TrustingAllProxies option sets to trust all proxies.
	TrustingAllProxies bool
	// KnownProxies represents addresses of trusted proxies.
	TrustedProxies []net.IP
	// TrustedNetworks represents addresses of trusted networks.
	TrustedNetworks []*net.IPNet
}

var defaultOptions = &ForwardedHeadersOptions{
	ForwardLimit: 1,
	TrustedProxies: []net.IP{
		net.IPv4(127, 0, 0, 1),
	},
}

// NewForwardedHeadersOptions creates new middleware options
func NewForwardedHeadersOptions() *ForwardedHeadersOptions {
	return &ForwardedHeadersOptions{
		ForwardLimit:    defaultOptions.ForwardLimit,
		TrustedProxies:  defaultOptions.TrustedProxies,
		TrustedNetworks: defaultOptions.TrustedNetworks,
	}
}

// WithForwardLimit sets number of entries to be processed
func (opts *ForwardedHeadersOptions) WithForwardLimit(limit int) *ForwardedHeadersOptions {
	opts.ForwardLimit = limit
	return opts
}

// TrustAllProxies sets to trust all proxies
func (opts *ForwardedHeadersOptions) TrustAllProxies() *ForwardedHeadersOptions {
	opts.TrustingAllProxies = true
	return opts
}

// ClearTrustedProxies clears trusted proxy list
func (opts *ForwardedHeadersOptions) ClearTrustedProxies() *ForwardedHeadersOptions {
	opts.TrustingAllProxies = false
	opts.TrustedProxies = make([]net.IP, 0)
	return opts
}

// AddTrustedProxy adds proxy IP to trusted proxy list
func (opts *ForwardedHeadersOptions) AddTrustedProxy(ip string) *ForwardedHeadersOptions {
	// Special option to trust all proxies if IP address is set as wildcard
	if ip == "*" {
		opts.TrustingAllProxies = true
		return opts
	}

	ipaddr := net.ParseIP(ip)
	if ipaddr == nil {
		return opts
	}

	opts.TrustedProxies = append(opts.TrustedProxies, ipaddr)
	return opts
}

// ClearTrustedNetworks clears trusted network list
func (opts *ForwardedHeadersOptions) ClearTrustedNetworks() *ForwardedHeadersOptions {
	opts.TrustedNetworks = make([]*net.IPNet, 0)
	return opts
}

// AddTrustedNetwork adds network to trusted network list
func (opts *ForwardedHeadersOptions) AddTrustedNetwork(cidr string) *ForwardedHeadersOptions {
	_, netmask, err := net.ParseCIDR(cidr)
	if err != nil || netmask == nil {
		return opts
	}

	opts.TrustedNetworks = append(opts.TrustedNetworks, netmask)
	return opts
}

func (opts *ForwardedHeadersOptions) isTrustedProxy(ip net.IP) bool {
	if opts.TrustingAllProxies {
		return true
	}

	if ip == nil {
		return false
	}

	for _, tip := range opts.TrustedProxies {
		if tip.Equal(ip) {
			return true
		}
	}

	for _, tnet := range opts.TrustedNetworks {
		if tnet.Contains(ip) {
			return true
		}
	}

	return false
}
