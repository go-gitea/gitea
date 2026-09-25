// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/log"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"

	"golang.org/x/net/http/httpproxy"
)

func NewMigrationPolicy() *policy.Policy {
	return newMigrationPolicy(proxy.Proxy())
}

// NewGitPolicy is the migration policy for the git proxy, which keeps git's own proxy choice
func NewGitPolicy() *policy.Policy {
	return newMigrationPolicy(gitProxySelector())
}

// gitProxySelector picks proxies like git did: [git.config] http.proxy, else a [proxy] PROXY_URL, else the environment incl. ALL_PROXY
func gitProxySelector() func(*http.Request) (*url.URL, error) {
	env := httpproxy.FromEnvironment()
	if rawURL, ok := setting.GitConfig.Options["http.proxy"]; ok {
		gitProxy, err := normalizeGitProxy(rawURL)
		if err == nil {
			env.HTTPProxy, env.HTTPSProxy = gitProxy, gitProxy
			return requestProxy(env)
		}
		log.Error("Ignoring [git.config] http.proxy: %v", err)
	}
	if setting.Proxy.Enabled && setting.Proxy.ProxyURL != "" {
		return proxy.Proxy()
	}
	allProxy := cmp.Or(os.Getenv("all_proxy"), os.Getenv("ALL_PROXY"))
	env.HTTPProxy, env.HTTPSProxy = cmp.Or(env.HTTPProxy, allProxy), cmp.Or(env.HTTPSProxy, allProxy)
	return requestProxy(env)
}

func requestProxy(cfg *httpproxy.Config) func(*http.Request) (*url.URL, error) {
	proxyFunc := cfg.ProxyFunc()
	return func(req *http.Request) (*url.URL, error) { return proxyFunc(req.URL) }
}

// normalizeGitProxy reads a proxy URL the way git reads http.proxy: http is the default scheme, 1080 curl's default port
func normalizeGitProxy(rawURL string) (string, error) {
	if rawURL == "" {
		return "", nil
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "http://" + rawURL
	}
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("invalid URL") // the parse error would echo its credentials
	}
	if !slices.Contains([]string{"http", "https", "socks5", "socks5h"}, proxyURL.Scheme) {
		return "", fmt.Errorf("unsupported scheme %q", proxyURL.Scheme)
	}
	if proxyURL.Scheme == "http" && proxyURL.Port() == "" {
		proxyURL.Host = net.JoinHostPort(proxyURL.Hostname(), "1080")
	}
	return proxyURL.String(), nil
}

func newMigrationPolicy(selectProxy func(*http.Request) (*url.URL, error)) *policy.Policy {
	return policy.NewPolicy("migrations",
		policy.WithAllow(setting.Migrations.AllowedHostList, "migrations.ALLOWED_HOST_LIST"),
		policy.WithBlock(setting.Migrations.BlockedHostList, "migrations.BLOCKED_HOST_LIST"),
		policy.WithLocalNeedsIPAllow(),
		policy.WithProxy(selectProxy))
}

func NewWebhookPolicy() *policy.Policy {
	var p *policy.Policy
	selectProxy := proxy.WebHookProxy()
	if webhookProxy := setting.Webhook.ProxyURLFixed; webhookProxy != nil {
		next := selectProxy
		selectProxy = func(req *http.Request) (*url.URL, error) {
			u, err := next(req)
			if err == nil && u == webhookProxy {
				err = p.CheckHost(req.URL.Hostname()) // the webhook proxy resolves the target, so only its name can be checked
			}
			return u, err
		}
	}
	p = policy.NewPolicy("webhook",
		policy.WithAllow(setting.Webhook.AllowedHostList, "security.ALLOWED_HOST_LIST"),
		policy.WithProxy(selectProxy))
	return p
}

func NewSecurityPolicy(usage string) *policy.Policy {
	return policy.NewPolicy(usage,
		policy.WithAllow(setting.Security.AllowedHostList, "security.ALLOWED_HOST_LIST"),
		policy.WithProxy(proxy.Proxy()))
}
