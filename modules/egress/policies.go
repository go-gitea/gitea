// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

import (
	"net/http"
	"net/url"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
)

func NewMigrationPolicy() *policy.Policy {
	return newMigrationPolicy(proxy.Proxy())
}

// NewGitPolicy is the migration policy for the git proxy, git used the environment's proxy even with [proxy] disabled
func NewGitPolicy() *policy.Policy {
	selectProxy := proxy.Proxy()
	if selectProxy == nil {
		selectProxy = http.ProxyFromEnvironment
	}
	return newMigrationPolicy(selectProxy)
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
