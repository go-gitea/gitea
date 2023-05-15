// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"net/http"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/web/middleware"
)

// DataStore represents a data store
type DataStore middleware.ContextDataStore

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
}

// Initializable represents a structure that requires initialization
// It usually should only be called once before anything else is called
type Initializable interface {
	// Init should be called exactly once before using any of the other methods,
	// in order to allow the plugin to allocate necessary resources
	Init(ctx context.Context) error
}

// Named represents a named thing
type Named interface {
	Name() string
}

// Freeable represents a structure that is required to be freed
type Freeable interface {
	// Free should be called exactly once before application closes, in order to
	// give chance to the plugin to free any allocated resources
	Free() error
}

// PasswordAuthenticator represents a source of authentication
type PasswordAuthenticator interface {
	Authenticate(user *user_model.User, login, password string) (*user_model.User, error)
}

// LocalTwoFASkipper represents a source of authentication that can skip local 2fa
type LocalTwoFASkipper interface {
	IsSkipLocalTwoFA() bool
}

// SynchronizableSource represents a source that can synchronize users
type SynchronizableSource interface {
	Sync(ctx context.Context, updateExisting bool) error
}
