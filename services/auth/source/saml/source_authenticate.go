// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package saml

import (
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/services/auth/source/db"
)

// Authenticate falls back to the db authenticator
func (source *Source) Authenticate(user *user_model.User, login, password string) (*user_model.User, error) {
	return db.Authenticate(user, login, password)
}
