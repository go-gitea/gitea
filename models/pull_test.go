// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strconv"
	"testing"
	"time"

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

// TODO TestPullRequest_APIFormat

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
	pr, err := GetUnmergedPullRequest(1, 1, "develop", "master")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.ID)

	pr, err = GetUnmergedPullRequest(1, 9223372036854775807, "branch1", "master")
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetUnmergedPullRequestsByHeadInfo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, err := GetUnmergedPullRequestsByHeadInfo(1, "develop")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	for _, pr := range prs {
		assert.Equal(t, int64(1), pr.HeadRepoID)
		assert.Equal(t, "develop", pr.HeadBranch)
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

	pr, err = GetPullRequestByIndex(9223372036854775807, 9223372036854775807)
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

	pr, err = GetPullRequestByIssueID(9223372036854775807)
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

// TODO TestPullRequest_UpdatePatch

// TODO TestPullRequest_PushToBaseRepo

func TestPullRequest_AddToTaskQueue(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	pr.AddToTaskQueue()

	select {
	case id := <-pullRequestQueue.Queue():
		assert.EqualValues(t, strconv.FormatInt(pr.ID, 10), id)
	case <-time.After(time.Second):
		assert.Fail(t, "Timeout: nothing was added to pullRequestQueue")
	}

	assert.True(t, pullRequestQueue.Exist(pr.ID))
	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.Equal(t, PullRequestStatusChecking, pr.Status)
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

func TestChangeUsernameInPullRequests(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	const newUsername = "newusername"
	assert.NoError(t, ChangeUsernameInPullRequests("user1", newUsername))

	prs := make([]*PullRequest, 0, 10)
	assert.NoError(t, x.Where("head_user_name = ?", newUsername).Find(&prs))
	assert.Len(t, prs, 2)
	for _, pr := range prs {
		assert.Equal(t, newUsername, pr.HeadUserName)
	}
	CheckConsistencyFor(t, &PullRequest{})
}

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

func closeActivePullRequests(t *testing.T, pr *PullRequest, repo *Repository, branchName string) {
	assert.NoError(t, repo.GetOwner())

	CloseActivePullRequests(repo.Owner, repo, branchName)
	pr.Issue = nil
	pr.LoadIssue()
	assert.Equal(t, true, pr.Issue.IsClosed)
}

func TestPullRequest_CloseActivePullRequests(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Delete head branch.
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.GetHeadRepo()
	closeActivePullRequests(t, pr, pr.HeadRepo, pr.HeadBranch)

	// Reopen pull request.
	assert.NoError(t, pr.Issue.ChangeStatus(pr.HeadRepo.Owner, false))

	// Delete base branch.
	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.GetBaseRepo()
	closeActivePullRequests(t, pr, pr.BaseRepo, pr.BaseBranch)
}

func createTestPullRequest(t *testing.T, repo *Repository, user *User, baseBranch string, headBranch string) *PullRequest {
	prIssue := &Issue{
		RepoID:   repo.ID,
		Repo:     repo,
		Title:    "test",
		PosterID: user.ID,
		Poster:   user,
	}
	pr := &PullRequest{
		HeadRepoID:   repo.ID,
		BaseRepoID:   repo.ID,
		HeadRepo:     repo,
		BaseRepo:     repo,
		HeadUserName: user.Name,
		HeadBranch:   headBranch,
		BaseBranch:   baseBranch,
		Type:         PullRequestGitea,
	}
	labelIDs := make([]int64, 0)
	uuids := make([]string, 0)
	patch := make([]byte, 0)
	assigneeIDs := make([]int64, 0)
	err := NewPullRequest(repo, prIssue, labelIDs, uuids, pr, patch, assigneeIDs)
	assert.NoError(t, err)
	return pr
}

func TestPullRequest_CloseActivePullRequestsDependent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	// Create a test pull request.
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	user := AssertExistsAndLoadBean(t, &User{ID: 1}).(*User)
	pr1 := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr2 := createTestPullRequest(t, repo, user, "develop", "master")
	pr1.LoadIssue()
	pr2.LoadIssue()

	// Add a dependency from pr2 to pr1.
	err := CreateIssueDependency(user, pr2.Issue, pr1.Issue)
	assert.NoError(t, err)

	closeActivePullRequests(t, pr2, pr2.BaseRepo, pr2.BaseBranch)
	pr1.Issue = nil
	pr1.LoadIssue()
	assert.Equal(t, true, pr1.Issue.IsClosed)
}
