// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package externalaccount

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/auth"
	user_model "code.gitea.io/gitea/models/user"
)

// Store represents a thing that stores things
type Store interface {
	Get(any) any
	Set(any, any) error
	Release() error
}

// LinkAccountFromStore links the provided user with a stored external user
func LinkAccountFromStore(ctx context.Context, store Store, user *user_model.User) error {
	externalLinkUserInterface := store.Get("linkAccountUser")
	if externalLinkUserInterface == nil {
		return fmt.Errorf("not in LinkAccount session")
	}

	externalLinkUser := externalLinkUserInterface.(auth.LinkAccountUser)

	return LinkAccountToUser(ctx, user, externalLinkUser.GothUser, externalLinkUser.Type)
}
