// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestRelease_Create(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1",
		Target:       "master",
		Title:        "v0.1 is released",
		Note:         "v0.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1.1",
		Target:       "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Title:        "v0.1.1 is released",
		Note:         "v0.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1.2",
		Target:       "65f1bf2",
		Title:        "v0.1.2 is released",
		Note:         "v0.1.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1.3",
		Target:       "65f1bf2",
		Title:        "v0.1.3 is released",
		Note:         "v0.1.3 is released",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1.4",
		Target:       "65f1bf2",
		Title:        "v0.1.4 is released",
		Note:         "v0.1.4 is released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v0.1.5",
		Target:       "65f1bf2",
		Title:        "v0.1.5 is released",
		Note:         "v0.1.5 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}, nil))
}

func TestRelease_Update(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Test a changed release
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v1.1",
		Target:       "master",
		Title:        "v1.1 is released",
		Note:         "v1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))
	release, err := models.GetRelease(repo.ID, "v0.1")
	assert.NoError(t, err)
	releaseCreatedUnix := release.CreatedUnix
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Equal(t, releaseCreatedUnix, release.CreatedUnix)

	// Test a changed draft
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v1.2",
		Target:       "65f1bf2",
		Title:        "v1.2 is draft",
		Note:         "v1.2 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil))
	release, err = models.GetRelease(repo.ID, "v1.1")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	release.Title = "Changed title"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Greater(t, release.CreatedUnix, releaseCreatedUnix, )

	// Test a changed pre-release
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v1.3",
		Target:       "65f1bf2",
		Title:        "v1.3 is pre-released",
		Note:         "v1.3 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil))
	release, err = models.GetRelease(repo.ID, "v1.3")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	release.Title = "Changed title"
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Equal(t, release.CreatedUnix, releaseCreatedUnix)
}

func TestRelease_createTag(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Test a changed release
	release := &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v2.1",
		Target:       "master",
		Title:        "v2.1 is released",
		Note:         "v2.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}
	assert.NoError(t, createTag(gitRepo, release))
	assert.NotEmpty(t, release.CreatedUnix)
	releaseCreatedUnix := release.CreatedUnix
	release.Note = "Changed note"
	assert.NoError(t, createTag(gitRepo, release))
	assert.Equal(t, releaseCreatedUnix, release.CreatedUnix)

	// Test a changed draft
	release = &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v2.2",
		Target:       "65f1bf2",
		Title:        "v2.2 is draft",
		Note:         "v2.2 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}
	assert.NoError(t, createTag(gitRepo, release))
	releaseCreatedUnix = release.CreatedUnix
	release.Title = "Changed title"
	assert.NoError(t, createTag(gitRepo, release))
	assert.Greater(t, release.CreatedUnix, releaseCreatedUnix, )

	// Test a changed pre-release
	release = &models.Release{
		RepoID:       repo.ID,
		PublisherID:  user.ID,
		TagName:      "v2.3",
		Target:       "65f1bf2",
		Title:        "v2.3 is pre-released",
		Note:         "v2.3 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}
	assert.NoError(t, createTag(gitRepo, release))
	releaseCreatedUnix = release.CreatedUnix
	release.Title = "Changed title"
	release.Note = "Changed note"
	assert.NoError(t, createTag(gitRepo, release))
	assert.Equal(t, release.CreatedUnix, releaseCreatedUnix)
}
