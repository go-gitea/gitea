// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushUpdateAddDeleteTagsPublisher(t *testing.T) {
	unittest.PrepareTestEnv(t)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	gitRepo, err := git.OpenRepository(t.Context(), repo)
	require.NoError(t, err)
	defer gitRepo.Close()
	require.NoError(t, gitRepo.CreateTag(t.Context(), "delete-tag", "master"))

	pusher := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	require.NoError(t, PushUpdateAddDeleteTags(t.Context(), repo, gitRepo, pusher, []string{"v1.1", "delete-tag"}, nil))
	assert.EqualValues(t, 2, unittest.AssertExistsAndLoadBean(t, &repo_model.Release{TagName: "v1.1", RepoID: repo.ID}).PublisherID)
	assert.Equal(t, pusher.ID, unittest.AssertExistsAndLoadBean(t, &repo_model.Release{TagName: "delete-tag", RepoID: repo.ID}).PublisherID)
}
