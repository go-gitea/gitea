// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mirror

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	release_service "code.gitea.io/gitea/services/release"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestRelease_MirrorDelete(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	migrationOptions := models.MigrateRepoOptions{
		Name:                 "test_mirror",
		Description:          "Test mirror",
		IsPrivate:            false,
		IsMirror:             true,
		RemoteAddr:           repoPath,
		Wiki:                 true,
		SyncReleasesWithTags: true,
	}
	mirror, err := models.MigrateRepository(user, user, migrationOptions)
	assert.NoError(t, err)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)

	findOptions := models.FindReleasesOptions{IncludeDrafts: true, IncludeTags: true}
	initCount, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)

	assert.NoError(t, release_service.CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.2",
		Target:       "master",
		Title:        "v0.2 is released",
		Note:         "v0.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}, nil))

	err = mirror.GetMirror()
	assert.NoError(t, err)

	_, ok := runSync(mirror.Mirror)
	assert.True(t, ok)

	count, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.EqualValues(t, initCount+1, count)

	release, err := models.GetRelease(repo.ID, "v0.2")
	assert.NoError(t, err)
	assert.NoError(t, release_service.DeleteReleaseByID(release.ID, user, true))

	_, ok = runSync(mirror.Mirror)
	assert.True(t, ok)

	count, err = models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.EqualValues(t, initCount, count)
}
