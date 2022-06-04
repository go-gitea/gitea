// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/repository"
	mirror_service "code.gitea.io/gitea/services/mirror"
	release_service "code.gitea.io/gitea/services/release"

	"github.com/stretchr/testify/assert"
)

func TestMirrorPull(t *testing.T) {
	defer prepareTestEnv(t)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	opts := migration.MigrateOptions{
		RepoName:    "test_mirror",
		Description: "Test mirror",
		Private:     false,
		Mirror:      true,
		CloneAddr:   repoPath,
		Wiki:        true,
		Releases:    false,
	}

	mirrorRepo, err := repository.CreateRepository(user, user, models.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	})
	assert.NoError(t, err)
	assert.True(t, mirrorRepo.IsMirror, "expected pull-mirror repo to be marked as a mirror immediately after its creation")

	ctx := context.Background()

	mirror, err := repository.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	assert.NoError(t, err)

	gitRepo, err := git.OpenRepository(git.DefaultContext, repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	findOptions := models.FindReleasesOptions{IncludeDrafts: true, IncludeTags: true}
	initCount, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)

	assert.NoError(t, release_service.CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.2",
		Target:       "master",
		Title:        "v0.2 is released",
		Note:         "v0.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}, nil, ""))

	_, err = repo_model.GetMirrorByRepoID(ctx, mirror.ID)
	assert.NoError(t, err)

	ok := mirror_service.SyncPullMirror(ctx, mirror.ID)
	assert.True(t, ok)

	count, err := models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)
	assert.EqualValues(t, initCount+1, count)

	release, err := models.GetRelease(repo.ID, "v0.2")
	assert.NoError(t, err)
	assert.NoError(t, release_service.DeleteReleaseByID(ctx, release.ID, user, true))

	ok = mirror_service.SyncPullMirror(ctx, mirror.ID)
	assert.True(t, ok)

	count, err = models.GetReleaseCountByRepoID(mirror.ID, findOptions)
	assert.NoError(t, err)
	assert.EqualValues(t, initCount, count)
}
