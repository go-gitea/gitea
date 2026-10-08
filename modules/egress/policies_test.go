// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package egress

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"gitea.dev/modules/egress/policy"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestNewMigrationPolicy(t *testing.T) {
	defer test.MockVariableValue(&setting.Migrations)()
	for _, tc := range []struct {
		allow, block, mode, target string
		want                       bool
	}{
		{target: "https://1.2.3.4", want: true},
		{allow: "github.com", target: "https://10.0.0.1"},            // a hostname allow doesn't cover a private IP
		{allow: "github.com", target: "https://8.8.8.8", want: true}, // lax exempts public targets
		{allow: "github.com", mode: "strict", target: "https://8.8.8.8"},
		{block: "8.8.0.0/16", target: "https://8.8.8.8"},
	} {
		setting.Migrations.AllowedHostList, setting.Migrations.BlockedHostList, setting.Migrations.EgressMode = tc.allow, tc.block, tc.mode
		u, err := url.Parse(tc.target)
		require.NoError(t, err)
		err = NewMigrationPolicy().CheckHostIPs(u)
		assert.Equal(t, tc.want, err == nil, "%+v: %v", tc, err)
	}
}

func TestWebhookPolicyProxy(t *testing.T) {
	proxyURL := &url.URL{Scheme: "http", Host: "localhost:8080"}
	defer test.MockVariableValue(&setting.Security.EgressMode, "lax")()
	defer test.MockVariableValue(&setting.Webhook.AllowedHostList, "discordapp.com,s.discordapp.com")()
	defer test.MockVariableValue(&setting.Webhook.ProxyURL, proxyURL.String())()
	defer test.MockVariableValue(&setting.Webhook.ProxyURLFixed, proxyURL)()
	defer test.MockVariableValue(&setting.Webhook.ProxyHosts, []string{"*.discordapp.com", "discordapp.com"})()
	selectProxy := NewWebhookPolicy().NewHTTPTransport().Proxy

	for target, want := range map[string]string{
		"https://discordapp.com/api/webhooks/xxxxxxxxx/xxxxxxxxxxxxxxxxxxx": proxyURL.String(),
		"http://s.discordapp.com/assets/xxxxxx":                             proxyURL.String(),
		"http://github.com/a/b":                                             "",
		"http://www.discordapp.com/assets/xxxxxx":                           proxyURL.String(),
	} {
		req, err := http.NewRequest(http.MethodPost, target, nil)
		require.NoError(t, err)
		req.Host = ""
		u, err := selectProxy(req)
		require.NoError(t, err, target)
		if want == "" {
			assert.Nil(t, u, target)
		} else {
			assert.Equal(t, want, u.String(), target)
		}
	}
}

func TestWebhookPolicyNeedsIPAllow(t *testing.T) {
	defer test.MockVariableValue(&setting.Webhook.AllowedHostList, "localhost")()
	defer test.MockVariableValue(&setting.Security.EgressMode, "lax")()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	dial := func() error {
		tcpAddr, ok := ln.Addr().(*net.TCPAddr)
		require.True(t, ok)
		target := net.JoinHostPort("localhost", strconv.Itoa(tcpAddr.Port))
		conn, err := NewWebhookPolicy().NewDialContext()(t.Context(), "tcp", target)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}
	assert.ErrorIs(t, dial(), policy.ErrDenied) // a host name entry doesn't cover the loopback address
	setting.Webhook.AllowedHostList = "loopback"
	assert.NoError(t, dial()) // an IP entry does
}

func TestSecurityPolicy(t *testing.T) {
	defer test.MockVariableValue(&setting.Security.AllowedHostList, "avatars.example.com")()
	defer test.MockVariableValue(&setting.Security.EgressMode, "lax")()
	lax := NewSecurityPolicy("test")
	assert.NoError(t, lax.CheckHost(mustURL(t, "https://avatars.example.com")))
	assert.NoError(t, lax.CheckHost(mustURL(t, "https://8.8.8.8"))) // lax exempts public targets
	assert.Error(t, lax.CheckHost(mustURL(t, "https://10.0.0.1")))  // restricted targets still need an allow entry

	setting.Security.EgressMode = "strict"
	strict := NewSecurityPolicy("test")
	assert.NoError(t, strict.CheckHost(mustURL(t, "https://avatars.example.com")))
	assert.Error(t, strict.CheckHost(mustURL(t, "https://8.8.8.8")))
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
