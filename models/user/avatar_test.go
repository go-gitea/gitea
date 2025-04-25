// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"io"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestUserAvatarGenerate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	var err error
	tmpDir := t.TempDir()
	storage.Avatars, err = storage.NewLocalStorage(t.Context(), &setting.Storage{Path: tmpDir})
	require.NoError(t, err)

	u := unittest.AssertExistsAndLoadBean(t, &User{ID: 2})

	// there was no avatar, generate a new one
	assert.Empty(t, u.Avatar)
	err = GenerateRandomAvatar(db.DefaultContext, u)
	require.NoError(t, err)
	assert.NotEmpty(t, u.Avatar)

	// make sure the generated one exists
	oldAvatarPath := u.CustomAvatarRelativePath()
	_, err = storage.Avatars.Stat(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	// and try to change its content
	_, err = storage.Avatars.Save(u.CustomAvatarRelativePath(), strings.NewReader("abcd"), 4)
	require.NoError(t, err)

	// try to generate again
	err = GenerateRandomAvatar(db.DefaultContext, u)
	require.NoError(t, err)
	assert.Equal(t, oldAvatarPath, u.CustomAvatarRelativePath())
	f, err := storage.Avatars.Open(u.CustomAvatarRelativePath())
	require.NoError(t, err)
	defer f.Close()
	content, _ := io.ReadAll(f)
	assert.Equal(t, "abcd", string(content))
}
