// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graphql

import (
	"time"

	"code.gitea.io/gitea/models"
)

func convertUser(user *models.User, authed bool) *User {
	if user == nil {
		return nil
	}

	u := &User{
		Login:      user.Name,
		Name:       user.FullName,
		DatabaseID: user.ID,
		CreatedAt:  time.Unix(int64(user.CreatedUnix), 0),
	}

	if !user.KeepEmailPrivate || authed {
		u.Email = &user.Email
	}

	return u
}
