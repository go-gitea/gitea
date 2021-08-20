// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package externalaccount

import (
	"fmt"

	"code.gitea.io/gitea/models"

	"github.com/markbates/goth"
)

type Store interface {
	Get(interface{}) interface{}
	Set(interface{}, interface{}) error
	Release() error
}

func LinkAccountFromStore(store Store, user *models.User) error {
	gothUser := store.Get("linkAccountGothUser")
	if gothUser == nil {
		return fmt.Errorf("not in LinkAccount session")
	}

	return LinkAccountToUser(user, gothUser.(goth.User))
}
