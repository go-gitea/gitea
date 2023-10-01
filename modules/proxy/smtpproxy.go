// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bufio"
	"code.gitea.io/gitea/modules/setting"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"net/url"
)

// SMTPConn create socks5 or https proxy
func SMTPConn(network string, addr string) (net.Conn, error) {
	cfg := setting.Proxy
	if !cfg.Enabled || !cfg.SMTPProxyEnabled {
		return proxy.Direct.Dial("tcp", addr)
	}

	var (
		proxyURL *url.URL
		err      error
	)

	req, err := http.NewRequest("GET", "https://"+addr, &bufio.Reader{})
	if err != nil {
		return nil, err
	}

	proxyURL, err = Proxy()(req)
	if err != nil {
		return nil, err
	}

	if proxyURL == nil {
		return net.Dial("tcp", addr)
	}

	if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks" {
		var auth *proxy.Auth
		if proxyURL.User != nil {
			auth = &proxy.Auth{User: proxyURL.User.Username(), Password: proxyURL.User.Username()}
		}
		d, err := proxy.SOCKS5(network, proxyURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}
		return d.Dial(network, addr)
	}

	hp, err := newHTTPProxy(proxyURL, proxy.Direct)
	if err != nil {
		return nil, err
	}

	return hp.Dial("tcp", addr)
}
