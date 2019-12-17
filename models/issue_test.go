// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIssue_ReplaceLabels(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(issueID int64, labelIDs []int64) {
		issue := AssertExistsAndLoadBean(t, &Issue{ID: issueID}).(*Issue)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: issue.RepoID}).(*Repository)
		doer := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)

		labels := make([]*Label, len(labelIDs))
		for i, labelID := range labelIDs {
			labels[i] = AssertExistsAndLoadBean(t, &Label{ID: labelID, RepoID: repo.ID}).(*Label)
		}
		assert.NoError(t, issue.ReplaceLabels(labels, doer))
		AssertCount(t, &IssueLabel{IssueID: issueID}, len(labelIDs))
		for _, labelID := range labelIDs {
			AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issueID, LabelID: labelID})
		}
	}

	testSuccess(1, []int64{2})
	testSuccess(1, []int64{1, 2})
	testSuccess(1, []int64{})
}

func TestIssueAPIURL(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	err := issue.LoadAttributes()

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/issues/1", issue.APIURL())
}

func TestGetIssuesByIDs(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	testSuccess := func(expectedIssueIDs []int64, nonExistentIssueIDs []int64) {
		issues, err := GetIssuesByIDs(append(expectedIssueIDs, nonExistentIssueIDs...))
		assert.NoError(t, err)
		actualIssueIDs := make([]int64, len(issues))
		for i, issue := range issues {
			actualIssueIDs[i] = issue.ID
		}
		assert.Equal(t, expectedIssueIDs, actualIssueIDs)

	}
	testSuccess([]int64{1, 2, 3}, []int64{})
	testSuccess([]int64{1, 2, 3}, []int64{NonexistentID})
}

func TestGetParticipantsByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	checkParticipants := func(issueID int64, userIDs []int) {
		participants, err := GetParticipantsByIssueID(issueID)
		if assert.NoError(t, err) {
			participantsIDs := make([]int, len(participants))
			for i, u := range participants {
				participantsIDs[i] = int(u.ID)
			}
			sort.Ints(participantsIDs)
			sort.Ints(userIDs)
			assert.Equal(t, userIDs, participantsIDs)
		}
	}

	// User 1 is issue1 poster (see fixtures/issue.yml)
	// User 2 only labeled issue1 (see fixtures/comment.yml)
	// Users 3 and 5 made actual comments (see fixtures/comment.yml)
	// User 3 is inactive, thus not active participant
	checkParticipants(1, []int{5})
}

func TestIssue_ClearLabels(t *testing.T) {
	var tests = []struct {
		issueID int64
		doerID  int64
	}{
		{1, 2}, // non-pull-request, has labels
		{2, 2}, // pull-request, has labels
		{3, 2}, // pull-request, has no labels
	}
	for _, test := range tests {
		assert.NoError(t, PrepareTestDatabase())
		issue := AssertExistsAndLoadBean(t, &Issue{ID: test.issueID}).(*Issue)
		doer := AssertExistsAndLoadBean(t, &User{ID: test.doerID}).(*User)
		assert.NoError(t, issue.ClearLabels(doer))
		AssertNotExistsBean(t, &IssueLabel{IssueID: test.issueID})
	}
}

