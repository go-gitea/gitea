// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"fmt"

	user_model "code.gitea.io/gitea/models/user"

	"github.com/markbates/goth"
)

// Store represents a thing that stores things
type Store interface {
	Get(any) any
	Set(any, any) error
	Release() error
}

// LinkAccountFromStore links the provided user with a stored external user
func LinkAccountFromStore(ctx context.Context, store Store, user *user_model.User) error {
	gothUser := store.Get("linkAccountGothUser")
	if gothUser == nil {
		return fmt.Errorf("not in LinkAccount session")
	}

	return LinkAccountToUser(ctx, user, gothUser.(goth.User))
}
