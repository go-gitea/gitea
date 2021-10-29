// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"net"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func TestWebhookProxy(t *testing.T) {
	setting.Webhook.ProxyURL = "http://localhost:8080"
	setting.Webhook.ProxyURLFixed, _ = url.Parse(setting.Webhook.ProxyURL)
	setting.Webhook.ProxyHosts = []string{"*.discordapp.com", "discordapp.com"}

	var kases = map[string]string{
		"https://discordapp.com/api/webhooks/xxxxxxxxx/xxxxxxxxxxxxxxxxxxx": "http://localhost:8080",
		"http://s.discordapp.com/assets/xxxxxx":                             "http://localhost:8080",
		"http://github.com/a/b":                                             "",
	}

	for reqURL, proxyURL := range kases {
		req, err := http.NewRequest("POST", reqURL, nil)
		assert.NoError(t, err)

		u, err := webhookProxy()(req)
		assert.NoError(t, err)
		if proxyURL == "" {
			assert.Nil(t, u)
		} else {
			assert.EqualValues(t, proxyURL, u.String())
		}
	}
}

func TestIsWebhookRequestAllowed(t *testing.T) {
	type tc struct {
		host     string
		ip       net.IP
		expected bool
	}

	ah, an := setting.ParseWebhookAllowedHostList("private, global, *.google.com, 169.254.1.0/24")
	cases := []tc{
		{"", net.IPv4zero, false},

		{"", net.ParseIP("127.0.0.1"), false},

		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},

		{"", net.ParseIP("8.8.8.8"), true},

		{"google.com", net.IPv4zero, false},
		{"sub.google.com", net.IPv4zero, true},

		{"", net.ParseIP("169.254.1.1"), true},
		{"", net.ParseIP("169.254.2.2"), false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, isWebhookRequestAllowed(ah, an, c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	ah, an = setting.ParseWebhookAllowedHostList("loopback")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), true},
		{"", net.ParseIP("10.0.1.1"), false},
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("8.8.8.8"), false},
		{"google.com", net.IPv4zero, false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, isWebhookRequestAllowed(ah, an, c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	ah, an = setting.ParseWebhookAllowedHostList("private")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), false},
		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"", net.ParseIP("8.8.8.8"), false},
		{"google.com", net.IPv4zero, false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, isWebhookRequestAllowed(ah, an, c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	ah, an = setting.ParseWebhookAllowedHostList("global")
	cases = []tc{
		{"", net.IPv4zero, false},
		{"", net.ParseIP("127.0.0.1"), false},
		{"", net.ParseIP("10.0.1.1"), false},
		{"", net.ParseIP("192.168.1.1"), false},
		{"", net.ParseIP("8.8.8.8"), true},
		{"google.com", net.IPv4zero, false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, isWebhookRequestAllowed(ah, an, c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}

	ah, an = setting.ParseWebhookAllowedHostList("all")
	cases = []tc{
		{"", net.IPv4zero, true},
		{"", net.ParseIP("127.0.0.1"), true},
		{"", net.ParseIP("10.0.1.1"), true},
		{"", net.ParseIP("192.168.1.1"), true},
		{"", net.ParseIP("8.8.8.8"), true},
		{"google.com", net.IPv4zero, true},
	}
	for _, c := range cases {
		assert.Equalf(t, c.expected, isWebhookRequestAllowed(ah, an, c.host, c.ip), "case %s(%v)", c.host, c.ip)
	}
}
