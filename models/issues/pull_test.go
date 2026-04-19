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
	"github.com/stretchr/testify/require"
)

func TestPullRequest(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	t.Run("LoadAttributes", testPullRequestLoadAttributes)
	t.Run("LoadIssue", testPullRequestLoadIssue)
	t.Run("LoadBaseRepo", testPullRequestLoadBaseRepo)
	t.Run("LoadHeadRepo", testPullRequestLoadHeadRepo)
	t.Run("PullRequestsNewest", testPullRequestsNewest)
	t.Run("PullRequestsOldest", testPullRequestsOldest)
	t.Run("GetUnmergedPullRequest", testGetUnmergedPullRequest)
	t.Run("HasUnmergedPullRequestsByHeadInfo", testHasUnmergedPullRequestsByHeadInfo)
	t.Run("GetUnmergedPullRequestsByHeadInfo", testGetUnmergedPullRequestsByHeadInfo)
	t.Run("GetUnmergedPullRequestsByBaseInfo", testGetUnmergedPullRequestsByBaseInfo)
	t.Run("GetPullRequestByIndex", testGetPullRequestByIndex)
	t.Run("GetPullRequestByID", testGetPullRequestByID)
	t.Run("GetPullRequestByIssueID", testGetPullRequestByIssueID)
	t.Run("PullRequest_UpdateCols", testPullRequestUpdateCols)
	t.Run("PullRequest_IsWorkInProgress", testPullRequestIsWorkInProgress)
	t.Run("PullRequest_GetWorkInProgressPrefixWorkInProgress", testPullRequestGetWorkInProgressPrefixWorkInProgress)
	t.Run("DeleteOrphanedObjects", testDeleteOrphanedObjects)
	t.Run("ParseCodeOwnersLine", testParseCodeOwnersLine)
	t.Run("CodeOwnerAbsolutePathPatterns", testCodeOwnerAbsolutePathPatterns)
	t.Run("GetApprovers", testGetApprovers)
	t.Run("GetPullRequestByMergedCommit", testGetPullRequestByMergedCommit)
	t.Run("Migrate_InsertPullRequests", testMigrateInsertPullRequests)
	t.Run("PullRequestsClosedRecentSortType", testPullRequestsClosedRecentSortType)
	t.Run("LoadRequestedReviewers", testLoadRequestedReviewers)
}

func testPullRequestLoadAttributes(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadAttributes(t.Context()))
	assert.NotNil(t, pr.Merger)
	assert.Equal(t, pr.MergerID, pr.Merger.ID)
}

func testPullRequestLoadIssue(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
	assert.NoError(t, pr.LoadIssue(t.Context()))
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
}

func testPullRequestLoadBaseRepo(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
	assert.NoError(t, pr.LoadBaseRepo(t.Context()))
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
}

func testPullRequestLoadHeadRepo(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.NoError(t, pr.LoadHeadRepo(t.Context()))
	assert.NotNil(t, pr.HeadRepo)
	assert.Equal(t, pr.HeadRepoID, pr.HeadRepo.ID)
}

// TODO TestMerge

// TODO TestNewPullRequest

