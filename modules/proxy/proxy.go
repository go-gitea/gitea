// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"net/http"
	"net/url"
	"os"
	"sync"

	"gitea.dev/modules/glob"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

var (
	onceGlobalProxy  sync.Once
	hostMatchers     []glob.Glob
	onceWebhookProxy sync.Once
	webhookMatchers  []glob.Glob
)

// GetProxyURL returns proxy url
func GetProxyURL() string {
	if !setting.Proxy.Enabled {
		return ""
	}

	if setting.Proxy.ProxyURL == "" {
		if os.Getenv("http_proxy") != "" {
			return os.Getenv("http_proxy")
		}
		return os.Getenv("https_proxy")
	}
	return setting.Proxy.ProxyURL
}

// Match return true if url needs to be proxied
func Match(u string) bool {
	if !setting.Proxy.Enabled {
		return false
	}

	// enforce do once
	Proxy()

	for _, v := range hostMatchers {
		if v.Match(u) {
			return true
		}
	}
	return false
}

// Proxy returns the system proxy
func Proxy() func(req *http.Request) (*url.URL, error) {
	if !setting.Proxy.Enabled {
		return nil
	}
	if setting.Proxy.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}

	onceGlobalProxy.Do(func() {
		for _, h := range setting.Proxy.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				return http.ProxyURL(setting.Proxy.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}

// WebHookProxy returns the webhook proxy, falling back to the system proxy if no webhook proxy is set
func WebHookProxy() func(req *http.Request) (*url.URL, error) {
	if setting.Webhook.ProxyURL == "" {
		return Proxy()
	}
	onceWebhookProxy.Do(func() {
		for _, h := range setting.Webhook.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				webhookMatchers = append(webhookMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})
	return func(req *http.Request) (*url.URL, error) {
		for _, v := range webhookMatchers {
			if v.Match(req.URL.Host) {
				return http.ProxyURL(setting.Webhook.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}
