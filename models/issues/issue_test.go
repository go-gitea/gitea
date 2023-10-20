// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestIssue_ReplaceLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(issueID int64, labelIDs, expectedLabelIDs []int64) {
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issueID})
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

		labels := make([]*issues_model.Label, len(labelIDs))
		for i, labelID := range labelIDs {
			labels[i] = unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: labelID, RepoID: repo.ID})
		}
		assert.NoError(t, issues_model.ReplaceIssueLabels(db.DefaultContext, issue, labels, doer))
		unittest.AssertCount(t, &issues_model.IssueLabel{IssueID: issueID}, len(expectedLabelIDs))
		for _, labelID := range expectedLabelIDs {
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issueID, LabelID: labelID})
		}
	}

	testSuccess(1, []int64{2}, []int64{2})
	testSuccess(1, []int64{1, 2}, []int64{1, 2})
	testSuccess(1, []int64{}, []int64{})

	// mutually exclusive scoped labels 7 and 8
	testSuccess(18, []int64{6, 7}, []int64{6, 7})
	testSuccess(18, []int64{7, 8}, []int64{8})
	testSuccess(18, []int64{6, 8, 7}, []int64{6, 7})
}

func Test_GetIssueIDsByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := issues_model.GetIssueIDsByRepoID(db.DefaultContext, 1)
	assert.NoError(t, err)
	assert.Len(t, ids, 5)
}

func TestIssueAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	err := issue.LoadAttributes(db.DefaultContext)

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/issues/1", issue.APIURL(db.DefaultContext))
}

func TestGetIssuesByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(expectedIssueIDs, nonExistentIssueIDs []int64) {
		issues, err := issues_model.GetIssuesByIDs(db.DefaultContext, append(expectedIssueIDs, nonExistentIssueIDs...), true)
		assert.NoError(t, err)
		actualIssueIDs := make([]int64, len(issues))
		for i, issue := range issues {
			actualIssueIDs[i] = issue.ID
		}
		assert.Equal(t, expectedIssueIDs, actualIssueIDs)
	}
	testSuccess([]int64{1, 2, 3}, []int64{})
	testSuccess([]int64{1, 2, 3}, []int64{unittest.NonexistentID})
	testSuccess([]int64{3, 2, 1}, []int64{})
}

func TestGetParticipantIDsByIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	checkParticipants := func(issueID int64, userIDs []int) {
		issue, err := issues_model.GetIssueByID(db.DefaultContext, issueID)
		assert.NoError(t, err)
		participants, err := issue.GetParticipantIDsByIssue(db.DefaultContext)
		if assert.NoError(t, err) {
			participantsIDs := make([]int, len(participants))
			for i, uid := range participants {
				participantsIDs[i] = int(uid)
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
	checkParticipants(1, []int{1, 5})
}

func TestIssue_ClearLabels(t *testing.T) {
	tests := []struct {
		issueID int64
		doerID  int64
	}{
		{1, 2}, // non-pull-request, has labels
		{2, 2}, // pull-request, has labels
		{3, 2}, // pull-request, has no labels
	}
	for _, test := range tests {
		assert.NoError(t, unittest.PrepareTestDatabase())
		issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: test.issueID})
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.doerID})
		assert.NoError(t, issues_model.ClearIssueLabels(db.DefaultContext, issue, doer))
		unittest.AssertNotExistsBean(t, &issues_model.IssueLabel{IssueID: test.issueID})
	}
}

