// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"net/http"
	"net/url"
	"sync"

	"gitea.dev/modules/glob"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

var (
	globalProxyHosts  = sync.OnceValue(func() []glob.Glob { return compileHosts(setting.Proxy.ProxyHosts) })
	webhookProxyHosts = sync.OnceValue(func() []glob.Glob { return compileHosts(setting.Webhook.ProxyHosts) })
)

func compileHosts(hosts []string) (globs []glob.Glob) {
	for _, h := range hosts {
		if g, err := glob.Compile(h); err == nil {
			globs = append(globs, g)
		} else {
			log.Error("glob.Compile %s failed: %v", h, err)
		}
	}
	return globs
}

// hostsProxy sends requests for hosts matching globs through proxyURL, others follow the environment
func hostsProxy(globs []glob.Glob, proxyURL *url.URL) func(req *http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		for _, g := range globs {
			if g.Match(req.URL.Host) {
				return proxyURL, nil
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}

// Proxy returns the system proxy
func Proxy() func(req *http.Request) (*url.URL, error) {
	if !setting.Proxy.Enabled {
		return nil
	}
	if setting.Proxy.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}
	return hostsProxy(globalProxyHosts(), setting.Proxy.ProxyURLFixed)
}

// WebHookProxy returns the webhook proxy, falling back to the system proxy if no webhook proxy is set
func WebHookProxy() func(req *http.Request) (*url.URL, error) {
	if setting.Webhook.ProxyURL == "" {
		return Proxy()
	}
	return hostsProxy(webhookProxyHosts(), setting.Webhook.ProxyURLFixed)
}
