// Copyright 2026 The Gitea Authors.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"
	"time"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCreateCommitComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	now := time.Now().Unix()
	c := &git_model.CommitComment{
		RepoID:    repo.ID,
		CommitSHA: "abcdef1",
		PosterID:  doer.ID,
		Content:   "hello commit",
	}
	assert.NoError(t, git_model.CreateCommitComment(t.Context(), c))
	then := time.Now().Unix()

	assert.Equal(t, repo.ID, c.RepoID)
	assert.Equal(t, "abcdef1", c.CommitSHA)
	assert.Equal(t, doer.ID, c.PosterID)
	assert.Equal(t, "hello commit", c.Content)
	unittest.AssertInt64InRange(t, now, then, int64(c.CreatedUnix))
	unittest.AssertExistsAndLoadBean(t, c)

	// load poster
	assert.NoError(t, c.LoadPoster(t.Context()))
	assert.NotNil(t, c.Poster)
}

func TestListUpdateDeleteCommitComment(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	c := &git_model.CommitComment{
		RepoID:    repo.ID,
		CommitSHA: "deadbeef",
		PosterID:  doer.ID,
		Content:   "first",
	}
	assert.NoError(t, git_model.CreateCommitComment(t.Context(), c))

	list, err := git_model.ListCommitComments(t.Context(), repo.ID, "deadbeef")
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "first", list[0].Content)

	// update
	list[0].Content = "updated"
	assert.NoError(t, git_model.UpdateCommitComment(t.Context(), list[0]))
	c2, err := git_model.GetCommitCommentByID(t.Context(), list[0].ID)
	assert.NoError(t, err)
	assert.Equal(t, "updated", c2.Content)

	// delete
	assert.NoError(t, git_model.DeleteCommitComment(t.Context(), c2.ID))
	_, err = git_model.GetCommitCommentByID(t.Context(), c2.ID)
	assert.Error(t, err)

	// ensure deleted not listed
	list2, err := git_model.ListCommitComments(t.Context(), repo.ID, "deadbeef")
	assert.NoError(t, err)
	assert.Empty(t, list2)

	// ensure DB consistency
	unittest.CheckConsistencyFor(t, &git_model.CommitComment{})
}
