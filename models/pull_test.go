// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_LoadAttributes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NotNil(t, pr.Merger)
	assert.Equal(t, pr.MergerID, pr.Merger.ID)
}

func TestPullRequest_LoadIssue(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadIssue())
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
	assert.NoError(t, pr.LoadIssue())
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
}

func TestPullRequest_APIFormat(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NoError(t, pr.LoadIssue())
	apiPullRequest := pr.APIFormat()
	assert.NotNil(t, apiPullRequest)
	assert.Nil(t, apiPullRequest.Head)
}

func TestPullRequest_GetBaseRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.GetBaseRepo())
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
	assert.NoError(t, pr.GetBaseRepo())
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
}

func TestPullRequest_GetHeadRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.GetHeadRepo())
	assert.NotNil(t, pr.HeadRepo)
	assert.Equal(t, pr.HeadRepoID, pr.HeadRepo.ID)
}

// TODO TestMerge

// TODO TestNewPullRequest

func TestPullRequestsNewest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, count, err := PullRequests(1, &PullRequestsOptions{
		Page:     1,
		State:    "open",
		SortType: "newest",
		Labels:   []string{},
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	if assert.Len(t, prs, 2) {
		assert.Equal(t, int64(2), prs[0].ID)
		assert.Equal(t, int64(1), prs[1].ID)
	}
}

func TestPullRequestsOldest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, count, err := PullRequests(1, &PullRequestsOptions{
		Page:     1,
		State:    "open",
		SortType: "oldest",
		Labels:   []string{},
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
	if assert.Len(t, prs, 2) {
		assert.Equal(t, int64(1), prs[0].ID)
		assert.Equal(t, int64(2), prs[1].ID)
	}
}

func TestGetUnmergedPullRequest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetUnmergedPullRequest(1, 1, "branch2", "master")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.ID)

	_, err = GetUnmergedPullRequest(1, 9223372036854775807, "branch1", "master")
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetUnmergedPullRequestsByHeadInfo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, err := GetUnmergedPullRequestsByHeadInfo(1, "branch2")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	for _, pr := range prs {
		assert.Equal(t, int64(1), pr.HeadRepoID)
		assert.Equal(t, "branch2", pr.HeadBranch)
	}
}

func TestGetUnmergedPullRequestsByBaseInfo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, err := GetUnmergedPullRequestsByBaseInfo(1, "master")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	pr := prs[0]
	assert.Equal(t, int64(2), pr.ID)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, "master", pr.BaseBranch)
}

func TestGetPullRequestByIndex(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByIndex(1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, int64(2), pr.Index)

	_, err = GetPullRequestByIndex(9223372036854775807, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.ID)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = GetPullRequestByID(9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByIssueID(2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = GetPullRequestByIssueID(9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestPullRequest_Update(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	pr.BaseBranch = "baseBranch"
	pr.HeadBranch = "headBranch"
	pr.Update()

	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: pr.ID}).(*PullRequest)
	assert.Equal(t, "baseBranch", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	CheckConsistencyFor(t, pr)
}

func TestPullRequest_UpdateCols(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := &PullRequest{
		ID:         1,
		BaseBranch: "baseBranch",
		HeadBranch: "headBranch",
	}
	pr.UpdateCols("head_branch")

	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.Equal(t, "master", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	CheckConsistencyFor(t, pr)
}

func TestPullRequestList_LoadAttributes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	prs := []*PullRequest{
		AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest),
		AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest),
	}
	assert.NoError(t, PullRequestList(prs).LoadAttributes())
	for _, pr := range prs {
		assert.NotNil(t, pr.Issue)
		assert.Equal(t, pr.IssueID, pr.Issue.ID)
	}

	assert.NoError(t, PullRequestList([]*PullRequest{}).LoadAttributes())
}

// TODO TestAddTestPullRequestTask

func TestPullRequest_IsWorkInProgress(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.LoadIssue()

	assert.False(t, pr.IsWorkInProgress())

	pr.Issue.Title = "WIP: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress())

	pr.Issue.Title = "[wip]: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress())
}

func TestPullRequest_GetWorkInProgressPrefixWorkInProgress(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.LoadIssue()

	assert.Empty(t, pr.GetWorkInProgressPrefix())

	original := pr.Issue.Title
	pr.Issue.Title = "WIP: " + original
	assert.Equal(t, "WIP:", pr.GetWorkInProgressPrefix())

	pr.Issue.Title = "[wip] " + original
	assert.Equal(t, "[wip]", pr.GetWorkInProgressPrefix())
}
