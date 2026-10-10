// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"strconv"

	"gitea.dev/modules/web/middleware"
)

// Credential kinds naming how a request authenticated, recorded so an audit
// event can point at the token rather than only at its owner.
const (
	credentialAccessToken = "access-token"
	credentialOAuth2Grant = "oauth2-grant"
)

func setAuthCredential(store DataStore, kind string, id int64) {
	store.GetData()[middleware.ContextDataKeyAuthCredential] = kind + ":" + strconv.FormatInt(id, 10)
}
