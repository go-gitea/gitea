// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"code.gitea.io/git"

	"github.com/stretchr/testify/assert"
)

func TestRelease_Create(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	repoPath := RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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

	assert.NoError(t, CreateRelease(gitRepo, &Release{
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
