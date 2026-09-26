// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

import (
	"net"
	"net/http"
	"net/url"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMigrationPolicy(t *testing.T) {
	defer test.MockVariableValue(&setting.Migrations)()
	for _, tc := range []struct {
		allow, block, host, ip string
		want                   bool
	}{
		{allow: "external", host: "github.com", ip: "1.2.3.4", want: true},
		{allow: "github.com", host: "github.com", ip: "10.0.0.1"},
		{allow: "external", block: "github.com", host: "github.com", ip: "1.2.3.4"},
	} {
		setting.Migrations.AllowedHostList, setting.Migrations.BlockedHostList = tc.allow, tc.block
		err := NewMigrationPolicy().CheckHostIPs(tc.host, []net.IP{net.ParseIP(tc.ip)})
		assert.Equal(t, tc.want, err == nil, "%+v: %v", tc, err)
	}
}

func TestWebhookPolicyProxy(t *testing.T) {
	proxyURL := &url.URL{Scheme: "http", Host: "localhost:8080"}
	defer test.MockVariableValue(&setting.Webhook.AllowedHostList, "discordapp.com,s.discordapp.com")()
	defer test.MockVariableValue(&setting.Webhook.ProxyURL, proxyURL.String())()
	defer test.MockVariableValue(&setting.Webhook.ProxyURLFixed, proxyURL)()
	defer test.MockVariableValue(&setting.Webhook.ProxyHosts, []string{"*.discordapp.com", "discordapp.com"})()
	selectProxy := NewWebhookPolicy().NewHTTPTransport().Proxy

	for target, want := range map[string]string{
		"https://discordapp.com/api/webhooks/xxxxxxxxx/xxxxxxxxxxxxxxxxxxx": proxyURL.String(),
		"http://s.discordapp.com/assets/xxxxxx":                             proxyURL.String(),
		"http://github.com/a/b":                                             "",
		"http://www.discordapp.com/assets/xxxxxx":                           "error",
	} {
		req, err := http.NewRequest(http.MethodPost, target, nil)
		require.NoError(t, err)
		req.Host = ""
		u, err := selectProxy(req)
		if want == "error" {
			assert.Error(t, err, target)
			continue
		}
		require.NoError(t, err, target)
		if want == "" {
			assert.Nil(t, u, target)
		} else {
			assert.Equal(t, want, u.String(), target)
		}
	}
}

func TestSecurityPolicy(t *testing.T) {
	defer test.MockVariableValue(&setting.Security.AllowedHostList, "avatars.example.com")()
	securityPolicy := NewSecurityPolicy("test")
	assert.NoError(t, securityPolicy.CheckHost("avatars.example.com"))
	assert.Error(t, securityPolicy.CheckHost("8.8.8.8"))
}

func TestNewGitPolicy(t *testing.T) {
	gitConfig := map[string]string{"http.proxy": "proxy.corp"}
	defer test.MockVariableValue(&setting.GitConfig.Options, gitConfig)()
	gitPolicy, err := NewGitPolicy()
	require.NoError(t, err)
	selected, err := gitPolicy.Proxy(&http.Request{URL: &url.URL{Scheme: "https", Host: "git.example.com"}})
	require.NoError(t, err)
	assert.Equal(t, "http://proxy.corp:1080", selected.String())

	gitConfig["http.proxy"] = "http://user:secret@[::1"
	_, err = NewGitPolicy()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
}