func testPullRequestsNewest(t *testing.T) {
	prs, count, err := issues_model.PullRequests(t.Context(), 1, &issues_model.PullRequestsOptions{
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

func testPullRequestsClosedRecentSortType(t *testing.T) {
	// Issue ID | Closed At.  | Updated At
	//    2     | 1707270001  | 1707270001
	//    3     | 1707271000  | 1707279999
	//    11    | 1707279999  | 1707275555
	tests := []struct {
		sortType             string
		expectedIssueIDOrder []int64
	}{
		{"recentupdate", []int64{3, 11, 2}},
		{"recentclose", []int64{11, 3, 2}},
	}

	_, err := db.Exec(t.Context(), "UPDATE issue SET closed_unix = 1707270001, updated_unix = 1707270001, is_closed = true WHERE id = 2")
	require.NoError(t, err)
	_, err = db.Exec(t.Context(), "UPDATE issue SET closed_unix = 1707271000, updated_unix = 1707279999, is_closed = true WHERE id = 3")
	require.NoError(t, err)
	_, err = db.Exec(t.Context(), "UPDATE issue SET closed_unix = 1707279999, updated_unix = 1707275555, is_closed = true WHERE id = 11")
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.sortType, func(t *testing.T) {
			prs, _, err := issues_model.PullRequests(t.Context(), 1, &issues_model.PullRequestsOptions{
				ListOptions: db.ListOptions{
					Page: 1,
				},
				State:    "closed",
				SortType: test.sortType,
			})
			require.NoError(t, err)

			if assert.Len(t, prs, len(test.expectedIssueIDOrder)) {
				for i := range test.expectedIssueIDOrder {
					assert.Equal(t, test.expectedIssueIDOrder[i], prs[i].IssueID)
				}
			}
		})
	}
}

func testLoadRequestedReviewers(t *testing.T) {
	pull := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	assert.NoError(t, pull.LoadIssue(t.Context()))
	issue := pull.Issue
	assert.NoError(t, issue.LoadRepo(t.Context()))
	assert.Empty(t, pull.RequestedReviewers)

	user1, err := user_model.GetUserByID(t.Context(), 1)
	assert.NoError(t, err)

	comment, err := issues_model.AddReviewRequest(t.Context(), issue, user1, &user_model.User{}, false)
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	assert.NoError(t, pull.LoadRequestedReviewers(t.Context()))
	assert.Len(t, pull.RequestedReviewers, 6)

	comment, err = issues_model.RemoveReviewRequest(t.Context(), issue, user1, &user_model.User{})
	assert.NoError(t, err)
	assert.NotNil(t, comment)

	pull.RequestedReviewers = nil
	assert.NoError(t, pull.LoadRequestedReviewers(t.Context()))
	assert.Empty(t, pull.RequestedReviewers)
}

func testPullRequestsOldest(t *testing.T) {
	prs, count, err := issues_model.PullRequests(t.Context(), 1, &issues_model.PullRequestsOptions{
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

func testGetUnmergedPullRequest(t *testing.T) {
	pr, err := issues_model.GetUnmergedPullRequest(t.Context(), 1, 1, "branch2", "master", issues_model.PullRequestFlowGithub)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.ID)

	_, err = issues_model.GetUnmergedPullRequest(t.Context(), 1, 9223372036854775807, "branch1", "master", issues_model.PullRequestFlowGithub)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func testHasUnmergedPullRequestsByHeadInfo(t *testing.T) {
	exist, err := issues_model.HasUnmergedPullRequestsByHeadInfo(t.Context(), 1, "branch2")
	assert.NoError(t, err)
	assert.True(t, exist)

	exist, err = issues_model.HasUnmergedPullRequestsByHeadInfo(t.Context(), 1, "not_exist_branch")
	assert.NoError(t, err)
	assert.False(t, exist)
}

func testGetUnmergedPullRequestsByHeadInfo(t *testing.T) {
	prs, err := issues_model.GetUnmergedPullRequestsByHeadInfo(t.Context(), 1, "branch2")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	for _, pr := range prs {
		assert.Equal(t, int64(1), pr.HeadRepoID)
		assert.Equal(t, "branch2", pr.HeadBranch)
	}
}

func testGetUnmergedPullRequestsByBaseInfo(t *testing.T) {
	prs, err := issues_model.GetUnmergedPullRequestsByBaseInfo(t.Context(), 1, "master")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	pr := prs[0]
	assert.Equal(t, int64(2), pr.ID)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, "master", pr.BaseBranch)
}

func testGetPullRequestByIndex(t *testing.T) {
	pr, err := issues_model.GetPullRequestByIndex(t.Context(), 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, int64(2), pr.Index)

	_, err = issues_model.GetPullRequestByIndex(t.Context(), 9223372036854775807, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))

	_, err = issues_model.GetPullRequestByIndex(t.Context(), 1, 0)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func testGetPullRequestByID(t *testing.T) {
	pr, err := issues_model.GetPullRequestByID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.ID)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = issues_model.GetPullRequestByID(t.Context(), 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func testGetPullRequestByIssueID(t *testing.T) {
	pr, err := issues_model.GetPullRequestByIssueID(t.Context(), 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = issues_model.GetPullRequestByIssueID(t.Context(), 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, issues_model.IsErrPullRequestNotExist(err))
}

func testPullRequestUpdateCols(t *testing.T) {
	pr := &issues_model.PullRequest{
		ID:         1,
		BaseBranch: "baseBranch",
		HeadBranch: "headBranch",
	}
	assert.NoError(t, pr.UpdateCols(t.Context(), "head_branch"))

	pr = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 1})
	assert.Equal(t, "master", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	unittest.CheckConsistencyFor(t, pr)
}

// TODO TestAddTestPullRequestTask

func testPullRequestIsWorkInProgress(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	pr.LoadIssue(t.Context())

	assert.False(t, pr.IsWorkInProgress(t.Context()))

	pr.Issue.Title = "WIP: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress(t.Context()))

	pr.Issue.Title = "[wip]: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress(t.Context()))
}

func testPullRequestGetWorkInProgressPrefixWorkInProgress(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 2})
	pr.LoadIssue(t.Context())

	assert.Empty(t, pr.GetWorkInProgressPrefix(t.Context()))

	original := pr.Issue.Title
	pr.Issue.Title = "WIP: " + original
	assert.Equal(t, "WIP:", pr.GetWorkInProgressPrefix(t.Context()))

	pr.Issue.Title = "[wip] " + original
	assert.Equal(t, "[wip]", pr.GetWorkInProgressPrefix(t.Context()))
}

func testDeleteOrphanedObjects(t *testing.T) {
	countBefore, err := db.GetEngine(t.Context()).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)

	_, err = db.GetEngine(t.Context()).Insert(&issues_model.PullRequest{IssueID: 1000}, &issues_model.PullRequest{IssueID: 1001}, &issues_model.PullRequest{IssueID: 1003})
	assert.NoError(t, err)

	orphaned, err := db.CountOrphanedObjects(t.Context(), "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, orphaned)

	err = db.DeleteOrphanedObjects(t.Context(), "pull_request", "issue", "pull_request.issue_id=issue.id")
	assert.NoError(t, err)

	countAfter, err := db.GetEngine(t.Context()).Count(&issues_model.PullRequest{})
	assert.NoError(t, err)
	assert.Equal(t, countBefore, countAfter)
}

func testParseCodeOwnersLine(t *testing.T) {
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
		{Line: `/docs/.*\\.md @user1`, Tokens: []string{`/docs/.*\.md`, "@user1"}},
		{Line: `!/assets/.*\\.(bin|exe|msi) @user1`, Tokens: []string{`!/assets/.*\.(bin|exe|msi)`, "@user1"}},
	}

	for _, g := range given {
		tokens := issues_model.TokenizeCodeOwnersLine(g.Line)
		assert.Equal(t, g.Tokens, tokens, "Codeowners tokenizer failed")
	}
}

func testCodeOwnerAbsolutePathPatterns(t *testing.T) {
	type testCase struct {
		content  string
		file     string
		expected bool
	}

	cases := []testCase{
		// Absolute path pattern should match (leading "/" stripped)
		{content: "/README.md @user5\n", file: "README.md", expected: true},
		// Absolute path pattern in subdirectory
		{content: "/docs/.* @user5\n", file: "docs/foo.md", expected: true},
		// Absolute path should not match nested paths it shouldn't
		{content: "/docs/.* @user5\n", file: "other/docs/foo.md", expected: false},
		// Relative path still works
		{content: "README.md @user5\n", file: "README.md", expected: true},
		// Negated absolute path pattern
		{content: "!/.* @user5\n", file: "README.md", expected: false},
	}

	for _, c := range cases {
		rules, _ := issues_model.GetCodeOwnersFromContent(t.Context(), c.content)
		require.NotEmpty(t, rules)
		rule := rules[0]
		regexpMatched, _ := rule.Rule.MatchString(c.file)
		ruleMatched := regexpMatched == !rule.Negative
		assert.Equal(t, c.expected, ruleMatched, "pattern %q against file %q", c.content, c.file)
	}
}

func testGetApprovers(t *testing.T) {
	pr := unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{ID: 5})
	// Official reviews are already deduplicated. Allow unofficial reviews
	// to assert that there are no duplicated approvers.
	setting.Repository.PullRequest.DefaultMergeMessageOfficialApproversOnly = false
	approvers := pr.GetApprovers(t.Context())
	expected := "Reviewed-by: User Five <user5@example.com>\nReviewed-by: Org Six <org6@example.com>\n"
	assert.Equal(t, expected, approvers)
}

func testGetPullRequestByMergedCommit(t *testing.T) {
	pr, err := issues_model.GetPullRequestByMergedCommit(t.Context(), 1, "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3")
	assert.NoError(t, err)
	assert.EqualValues(t, 1, pr.ID)

	_, err = issues_model.GetPullRequestByMergedCommit(t.Context(), 0, "1a8823cd1a9549fde083f992f6b9b87a7ab74fb3")
	assert.ErrorAs(t, err, &issues_model.ErrPullRequestNotExist{})
	_, err = issues_model.GetPullRequestByMergedCommit(t.Context(), 1, "")
	assert.ErrorAs(t, err, &issues_model.ErrPullRequestNotExist{})
}

func testMigrateInsertPullRequests(t *testing.T) {
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

	err := issues_model.InsertPullRequests(t.Context(), p)
	assert.NoError(t, err)

	_ = unittest.AssertExistsAndLoadBean(t, &issues_model.PullRequest{IssueID: i.ID})

	unittest.CheckConsistencyFor(t, &issues_model.Issue{}, &issues_model.PullRequest{})
}
