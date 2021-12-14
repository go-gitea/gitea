// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package externalaccount

import (
	"fmt"

	user_model "code.gitea.io/gitea/models/user"
	"github.com/markbates/goth"
)

// Store represents a thing that stores things
type Store interface {
	Get(interface{}) interface{}
	Set(interface{}, interface{}) error
	Release() error
}

// LinkAccountFromStore links the provided user with a stored external user
func LinkAccountFromStore(store Store, user *user_model.User) error {
	gothUser := store.Get("linkAccountGothUser")
	if gothUser == nil {
		return fmt.Errorf("not in LinkAccount session")
	}

	return LinkAccountToUser(user, gothUser.(goth.User))
}
