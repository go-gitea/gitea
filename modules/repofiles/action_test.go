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

func testCorrectRepoAction(t *testing.T, opts *CommitRepoActionOptions, actionBean *models.Action) {
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

		testCorrectRepoAction(t, &s.commitRepoActionOptions, &s.action)
	}
}

func TestUpdateIssuesCommit(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	pushCommits := []*models.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "start working on #FST-1, #1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "a plain message",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close #2",
		},
	}

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo.Owner = user

	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}
	issueBean := &models.Issue{RepoID: repo.ID, Index: 4}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 2}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})

	// Test that push to a non-default branch closes no issue.
	pushCommits = []*models.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #1",
		},
	}
	repo = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	commentBean = &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   6,
	}
	issueBean = &models.Issue{RepoID: repo.ID, Index: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 1}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_Colon(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	pushCommits := []*models.PushCommit{
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close: #2",
		},
	}

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo.Owner = user

	issueBean := &models.Issue{RepoID: repo.ID, Index: 4}

	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 2}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_Issue5957(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// Test that push to a non-default branch closes an issue.
	pushCommits := []*models.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #2",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   7,
	}

	issueBean := &models.Issue{RepoID: repo.ID, Index: 2, ID: 7}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_AnotherRepo(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// Test that a push to default branch closes issue in another repo
	// If the user also has push permissions to that repo
	pushCommits := []*models.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close user2/repo1#1",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}

	issueBean := &models.Issue{RepoID: 1, Index: 1, ID: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_AnotherRepoNoPermission(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 10}).(*models.User)

	// Test that a push with close reference *can not* close issue
	// If the commiter doesn't have push rights in that repo
	pushCommits := []*models.PushCommit{
		{
			Sha1:           "abcdef3",
			CommitterEmail: "user10@example.com",
			CommitterName:  "User Ten",
			AuthorEmail:    "user10@example.com",
			AuthorName:     "User Ten",
			Message:        "close user3/repo3#1",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 6}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef3",
		PosterID:  user.ID,
		IssueID:   6,
	}

	issueBean := &models.Issue{RepoID: 3, Index: 1, ID: 6}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}
