// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"context"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
)

// Source is a password authentication service
type Source struct{}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return nil
}

// ToDB exports the config to a byte slice to be saved into database (this method is just dummy and does nothing for DB source)
func (source *Source) ToDB() ([]byte, error) {
	return nil, nil
}

// Authenticate queries if login/password is valid against the PAM,
// and create a local user if success when enabled.
func (source *Source) Authenticate(ctx context.Context, user *user_model.User, login, password string) (*user_model.User, error) {
	return Authenticate(ctx, user, login, password)
}

func init() {
	auth.RegisterTypeConfig(auth.NoType, &Source{})
	auth.RegisterTypeConfig(auth.Plain, &Source{})
}
