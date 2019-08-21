// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func testCorrectRepoAction(t *testing.T, opts CommitRepoActionOptions, actionBean *models.Action) {
	models.AssertNotExistsBean(t, actionBean)
	assert.NoError(t, CommitRepoAction(opts))
	models.AssertExistsAndLoadBean(t, actionBean)
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestCommitRepoAction(t *testing.T) {
	samples := []struct {
		userID                  int64
		repositoryID            int64
		commitRepoActionOptions CommitRepoActionOptions
		action                  models.Action
	}{
		{
			userID:       2,
			repositoryID: 16,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: "refName",
				OldCommitID: "oldCommitID",
				NewCommitID: "newCommitID",
				Commits: &models.PushCommits{
					Commits: []*models.PushCommit{
						{
							Sha1:           "69554a6",
							CommitterEmail: "user2@example.com",
							CommitterName:  "User2",
							AuthorEmail:    "user2@example.com",
							AuthorName:     "User2",
							Message:        "not signed commit",
						},
						{
							Sha1:           "27566bd",
							CommitterEmail: "user2@example.com",
							CommitterName:  "User2",
							AuthorEmail:    "user2@example.com",
							AuthorName:     "User2",
							Message:        "good signed commit (with not yet validated email)",
						},
					},
					Len: 2,
				},
			},
			action: models.Action{
				OpType:  models.ActionCommitRepo,
				RefName: "refName",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.TagPrefix + "v1.1",
				OldCommitID: git.EmptySHA,
				NewCommitID: "newCommitID",
				Commits:     &models.PushCommits{},
			},
			action: models.Action{
				OpType:  models.ActionPushTag,
				RefName: "v1.1",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.TagPrefix + "v1.1",
				OldCommitID: "oldCommitID",
				NewCommitID: git.EmptySHA,
				Commits:     &models.PushCommits{},
			},
			action: models.Action{
				OpType:  models.ActionDeleteTag,
				RefName: "v1.1",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.BranchPrefix + "feature/1",
				OldCommitID: "oldCommitID",
				NewCommitID: git.EmptySHA,
				Commits:     &models.PushCommits{},
			},
			action: models.Action{
				OpType:  models.ActionDeleteBranch,
				RefName: "feature/1",
			},
		},
	}

	for _, s := range samples {
		models.PrepareTestEnv(t)

		user := models.AssertExistsAndLoadBean(t, &models.User{ID: s.userID}).(*models.User)
		repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: s.repositoryID, OwnerID: user.ID}).(*models.Repository)
		repo.Owner = user

		s.commitRepoActionOptions.PusherName = user.Name
		s.commitRepoActionOptions.RepoOwnerID = user.ID
		s.commitRepoActionOptions.RepoName = repo.Name

		s.action.ActUserID = user.ID
		s.action.RepoID = repo.ID
		s.action.Repo = repo
		s.action.IsPrivate = repo.IsPrivate

		testCorrectRepoAction(t, s.commitRepoActionOptions, &s.action)
	}
}