func TestUpdateIssueCols(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{})

	const newTitle = "New Title for unit test"
	issue.Title = newTitle

	prevContent := issue.Content
	issue.Content = "This should have no effect"

	now := time.Now().Unix()
	assert.NoError(t, issues_model.UpdateIssueCols(db.DefaultContext, issue, "name"))
	then := time.Now().Unix()

	updatedIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
	assert.EqualValues(t, newTitle, updatedIssue.Title)
	assert.EqualValues(t, prevContent, updatedIssue.Content)
	unittest.AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	for _, test := range []struct {
		Opts             issues_model.IssuesOptions
		ExpectedIssueIDs []int64
	}{
		{
			issues_model.IssuesOptions{
				AssigneeID: 1,
				SortType:   "oldest",
			},
			[]int64{1, 6},
		},
		{
			issues_model.IssuesOptions{
				RepoCond: builder.In("repo_id", 1, 3),
				SortType: "oldest",
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{1, 2, 3, 5},
		},
		{
			issues_model.IssuesOptions{
				LabelIDs: []int64{1},
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{2, 1},
		},
		{
			issues_model.IssuesOptions{
				LabelIDs: []int64{1, 2},
				Paginator: &db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{}, // issues with **both** label 1 and 2, none of these issues matches, TODO: add more tests
		},
		{
			issues_model.IssuesOptions{
				MilestoneIDs: []int64{1},
			},
			[]int64{2},
		},
	} {
		issues, err := issues_model.Issues(db.DefaultContext, &test.Opts)
		assert.NoError(t, err)
		if assert.Len(t, issues, len(test.ExpectedIssueIDs)) {
			for i, issue := range issues {
				assert.EqualValues(t, test.ExpectedIssueIDs[i], issue.ID)
			}
		}
	}
}

func TestIssue_loadTotalTimes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ms, err := issues_model.GetIssueByID(db.DefaultContext, 2)
	assert.NoError(t, err)
	assert.NoError(t, ms.LoadTotalTimes(db.DefaultContext))
	assert.Equal(t, int64(3682), ms.TotalTrackedTime)
}

func TestGetRepoIDsForIssuesOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	for _, test := range []struct {
		Opts            issues_model.IssuesOptions
		ExpectedRepoIDs []int64
	}{
		{
			issues_model.IssuesOptions{
				AssigneeID: 2,
			},
			[]int64{3, 32},
		},
		{
			issues_model.IssuesOptions{
				RepoCond: builder.In("repo_id", 1, 2),
			},
			[]int64{1, 2},
		},
	} {
		repoIDs, err := issues_model.GetRepoIDsForIssuesOptions(db.DefaultContext, &test.Opts, user)
		assert.NoError(t, err)
		if assert.Len(t, repoIDs, len(test.ExpectedRepoIDs)) {
			for i, repoID := range repoIDs {
				assert.EqualValues(t, test.ExpectedRepoIDs[i], repoID)
			}
		}
	}
}

func testInsertIssue(t *testing.T, title, content string, expectIndex int64) *issues_model.Issue {
	var newIssue issues_model.Issue
	t.Run(title, func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

		issue := issues_model.Issue{
			RepoID:   repo.ID,
			PosterID: user.ID,
			Poster:   user,
			Title:    title,
			Content:  content,
		}
		err := issues_model.NewIssue(db.DefaultContext, repo, &issue, nil, nil)
		assert.NoError(t, err)

		has, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Get(&newIssue)
		assert.NoError(t, err)
		assert.True(t, has)
		assert.EqualValues(t, issue.Title, newIssue.Title)
		assert.EqualValues(t, issue.Content, newIssue.Content)
		if expectIndex > 0 {
			assert.EqualValues(t, expectIndex, newIssue.Index)
		}
	})
	return &newIssue
}

func TestIssue_InsertIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// there are 5 issues and max index is 5 on repository 1, so this one should 6
	issue := testInsertIssue(t, "my issue1", "special issue's comments?", 6)
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Delete(new(issues_model.Issue))
	assert.NoError(t, err)

	issue = testInsertIssue(t, `my issue2, this is my son's love \n \r \ `, "special issue's '' comments?", 7)
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Delete(new(issues_model.Issue))
	assert.NoError(t, err)
}

func TestIssue_ResolveMentions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(owner, repo, doer string, mentions []string, expected []int64) {
		o := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: owner})
		r := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: o.ID, LowerName: repo})
		issue := &issues_model.Issue{RepoID: r.ID}
		d := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: doer})
		resolved, err := issues_model.ResolveIssueMentionsByVisibility(db.DefaultContext, issue, d, mentions)
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
	testSuccess("org17", "big_test_private_4", "user20", []string{"user2"}, []int64{2})
	// Private repo, not a team member
	testSuccess("org17", "big_test_private_4", "user20", []string{"user5"}, []int64{})
	// Private repo, whole team
	testSuccess("org17", "big_test_private_4", "user15", []string{"org17/owners"}, []int64{18})
}

func TestResourceIndex(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			testInsertIssue(t, fmt.Sprintf("issue %d", i+1), "my issue", 0)
			wg.Done()
		}(i)
	}
	wg.Wait()
}

