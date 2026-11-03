// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestRepoAvatarLink(t *testing.T) {
	defer test.MockVariableValue(&setting.AppURL, "https://localhost/")()
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	repo := &Repository{ID: 1, Avatar: "avatar.png"}
	link := repo.AvatarLink(t.Context())
	assert.Equal(t, "https://localhost/repo-avatars/avatar.png", link)

	setting.AppURL = "https://localhost/sub-path/"
	setting.AppSubURL = "/sub-path"
	link = repo.AvatarLink(t.Context())
	assert.Equal(t, "https://localhost/sub-path/repo-avatars/avatar.png", link)
}

func TestArchiveType(t *testing.T) {
	for _, tc := range []struct {
		input, name, archiveType string
	}{
		{"test.tar.gz", "test", "tar.gz"},
		{"a/b/test.zip", "a/b/test", "zip"},
		{"1234.bundle", "1234", "bundle"},
		{"test", "test", "unknown"},
		{"test.xz", "test.xz", "unknown"},
	} {
		name, archiveType := SplitArchiveNameType(tc.input)
		assert.Equal(t, tc.name, name)
		assert.Equal(t, tc.archiveType, archiveType.String())
	}
}
