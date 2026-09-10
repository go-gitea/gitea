// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package turnstile

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientHonorsProxy(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.example.com:3128")
	require.NoError(t, err)

	defer test.MockVariableValue(&setting.Proxy.Enabled, true)()
	defer test.MockVariableValue(&setting.Proxy.ProxyURL, proxyURL.String())()
	defer test.MockVariableValue(&setting.Proxy.ProxyURLFixed, proxyURL)()
	defer test.MockVariableValue(&setting.Proxy.ProxyHosts, []string{"**"})()
	httpClient.Reset()
	transport, ok := httpClient.Value().Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.Proxy)

	// The Turnstile verification request must be routed through the configured proxy.
	req := httptest.NewRequest(http.MethodPost, "https://any.example.com", nil)
	got, err := transport.Proxy(req)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, proxyURL.String(), got.String())
}
