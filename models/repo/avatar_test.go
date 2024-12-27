// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRepoAvatarLink(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	repo := &Repository{ID: 1, Avatar: "avatar.png"}
	link := repo.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/repo-avatars/avatar.png", link)

	setting.AppURL = "https://localhost/sub-path/"
	setting.AppSubURL = "/sub-path"
	link = repo.AvatarLink(db.DefaultContext)
	assert.Equal(t, "https://localhost/sub-path/repo-avatars/avatar.png", link)
}
