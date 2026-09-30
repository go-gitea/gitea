// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveMirrorCredentialsToDatabase(t *testing.T) {
	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		OwnerName string
		Name      string
	}
	type Mirror struct {
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64
	}
	type PushMirror struct {
		ID         int64 `xorm:"pk autoincr"`
		RepoID     int64
		RemoteName string
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0, new(Repository), new(Mirror), new(PushMirror))
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&Repository{ID: 1, OwnerName: "user2", Name: "repo1"}, &Repository{ID: 2, OwnerName: "user2", Name: "repo2"},
		&Mirror{ID: 1, RepoID: 1}, &PushMirror{ID: 1, RepoID: 1, RemoteName: "remote_mirror_a"},
		&PushMirror{ID: 2, RepoID: 2, RemoteName: "remote_mirror_b"}, &PushMirror{ID: 3, RepoID: 2, RemoteName: "missing"})
	require.NoError(t, err)

	setRemote := func(repo gitrepo.RepositoryFacade, name, addr string) {
		_, _, err := gitcmd.NewCommand("config").AddDynamicArguments("remote."+name+".url", addr).WithRepo(repo).RunStdString(t.Context())
		require.NoError(t, err)
	}
	getRemote := func(repo gitrepo.RepositoryFacade, name string) string {
		addr, err := remoteAddress(t.Context(), repo, name)
		require.NoError(t, err)
		return addr
	}
	repo1, wiki1, repo2 := base.LocalCodeGitRepo("user2", "repo1"), base.LocalWikiGitRepo("user2", "repo1"), base.LocalCodeGitRepo("user2", "repo2")
	setRemote(repo1, "origin", "https://u:p@example.com/o/r.git")
	setRemote(wiki1, "origin", "https://u:p@example.com/o/r.wiki.git")
	setRemote(repo1, "remote_mirror_a", "https://:token@example.com/o/a.git")
	setRemote(repo2, "remote_mirror_b", "git@example.com:o/b.git")

	require.NoError(t, MoveMirrorCredentialsToDatabase(t.Context(), x))

	encrypted := func(table string, id int64) string {
		var s string
		_, err := x.Table(table).Where("id = ?", id).Cols("remote_address_encrypted").Get(&s)
		require.NoError(t, err)
		if s == "" {
			return ""
		}
		s, err = secret.DecryptSecret(setting.SecretKey, s)
		require.NoError(t, err)
		return s
	}
	assert.Equal(t, "https://u:p@example.com/o/r.git", encrypted("mirror", 1))
	assert.Equal(t, "https://:token@example.com/o/a.git", encrypted("push_mirror", 1))
	assert.Empty(t, encrypted("push_mirror", 2))
	assert.Empty(t, encrypted("push_mirror", 3))

	assert.Equal(t, "https://example.com/o/r.git", getRemote(repo1, "origin"))
	assert.Equal(t, "https://example.com/o/r.wiki.git", getRemote(wiki1, "origin"))
	assert.Equal(t, "https://example.com/o/a.git", getRemote(repo1, "remote_mirror_a"))
	assert.Equal(t, "git@example.com:o/b.git", getRemote(repo2, "remote_mirror_b"))
}