func TestUpdateIssueCols(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	issue := AssertExistsAndLoadBean(t, &Issue{}).(*Issue)

	const newTitle = "New Title for unit test"
	issue.Title = newTitle

	prevContent := issue.Content
	issue.Content = "This should have no effect"

	now := time.Now().Unix()
	assert.NoError(t, updateIssueCols(x, issue, "name"))
	then := time.Now().Unix()

	updatedIssue := AssertExistsAndLoadBean(t, &Issue{ID: issue.ID}).(*Issue)
	assert.EqualValues(t, newTitle, updatedIssue.Title)
	assert.EqualValues(t, prevContent, updatedIssue.Content)
	AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestIssues(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	for _, test := range []struct {
		Opts             IssuesOptions
		ExpectedIssueIDs []int64
	}{
		{
			IssuesOptions{
				AssigneeID: 1,
				SortType:   "oldest",
			},
			[]int64{1, 6},
		},
		{
			IssuesOptions{
				RepoIDs:  []int64{1, 3},
				SortType: "oldest",
				Page:     1,
				PageSize: 4,
			},
			[]int64{1, 2, 3, 5},
		},
		{
			IssuesOptions{
				LabelIDs: []int64{1},
				Page:     1,
				PageSize: 4,
			},
			[]int64{2, 1},
		},
		{
			IssuesOptions{
				LabelIDs: []int64{1, 2},
				Page:     1,
				PageSize: 4,
			},
			[]int64{}, // issues with **both** label 1 and 2, none of these issues matches, TODO: add more tests
		},
	} {
		issues, err := Issues(&test.Opts)
		assert.NoError(t, err)
		if assert.Len(t, issues, len(test.ExpectedIssueIDs)) {
			for i, issue := range issues {
				assert.EqualValues(t, test.ExpectedIssueIDs[i], issue.ID)
			}
		}
	}
}

func TestGetUserIssueStats(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	for _, test := range []struct {
		Opts               UserIssueStatsOptions
		ExpectedIssueStats IssueStats
	}{
		{
			UserIssueStatsOptions{
				UserID:     1,
				RepoIDs:    []int64{1},
				FilterMode: FilterModeAll,
			},
			IssueStats{
				YourRepositoriesCount: 0,
				AssignCount:           1,
				CreateCount:           1,
				OpenCount:             0,
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeAssign,
			},
			IssueStats{
				YourRepositoriesCount: 0,
				AssignCount:           2,
				CreateCount:           2,
				OpenCount:             2,
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeCreate,
			},
			IssueStats{
				YourRepositoriesCount: 0,
				AssignCount:           2,
				CreateCount:           2,
				OpenCount:             2,
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:      2,
				UserRepoIDs: []int64{1, 2},
				FilterMode:  FilterModeAll,
				IsClosed:    true,
			},
			IssueStats{
				YourRepositoriesCount: 2,
				AssignCount:           0,
				CreateCount:           2,
				OpenCount:             2,
				ClosedCount:           2,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeMention,
			},
			IssueStats{
				YourRepositoriesCount: 0,
				AssignCount:           2,
				CreateCount:           2,
				OpenCount:             0,
				ClosedCount:           0,
			},
		},
	} {
		stats, err := GetUserIssueStats(test.Opts)
		if !assert.NoError(t, err) {
			continue
		}
		assert.Equal(t, test.ExpectedIssueStats, *stats)
	}
}

func TestIssue_loadTotalTimes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	ms, err := GetIssueByID(2)
	assert.NoError(t, err)
	assert.NoError(t, ms.loadTotalTimes(x))
	assert.Equal(t, int64(3682), ms.TotalTrackedTime)
}

func TestIssue_SearchIssueIDsByKeyword(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	total, ids, err := SearchIssueIDsByKeyword("issue2", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{2}, ids)

	total, ids, err = SearchIssueIDsByKeyword("first", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{1}, ids)

	total, ids, err = SearchIssueIDsByKeyword("for", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, total)
	assert.EqualValues(t, []int64{1, 2, 3, 5}, ids)

	// issue1's comment id 2
	total, ids, err = SearchIssueIDsByKeyword("good", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{1}, ids)
}

func testInsertIssue(t *testing.T, title, content string) {
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	var issue = Issue{
		RepoID:   repo.ID,
		PosterID: user.ID,
		Title:    title,
		Content:  content,
	}
	err := NewIssue(repo, &issue, nil, nil)
	assert.NoError(t, err)

	var newIssue Issue
	has, err := x.ID(issue.ID).Get(&newIssue)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, issue.Title, newIssue.Title)
	assert.EqualValues(t, issue.Content, newIssue.Content)
	// there are 4 issues and max index is 4 on repository 1, so this one should 5
	assert.EqualValues(t, 5, newIssue.Index)

	_, err = x.ID(issue.ID).Delete(new(Issue))
	assert.NoError(t, err)
}

func TestIssue_InsertIssue(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testInsertIssue(t, "my issue1", "special issue's comments?")
	testInsertIssue(t, `my issue2, this is my son's love \n \r \ `, "special issue's '' comments?")
}

func TestIssue_ResolveMentions(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	testSuccess := func(owner, repo, doer string, mentions []string, expected []int64) {
		o := AssertExistsAndLoadBean(t, &User{LowerName: owner}).(*User)
		r := AssertExistsAndLoadBean(t, &Repository{OwnerID: o.ID, LowerName: repo}).(*Repository)
		issue := &Issue{RepoID: r.ID}
		d := AssertExistsAndLoadBean(t, &User{LowerName: doer}).(*User)
		resolved, err := issue.ResolveMentionsByVisibility(DefaultDBContext(), d, mentions)
		assert.NoError(t, err)
		ids := make([]int64, len(resolved))
		for i, user := range resolved {
			ids[i] = user.ID
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		assert.EqualValues(t, expected, ids)
	}

	// Public repo, existing user
	testSuccess("user2", "repo1", "user1", []string{"user5"}, []int64{5})
	// Public repo, non-existing user
	testSuccess("user2", "repo1", "user1", []string{"nonexisting"}, []int64{})
	// Public repo, doer
	testSuccess("user2", "repo1", "user1", []string{"user1"}, []int64{})
	// Private repo, team member
	testSuccess("user17", "big_test_private_4", "user20", []string{"user2"}, []int64{2})
	// Private repo, not a team member
	testSuccess("user17", "big_test_private_4", "user20", []string{"user5"}, []int64{})
	// Private repo, whole team
	testSuccess("user17", "big_test_private_4", "user15", []string{"owners"}, []int64{18})
}
