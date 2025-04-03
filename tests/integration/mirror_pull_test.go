// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"slices"
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/migration"
	mirror_service "code.gitea.io/gitea/services/mirror"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMirrorPull(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	ctx := t.Context()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repoPath := repo_model.RepoPath(user.Name, repo.Name)

	opts := migration.MigrateOptions{
		RepoName:    "test_mirror",
		Description: "Test mirror",
		Private:     false,
		Mirror:      true,
		CloneAddr:   repoPath,
		Wiki:        true,
		Releases:    true,
	}

	mirrorRepo, err := repo_service.CreateRepositoryDirectly(ctx, user, user, repo_service.CreateRepoOptions{
		Name:        opts.RepoName,
		Description: opts.Description,
		IsPrivate:   opts.Private,
		IsMirror:    opts.Mirror,
		Status:      repo_model.RepositoryBeingMigrated,
	})
	assert.NoError(t, err)
	assert.True(t, mirrorRepo.IsMirror, "expected pull-mirror repo to be marked as a mirror immediately after its creation")

	mirrorRepo, err = repo_service.MigrateRepositoryGitData(ctx, user, mirrorRepo, opts, nil)
	assert.NoError(t, err)

	// these units should have been enabled
	mirrorRepo.Units = nil
	require.NoError(t, mirrorRepo.LoadUnits(ctx))
	assert.True(t, slices.ContainsFunc(mirrorRepo.Units, func(u *repo_model.RepoUnit) bool { return u.Type == unit.TypeReleases }))
	assert.True(t, slices.ContainsFunc(mirrorRepo.Units, func(u *repo_model.RepoUnit) bool { return u.Type == unit.TypeWiki }))

	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo)
	assert.NoError(t, err)
	defer gitRepo.Close()

	findOptions := repo_model.FindReleasesOptions{
		IncludeDrafts: true,
		IncludeTags:   true,
		RepoID:        mirrorRepo.ID,
	}
	initCount, err := db.Count[repo_model.Release](db.DefaultContext, findOptions)
	assert.NoError(t, err)
	assert.Zero(t, initCount) // no sync yet, so even though there is a tag in source repo, the mirror's release table is still empty

	assert.NoError(t, release_service.CreateRelease(gitRepo, &repo_model.Release{
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

	_, err = repo_model.GetMirrorByRepoID(ctx, mirrorRepo.ID)
	assert.NoError(t, err)

	ok := mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.True(t, ok)

	// actually there is a tag in the source repo, so after "sync", that tag will also come into the mirror
	initCount++

	count, err := db.Count[repo_model.Release](db.DefaultContext, findOptions)
	assert.NoError(t, err)
	assert.Equal(t, initCount+1, count)

	release, err := repo_model.GetRelease(db.DefaultContext, repo.ID, "v0.2")
	assert.NoError(t, err)
	assert.NoError(t, release_service.DeleteReleaseByID(ctx, repo, release, user, true))

	ok = mirror_service.SyncPullMirror(ctx, mirrorRepo.ID)
	assert.True(t, ok)

	count, err = db.Count[repo_model.Release](db.DefaultContext, findOptions)
	assert.NoError(t, err)
	assert.Equal(t, initCount, count)
}