func TestCorrectIssueStats(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// Because the condition is to have chunked database look-ups,
	// We have to more issues than `maxQueryParameters`, we will insert.
	// maxQueryParameters + 10 issues into the testDatabase.
	// Each new issues will have a constant description "Bugs are nasty"
	// Which will be used later on.

	issueAmount := issues_model.MaxQueryParameters + 10

	var wg sync.WaitGroup
	for i := 0; i < issueAmount; i++ {
		wg.Add(1)
		go func(i int) {
			testInsertIssue(t, fmt.Sprintf("Issue %d", i+1), "Bugs are nasty", 0)
			wg.Done()
		}(i)
	}
	wg.Wait()

	// Now we will get all issueID's that match the "Bugs are nasty" query.
	issues, err := issues_model.Issues(context.TODO(), &issues_model.IssuesOptions{
		Paginator: &db.ListOptions{
			PageSize: issueAmount,
		},
		RepoIDs: []int64{1},
	})
	total := int64(len(issues))
	var ids []int64
	for _, issue := range issues {
		if issue.Content == "Bugs are nasty" {
			ids = append(ids, issue.ID)
		}
	}

	// Just to be sure.
	assert.NoError(t, err)
	assert.EqualValues(t, issueAmount, total)

	// Now we will call the GetIssueStats with these IDs and if working,
	// get the correct stats back.
	issueStats, err := issues_model.GetIssueStats(db.DefaultContext, &issues_model.IssuesOptions{
		RepoIDs:  []int64{1},
		IssueIDs: ids,
	})

	// Now check the values.
	assert.NoError(t, err)
	assert.EqualValues(t, issueStats.OpenCount, issueAmount)
}

func TestMilestoneList_LoadTotalTrackedTimes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	miles := issues_model.MilestoneList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}),
	}

	assert.NoError(t, miles.LoadTotalTrackedTimes(db.DefaultContext))

	assert.Equal(t, int64(3682), miles[0].TotalTrackedTime)
}

func TestLoadTotalTrackedTime(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})

	assert.NoError(t, milestone.LoadTotalTrackedTime(db.DefaultContext))

	assert.Equal(t, int64(3682), milestone.TotalTrackedTime)
}

func TestCountIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	count, err := issues_model.CountIssues(db.DefaultContext, &issues_model.IssuesOptions{})
	assert.NoError(t, err)
	assert.EqualValues(t, 20, count)
}

func TestIssueLoadAttributes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	setting.Service.EnableTimetracking = true

	issueList := issues_model.IssueList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}),
		unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4}),
	}

	for _, issue := range issueList {
		assert.NoError(t, issue.LoadAttributes(db.DefaultContext))
		assert.EqualValues(t, issue.RepoID, issue.Repo.ID)
		for _, label := range issue.Labels {
			assert.EqualValues(t, issue.RepoID, label.RepoID)
			unittest.AssertExistsAndLoadBean(t, &issues_model.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
		}
		if issue.PosterID > 0 {
			assert.EqualValues(t, issue.PosterID, issue.Poster.ID)
		}
		if issue.AssigneeID > 0 {
			assert.EqualValues(t, issue.AssigneeID, issue.Assignee.ID)
		}
		if issue.MilestoneID > 0 {
			assert.EqualValues(t, issue.MilestoneID, issue.Milestone.ID)
		}
		if issue.IsPull {
			assert.EqualValues(t, issue.ID, issue.PullRequest.IssueID)
		}
		for _, attachment := range issue.Attachments {
			assert.EqualValues(t, issue.ID, attachment.IssueID)
		}
		for _, comment := range issue.Comments {
			assert.EqualValues(t, issue.ID, comment.IssueID)
		}
		if issue.ID == int64(1) {
			assert.Equal(t, int64(400), issue.TotalTrackedTime)
			assert.NotNil(t, issue.Project)
			assert.Equal(t, int64(1), issue.Project.ID)
		} else {
			assert.Nil(t, issue.Project)
		}
	}
}

func assertCreateIssues(t *testing.T, isPull bool) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	reponame := "repo1"
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{Name: reponame})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1})
	assert.EqualValues(t, milestone.ID, 1)
	reaction := &issues_model.Reaction{
		Type:   "heart",
		UserID: owner.ID,
	}

	title := "issuetitle1"
	is := &issues_model.Issue{
		RepoID:      repo.ID,
		MilestoneID: milestone.ID,
		Repo:        repo,
		Title:       title,
		Content:     "issuecontent1",
		IsPull:      isPull,
		PosterID:    owner.ID,
		Poster:      owner,
		IsClosed:    true,
		Labels:      []*issues_model.Label{label},
		Reactions:   []*issues_model.Reaction{reaction},
	}
	err := issues_model.InsertIssues(db.DefaultContext, is)
	assert.NoError(t, err)

	i := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{Title: title})
	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: owner.ID, IssueID: i.ID})
}

func TestMigrate_CreateIssuesIsPullFalse(t *testing.T) {
	assertCreateIssues(t, false)
}

func TestMigrate_CreateIssuesIsPullTrue(t *testing.T) {
	assertCreateIssues(t, true)
}
