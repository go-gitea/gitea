// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package swagger

import (
	api "gitea.dev/modules/structs"
)

// OAuth2Application
// swagger:response OAuth2Application
type swaggerResponseOAuth2Application struct {
	// in:body
	Body api.OAuth2Application `json:"body"`
}

// AccessToken represents an API access token.
// swagger:response AccessToken
type swaggerResponseAccessToken struct {
	// in:body
	Body api.AccessToken `json:"body"`
}

// CurrentAccessToken represents the currently authenticated access token.
// swagger:response CurrentAccessToken
type swaggerResponseCurrentAccessToken struct {
	// in:body
	Body api.CurrentAccessToken `json:"body"`
}
