// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package openid

import (
	"net/http"
	"sync"
	"time"

	"gitea.dev/modules/egress"

	"github.com/yohcop/openid-go"
)

// For the demo, we use in-memory infinite storage nonce and discovery
// cache. In your app, do not use this as it will eat up memory and
// never
// free it. Use your own implementation, on a better database system.
// If you have multiple servers for example, you may need to share at
// least
// the nonceStore between them.
var (
	nonceStore     = openid.NewSimpleNonceStore()
	discoveryCache = newTimedDiscoveryCache(24 * time.Hour)

	// openIDInstance keeps user-supplied OpenID identifiers within [security] ALLOWED_HOST_LIST
	openIDInstance = sync.OnceValue(func() *openid.OpenID {
		return openid.NewOpenID(&http.Client{
			Timeout:   30 * time.Second,
			Transport: egress.NewSecurityPolicy("openid").NewHTTPTransport(),
		})
	})
)

// Verify handles response from OpenID provider
func Verify(fullURL string) (id string, err error) {
	return openIDInstance().Verify(fullURL, discoveryCache, nonceStore)
}

// Normalize normalizes an OpenID URI
func Normalize(url string) (id string, err error) {
	return openid.Normalize(url)
}

// RedirectURL redirects browser
func RedirectURL(id, callbackURL, realm string) (string, error) {
	return openIDInstance().RedirectURL(id, callbackURL, realm)
}
