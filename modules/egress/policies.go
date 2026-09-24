package egress

import (
	"strings"
	"sync"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/proxy"
	"gitea.dev/modules/setting"
)

var (
	migrationPolicy    = sync.OnceValue(newMigrationPolicy)
	gitPolicy          = sync.OnceValue(newGitPolicy)
	oauth2AvatarPolicy = sync.OnceValue(newOauth2AvatarPolicy)
	openIDPolicy       = sync.OnceValue(newOpenIDPolicy)
	webhookPolicy      = sync.OnceValue(newWebhookPolicy)
)

func newMigrationPolicy() *policy.Policy {
	return commonGitPolicy("migrations")
}

func newGitPolicy() *policy.Policy {
	return commonGitPolicy("git-proxy")
}

func commonGitPolicy(name string) *policy.Policy {
	allow, block := setting.Migrations.AllowedHostList, setting.Migrations.DeniedHostList
	if strings.TrimSpace(allow) == "" {
		// an empty allow list means "any external host", matching the historical default
		allow = "external"
	}
	if setting.Migrations.AllowLocalNetworks {
		allow = joinHostList(allow, "private", "loopback")
	} else {
		block = joinHostList(block, "private", "loopback")
	}

	return policy.NewPolicy(name,
		policy.WithAllow(allow, "migrations.ALLOWED_HOST_LIST"),
		policy.WithBlock(block, "migrations.BLOCKED_HOST_LIST"),
		policy.WithProxy(setting.Proxy.ProxyURLFixed, proxy.Proxy()))
}

func newOauth2AvatarPolicy() *policy.Policy {
	return policy.NewPolicy("oauth2-avatar",
		policy.WithAllow(setting.Security.AllowedHostList, "security.ALLOWED_HOST_LIST"),
		policy.WithProxy(setting.Proxy.ProxyURLFixed, proxy.Proxy()))
}

func newOpenIDPolicy() *policy.Policy {
	return policy.NewPolicy("openid",
		policy.WithAllow(setting.Security.AllowedHostList, "security.ALLOWED_HOST_LIST"),
		policy.WithProxy(setting.Proxy.ProxyURLFixed, proxy.Proxy()))
}

func newWebhookPolicy() *policy.Policy {
	return policy.NewPolicy("webhook",
		policy.WithAllow(setting.Webhook.AllowedHostList, "security.ALLOWED_HOST_LIST"),
		policy.WithProxy(setting.Webhook.ProxyURLFixed, proxy.WebHookProxy()),
		policy.WithProxyPreScreen(true)) // webhook had  prescreening in its handler so keeping it for now.
}

func GetOpenIDPolicy() *policy.Policy {
	return openIDPolicy()
}

func GetOauth2AvatarPolicy() *policy.Policy {
	return oauth2AvatarPolicy()
}

// GetMigrationPolicy returns the policy for migrations
// It must be called after loading settings
func GetMigrationPolicy() *policy.Policy {
	return migrationPolicy()
}

func GetGitPolicy() *policy.Policy {
	return gitPolicy()
}

func GetWebhookPolicy() *policy.Policy {
	return webhookPolicy()
}

// joinHostList appends entries to a comma-separated list, skipping an empty base.
func joinHostList(list string, entries ...string) string {
	parts := make([]string, 0, len(entries)+1)
	if list = strings.TrimSpace(list); list != "" {
		parts = append(parts, list)
	}
	parts = append(parts, entries...)
	return strings.Join(parts, ", ")
}
