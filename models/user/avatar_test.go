// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestUserAvatarLink(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	u := &User{ID: 1, Avatar: "avatar.png"}
	link := u.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/avatars/avatar.png", link)

	setting.AppURL = "https://localhost/sub-path/"
	setting.AppSubURL = "/sub-path"
	link = u.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/sub-path/avatars/avatar.png", link)
}
