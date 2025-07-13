// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pull_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/services/gitdiff"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/stretchr/testify/assert"
)

func TestDismissReview(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{})
	assert.NoError(t, pull.LoadIssue(db.DefaultContext))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))
	reviewer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	review, err := issues_model.CreateReview(db.DefaultContext, issues_model.CreateReviewOptions{
		Issue:    issue,
		Reviewer: reviewer,
		Type:     issues_model.ReviewTypeReject,
	})

	assert.NoError(t, err)
	issue.IsClosed = true
	pull.HasMerged = false
	assert.NoError(t, issues_model.UpdateIssueCols(db.DefaultContext, issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(db.DefaultContext, "has_merged"))
	_, err = pull_service.DismissReview(db.DefaultContext, review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))

	pull.HasMerged = true
	pull.Issue.IsClosed = false
	assert.NoError(t, issues_model.UpdateIssueCols(db.DefaultContext, issue, "is_closed"))
	assert.NoError(t, pull.UpdateCols(db.DefaultContext, "has_merged"))
	_, err = pull_service.DismissReview(db.DefaultContext, review.ID, issue.RepoID, "", &user_model.User{}, false, false)
	assert.Error(t, err)
	assert.True(t, pull_service.IsErrDismissRequestOnClosedPR(err))
}

func setupDefaultDiff() *gitdiff.Diff {
	return &gitdiff.Diff{
		Files: []*gitdiff.DiffFile{
			{
				Name: "README.md",
				Sections: []*gitdiff.DiffSection{
					{
						Lines: []*gitdiff.DiffLine{
							{
								LeftIdx:  4,
								RightIdx: 4,
							},
						},
					},
				},
			},
		},
	}
}

func TestDiff_LoadCommentsNoOutdated(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	diff := setupDefaultDiff()
	assert.NoError(t, issue.LoadRepo(t.Context()))
	assert.NoError(t, issue.LoadPullRequest(t.Context()))

	gitRepo, err := gitrepo.OpenRepository(t.Context(), issue.Repo)
	assert.NoError(t, err)
	defer gitRepo.Close()
	startCommit, err := gitRepo.GetCommit(issue.PullRequest.MergeBase)
	assert.NoError(t, err)
	endCommit, err := gitRepo.GetCommit(issue.PullRequest.GetGitHeadRefName())
	assert.NoError(t, err)

	assert.NoError(t, pull_service.LoadCodeComments(db.DefaultContext, gitRepo, issue.Repo, diff, issue.ID, user, startCommit, endCommit, false))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 2)
}

func TestDiff_LoadCommentsWithOutdated(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	assert.NoError(t, issue.LoadRepo(t.Context()))
	assert.NoError(t, issue.LoadPullRequest(t.Context()))

	diff := setupDefaultDiff()
	gitRepo, err := gitrepo.OpenRepository(t.Context(), issue.Repo)
	assert.NoError(t, err)
	defer gitRepo.Close()
	startCommit, err := gitRepo.GetCommit(issue.PullRequest.MergeBase)
	assert.NoError(t, err)
	endCommit, err := gitRepo.GetCommit(issue.PullRequest.GetGitHeadRefName())
	assert.NoError(t, err)

	assert.NoError(t, pull_service.LoadCodeComments(db.DefaultContext, gitRepo, issue.Repo, diff, issue.ID, user, startCommit, endCommit, true))
	assert.Len(t, diff.Files[0].Sections[0].Lines[0].Comments, 3)
}

func Test_reCalculateLineNumber(t *testing.T) {
	hunks := []*git.HunkInfo{
		{
			LeftLine:  0,
			LeftHunk:  0,
			RightLine: 1,
			RightHunk: 3,
		},
	}
	assert.EqualValues(t, 6, pull_service.ReCalculateLineNumber(hunks, 3))

	hunks = []*git.HunkInfo{
		{
			LeftLine:  1,
			LeftHunk:  4,
			RightLine: 1,
			RightHunk: 4,
		},
	}
	assert.EqualValues(t, 0, pull_service.ReCalculateLineNumber(hunks, 4))
	assert.EqualValues(t, 5, pull_service.ReCalculateLineNumber(hunks, 5))
	assert.EqualValues(t, 0, pull_service.ReCalculateLineNumber(hunks, -1))
}
