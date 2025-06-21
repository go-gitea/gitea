// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/session"
)

type DataStore = reqctx.ContextDataProvider

// SessionStore represents a session store
type SessionStore session.Store

// Method represents an authentication method (plugin) for HTTP requests.
type Method interface {
	// Verify tries to verify the authentication data contained in the request.
	// If verification is successful returns either an existing user object (with id > 0)
	// or a new user object (with id = 0) populated with the information that was found
	// in the authentication data (username or email).
	// Second argument returns err if verification fails, otherwise
	// First return argument returns nil if no matched verification condition
	Verify(http *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) (*user_model.User, error)

	Name() string
}

// PasswordAuthenticator represents a source of authentication
type PasswordAuthenticator interface {
	Authenticate(ctx context.Context, user *user_model.User, login, password string) (*user_model.User, error)
}

// SynchronizableSource represents a source that can synchronize users
type SynchronizableSource interface {
	Sync(ctx context.Context, updateExisting bool) error
}
