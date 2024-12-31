// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_LoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadAttributes(db.DefaultContext))
	assert.NotNil(t, pr.Merger)
	assert.Equal(t, pr.MergerID, pr.Merger.ID)
}

func TestPullRequest_LoadIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(db.DefaultContext))
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
	assert.NoError(t, pr.LoadIssue(db.DefaultContext))
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
}

func TestPullRequest_LoadBaseRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadBaseRepo(db.DefaultContext))
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
	assert.NoError(t, pr.LoadBaseRepo(db.DefaultContext))
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
}

func TestPullRequest_LoadHeadRepo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadHeadRepo(db.DefaultContext))
	assert.NotNil(t, pr.HeadRepo)
	assert.Equal(t, pr.HeadRepoID, pr.HeadRepo.ID)
}

// TODO TestMerge

// TODO TestNewPullRequest

func TestPullRequestsNewest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	prs, count, err := issues_model.PullRequests(db.DefaultContext, 1, &issues_model.PullRequestsOptions{
		ListOptions: db.ListOptions{
			Page: 1,
		},
		State:    "open",
		SortType: "newest",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
	if assert.Len(t, prs, 3) {
		assert.EqualValues(t, 5, prs[0].ID)
		assert.EqualValues(t, 2, prs[1].ID)
		assert.EqualValues(t, 1, prs[2].ID)
	}
}

func TestLoadRequestedReviewers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(db.DefaultContext))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(db.DefaultContext))
	assert.Empty(t, pull.RequestedReviewers)

	user1, err := user_model.GetUserByID(db.DefaultContext, 1)
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(db.DefaultContext, issue, user1, &user_model.User{})
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	assert.NoError(t, pull.LoadRequestedReviewers(db.DefaultContext))
	assert.Len(t, pull.RequestedReviewers, 6)

	comment, err = issues_model.RemoveReviewRequest(db.DefaultContext, issue, user1, &user_model.User{})
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	pull.RequestedReviewers = nil
	assert.NoError(t, pull.LoadRequestedReviewers(db.DefaultContext))
	assert.Empty(t, pull.RequestedReviewers)
}

func TestPullRequestsOldest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	prs, count, err := issues_model.PullRequests(db.DefaultContext, 1, &issues_model.PullRequestsOptions{
		ListOptions: db.ListOptions{
			Page: 1,
		},
		State:    "open",
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
	if assert.Len(t, prs, 3) {
		assert.EqualValues(t, 1, prs[0].ID)
		assert.EqualValues(t, 2, prs[1].ID)
		assert.EqualValues(t, 5, prs[2].ID)
	}
}

func TestGetUnmergedPullRequest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetUnmergedPullRequest(db.DefaultContext, 1, 1, "branch2", "master", issues_model.PullRequestFlowGithub)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.ID)

	_, err = issues_model.GetUnmergedPullRequest(db.DefaultContext, 1, 9223372036854775807, "branch1", "master", issues_model.PullRequestFlowGithub)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func TestHasUnmergedPullRequestsByHeadInfo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(db.DefaultContext, 1, "branch2")
	assert.NoError(t, err)
	assert.True(t, exist)

	exist, err = issues_model.HasUnmergedPullRequestsByHeadInfo(db.DefaultContext, 1, "not_exist_branch")
	assert.NoError(t, err)
	assert.False(t, exist)
}

func TestGetUnmergedPullRequestsByHeadInfo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	prs, err := issues_model.GetUnmergedPullRequestsByHeadInfo(db.DefaultContext, 1, "branch2")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	for _, pr := range prs {
		assert.Equal(t, int64(1), pr.HeadRepoID)
		assert.Equal(t, "branch2", pr.HeadBranch)
	}
}

func TestGetUnmergedPullRequestsByBaseInfo(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	prs, err := issues_model.GetUnmergedPullRequestsByBaseInfo(db.DefaultContext, 1, "master")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	pr := prs[0]
	assert.Equal(t, int64(2), pr.ID)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, "master", pr.BaseBranch)
}

func TestGetPullRequestByIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetPullRequestByIndex(db.DefaultContext, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, int64(2), pr.Index)

	_, err = issues_model.GetPullRequestByIndex(db.DefaultContext, 9223372036854775807, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))

	_, err = issues_model.GetPullRequestByIndex(db.DefaultContext, 1, 0)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetPullRequestByID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.ID)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = issues_model.GetPullRequestByID(db.DefaultContext, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByIssueID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetPullRequestByIssueID(db.DefaultContext, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = issues_model.GetPullRequestByIssueID(db.DefaultContext, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func TestPullRequest_Update(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	pr.BaseBranch = "baseBranch"
	pr.HeadBranch = "headBranch"
	pr.Update(db.DefaultContext)

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: pr.ID})
	assert.Equal(t, "baseBranch", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	unittest.CheckConsistencyFor(t, pr)
}

func TestPullRequest_UpdateCols(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := &issues_model.PullRequest{
		ID:         1,
		BaseBranch: "baseBranch",
		HeadBranch: "headBranch",
	}
	assert.NoError(t, pr.UpdateCols(db.DefaultContext, "head_branch"))

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.Equal(t, "master", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	unittest.CheckConsistencyFor(t, pr)
}

// TODO TestAddTestPullRequestTask

func TestPullRequest_IsWorkInProgress(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	pr.LoadIssue(db.DefaultContext)

	assert.False(t, pr.IsWorkInProgress(db.DefaultContext))

	pr.Issue.Title = "WIP: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress(db.DefaultContext))

	pr.Issue.Title = "[wip]: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress(db.DefaultContext))
}

func TestPullRequest_GetWorkInProgressPrefixWorkInProgress(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	pr.LoadIssue(db.DefaultContext)

	assert.Empty(t, pr.GetWorkInProgressPrefix(db.DefaultContext))

	original := pr.Issue.Title
	pr.Issue.Title = "WIP: " + original
	assert.Equal(t, "WIP:", pr.GetWorkInProgressPrefix(db.DefaultContext))

	pr.Issue.Title = "[wip] " + original
	assert.Equal(t, "[wip]", pr.GetWorkInProgressPrefix(db.DefaultContext))
}

func TestDeleteOrphanedObjects(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	countBefore, err := db.GetEngine(db.DefaultContext).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(db.DefaultContext).Insert(&issues_model.PullRequest{IssueID: 1000}, &issues_model.PullRequest{IssueID: 1001}, &issues_model.PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := db.CountOrphanedObjects(db.DefaultContext, "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = db.DeleteOrphanedObjects(db.DefaultContext, "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(db.DefaultContext).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)
	assert.EqualValues(t, countBefore, countAfter)
}

func TestParseCodeOwnersLine(t *testing.T) {
	type CodeOwnerTest struct {
		Line   string
		Tokens []string
	}

	given := []CodeOwnerTest{
		{Line: "", Tokens: nil},
		{Line: "# comment", Tokens: []string{}},
		{Line: "!.* @user1 @org1/team1", Tokens: []string{"!.*", "@user1", "@org1/team1"}},
		{Line: `.*\\.js @user2 #comment`, Tokens: []string{`.*\.js`, "@user2"}},
		{Line: `docs/(aws|google|azure)/[^/]*\\.(md|txt) @org3 @org2/team2`, Tokens: []string{`docs/(aws|google|azure)/[^/]*\.(md|txt)`, "@org3", "@org2/team2"}},
		{Line: `\#path @org3`, Tokens: []string{`#path`, "@org3"}},
		{Line: `path\ with\ spaces/ @org3`, Tokens: []string{`path with spaces/`, "@org3"}},
	}

	for _, g := range given {
		tokens := issues_model.TokenizeCodeOwnersLine(g.Line)
		assert.Equal(t, g.Tokens, tokens, "Codeowners tokenizer failed")
	}
}

func TestGetApprovers(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5})
	// Official reviews are already deduplicated. Allow unofficial reviews
	// to assert that there are no duplicated approvers.
	setting.Repository.PullRequest.DefaultMergeMessageOfficialApproversOnly = false
	approvers := pr.GetApprovers(db.DefaultContext)
	expected := "Reviewed-by: User Five <user5@example.com>\nReviewed-by: Org Six <org6@example.com>\n"
	assert.EqualValues(t, expected, approvers)
}

func TestGetPullRequestByMergedCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	pr, err := issues_model.GetPullRequestByMergedCommit(db.DefaultContext, 1, "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, pr.ID)

	_, err = issues_model.GetPullRequestByMergedCommit(db.DefaultContext, 0, "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3")
	assert.ErrorAs(t, err, &issues_model.ErrPullRequestNotExist{})
	_, err = issues_model.GetPullRequestByMergedCommit(db.DefaultContext, 1, "")
	assert.ErrorAs(t, err, &issues_model.ErrPullRequestNotExist{})
}

func TestMigrate_InsertPullRequests(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	i := &issues_model.Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    "title1",
		Content:  "issuecontent1",
		IsPull:   true,
		PosterID: owner.ID,
		Poster:   owner,
	}

	p := &issues_model.PullRequest{
		Issue: i,
	}

	err := issues_model.InsertPullRequests(db.DefaultContext, p)
	assert.NoError(t, err)

	_ = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: i.ID})

	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.PullRequest{})
}
