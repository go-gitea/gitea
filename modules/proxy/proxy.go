// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/glob"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

var (
	once         sync.Once
	hostMatchers []glob.Glob
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
		return func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
	if setting.Proxy.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}

	once.Do(func() {
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

// EnvWithProxy returns os.Environ(), with a https_proxy env, if the given url
// needs to be proxied.
func EnvWithProxy(u *url.URL) []string {
	envs := os.Environ()
	if strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https") {
		if Match(u.Host) {
			envs = append(envs, "https_proxy="+GetProxyURL())
		}
	}

	return envs
}
