// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bufio"
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestGetProxyURL(t *testing.T) {
	os.Setenv("http_proxy", "http://127.0.0.1:1087")
	os.Setenv("https_proxy", "http://127.0.0.1:1087")
	os.Setenv("no_proxy", "example2.com")

	cfg := &setting.Proxy
	cfg.Enabled = false
	cfg.SMTPProxyEnabled = false
	cfg.ProxyURL = "https://127.0.0.1:2087"
	cfg.ProxyHosts = []string{`gitea.io`}

	setting.ParseProxy()

	req, err := http.NewRequest("GET", "https://gitea.io", &bufio.Reader{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Proxy()(req)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Enabled = true
	proxyURL, err := Proxy()(req)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, proxyURL)
	assert.Equal(t, "127.0.0.1:2087", proxyURL.Host) // in PROXY_HOSTS list

	req, err = http.NewRequest("GET", "https://example.com", &bufio.Reader{})
	if err != nil {
		t.Fatal(err)
	}

	proxyURL, err = Proxy()(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "127.0.0.1:1087", proxyURL.Host) // not in PROXY_HOSTS, from env

	req, err = http.NewRequest("GET", "https://example2.com", &bufio.Reader{})
	if err != nil {
		t.Fatal(err)
	}

	proxyURL, err = Proxy()(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Nil(t, proxyURL) // not in PROXY_HOSTS, from env, ignored by no_proxy
}
