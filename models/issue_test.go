// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/foreignreference"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"xorm.io/builder"
)

func TestIssue_ReplaceLabels(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(issueID int64, labelIDs []int64) {
		issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: issueID}).(*Issue)
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID}).(*repo_model.Repository)
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID}).(*user_model.User)

		labels := make([]*Label, len(labelIDs))
		for i, labelID := range labelIDs {
			labels[i] = unittest.AssertExistsAndLoadBean(t, &Label{ID: labelID, RepoID: repo.ID}).(*Label)
		}
		assert.NoError(t, ReplaceIssueLabels(issue, labels, doer))
		unittest.AssertCount(t, &IssueLabel{IssueID: issueID}, len(labelIDs))
		for _, labelID := range labelIDs {
			unittest.AssertExistsAndLoadBean(t, &IssueLabel{IssueID: issueID, LabelID: labelID})
		}
	}

	testSuccess(1, []int64{2})
	testSuccess(1, []int64{1, 2})
	testSuccess(1, []int64{})
}

func Test_GetIssueIDsByRepoID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ids, err := GetIssueIDsByRepoID(1)
	assert.NoError(t, err)
	assert.Len(t, ids, 5)
}

func TestIssueAPIURL(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 1}).(*Issue)
	err := issue.LoadAttributes()

	assert.NoError(t, err)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/issues/1", issue.APIURL())
}

func TestGetIssuesByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	testSuccess := func(expectedIssueIDs, nonExistentIssueIDs []int64) {
		issues, err := GetIssuesByIDs(append(expectedIssueIDs, nonExistentIssueIDs...))
		assert.NoError(t, err)
		actualIssueIDs := make([]int64, len(issues))
		for i, issue := range issues {
			actualIssueIDs[i] = issue.ID
		}
		assert.Equal(t, expectedIssueIDs, actualIssueIDs)
	}
	testSuccess([]int64{1, 2, 3}, []int64{})
	testSuccess([]int64{1, 2, 3}, []int64{unittest.NonexistentID})
}

func TestGetParticipantIDsByIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	checkParticipants := func(issueID int64, userIDs []int) {
		issue, err := GetIssueByID(issueID)
		assert.NoError(t, err)
		participants, err := issue.getParticipantIDsByIssue(db.GetEngine(db.DefaultContext))
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
		issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: test.issueID}).(*Issue)
		doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: test.doerID}).(*user_model.User)
		assert.NoError(t, ClearIssueLabels(issue, doer))
		unittest.AssertNotExistsBean(t, &IssueLabel{IssueID: test.issueID})
	}
}

func TestUpdateIssueCols(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{}).(*Issue)

	const newTitle = "New Title for unit test"
	issue.Title = newTitle

	prevContent := issue.Content
	issue.Content = "This should have no effect"

	now := time.Now().Unix()
	assert.NoError(t, UpdateIssueCols(db.DefaultContext, issue, "name"))
	then := time.Now().Unix()

	updatedIssue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: issue.ID}).(*Issue)
	assert.EqualValues(t, newTitle, updatedIssue.Title)
	assert.EqualValues(t, prevContent, updatedIssue.Content)
	unittest.AssertInt64InRange(t, now, then, int64(updatedIssue.UpdatedUnix))
}

func TestIssues(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
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
				RepoCond: builder.In("repo_id", 1, 3),
				SortType: "oldest",
				ListOptions: db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{1, 2, 3, 5},
		},
		{
			IssuesOptions{
				LabelIDs: []int64{1},
				ListOptions: db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
			},
			[]int64{2, 1},
		},
		{
			IssuesOptions{
				LabelIDs: []int64{1, 2},
				ListOptions: db.ListOptions{
					Page:     1,
					PageSize: 4,
				},
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
	assert.NoError(t, unittest.PrepareTestDatabase())
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
				YourRepositoriesCount: 1, // 6
				AssignCount:           1, // 6
				CreateCount:           1, // 6
				OpenCount:             1, // 6
				ClosedCount:           1, // 1
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				RepoIDs:    []int64{1},
				FilterMode: FilterModeAll,
				IsClosed:   true,
			},
			IssueStats{
				YourRepositoriesCount: 1, // 6
				AssignCount:           0,
				CreateCount:           0,
				OpenCount:             1, // 6
				ClosedCount:           1, // 1
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeAssign,
			},
			IssueStats{
				YourRepositoriesCount: 1, // 6
				AssignCount:           1, // 6
				CreateCount:           1, // 6
				OpenCount:             1, // 6
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeCreate,
			},
			IssueStats{
				YourRepositoriesCount: 1, // 6
				AssignCount:           1, // 6
				CreateCount:           1, // 6
				OpenCount:             1, // 6
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeMention,
			},
			IssueStats{
				YourRepositoriesCount: 1, // 6
				AssignCount:           1, // 6
				CreateCount:           1, // 6
				MentionCount:          0,
				OpenCount:             0,
				ClosedCount:           0,
			},
		},
		{
			UserIssueStatsOptions{
				UserID:     1,
				FilterMode: FilterModeCreate,
				IssueIDs:   []int64{1},
			},
			IssueStats{
				YourRepositoriesCount: 1, // 1
				AssignCount:           1, // 1
				CreateCount:           1, // 1
				OpenCount:             1, // 1
				ClosedCount:           0,
			},
		},
	} {
		t.Run(fmt.Sprintf("%#v", test.Opts), func(t *testing.T) {
			stats, err := GetUserIssueStats(test.Opts)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, test.ExpectedIssueStats, *stats)
		})
	}
}

func TestIssue_loadTotalTimes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ms, err := GetIssueByID(2)
	assert.NoError(t, err)
	assert.NoError(t, ms.loadTotalTimes(db.GetEngine(db.DefaultContext)))
	assert.Equal(t, int64(3682), ms.TotalTrackedTime)
}

func TestIssue_SearchIssueIDsByKeyword(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	total, ids, err := SearchIssueIDsByKeyword(context.TODO(), "issue2", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{2}, ids)

	total, ids, err = SearchIssueIDsByKeyword(context.TODO(), "first", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{1}, ids)

	total, ids, err = SearchIssueIDsByKeyword(context.TODO(), "for", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 5, total)
	assert.ElementsMatch(t, []int64{1, 2, 3, 5, 11}, ids)

	// issue1's comment id 2
	total, ids, err = SearchIssueIDsByKeyword(context.TODO(), "good", []int64{1}, 10, 0)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.EqualValues(t, []int64{1}, ids)
}

func TestGetRepoIDsForIssuesOptions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)
	for _, test := range []struct {
		Opts            IssuesOptions
		ExpectedRepoIDs []int64
	}{
		{
			IssuesOptions{
				AssigneeID: 2,
			},
			[]int64{3},
		},
		{
			IssuesOptions{
				RepoCond: builder.In("repo_id", 1, 2),
			},
			[]int64{1, 2},
		},
	} {
		repoIDs, err := GetRepoIDsForIssuesOptions(&test.Opts, user)
		assert.NoError(t, err)
		if assert.Len(t, repoIDs, len(test.ExpectedRepoIDs)) {
			for i, repoID := range repoIDs {
				assert.EqualValues(t, test.ExpectedRepoIDs[i], repoID)
			}
		}
	}
}

func testInsertIssue(t *testing.T, title, content string, expectIndex int64) *Issue {
	var newIssue Issue
	t.Run(title, func(t *testing.T) {
		repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1}).(*repo_model.Repository)
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}).(*user_model.User)

		issue := Issue{
			RepoID:   repo.ID,
			PosterID: user.ID,
			Poster:   user,
			Title:    title,
			Content:  content,
		}
		err := NewIssue(repo, &issue, nil, nil)
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
	_, err := db.GetEngine(db.DefaultContext).ID(issue.ID).Delete(new(Issue))
	assert.NoError(t, err)

	issue = testInsertIssue(t, `my issue2, this is my son's love \n \r \ `, "special issue's '' comments?", 7)
	_, err = db.GetEngine(db.DefaultContext).ID(issue.ID).Delete(new(Issue))
	assert.NoError(t, err)
}

func TestIssue_DeleteIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueIDs, err := GetIssueIDsByRepoID(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 5, len(issueIDs))

	issue := &Issue{
		RepoID: 1,
		ID:     issueIDs[2],
	}

	err = DeleteIssue(issue)
	assert.NoError(t, err)
	issueIDs, err = GetIssueIDsByRepoID(1)
	assert.NoError(t, err)
	assert.EqualValues(t, 4, len(issueIDs))

	// check attachment removal
	attachments, err := repo_model.GetAttachmentsByIssueID(4)
	assert.NoError(t, err)
	issue, err = GetIssueByID(4)
	assert.NoError(t, err)
	err = DeleteIssue(issue)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(attachments))
	for i := range attachments {
		attachment, err := repo_model.GetAttachmentByUUID(attachments[i].UUID)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrAttachmentNotExist(err))
		assert.Nil(t, attachment)
	}

	// check issue dependencies
	user, err := user_model.GetUserByID(1)
	assert.NoError(t, err)
	issue1, err := GetIssueByID(1)
	assert.NoError(t, err)
	issue2, err := GetIssueByID(2)
	assert.NoError(t, err)
	err = CreateIssueDependency(user, issue1, issue2)
	assert.NoError(t, err)
	left, err := IssueNoDependenciesLeft(issue1)
	assert.NoError(t, err)
	assert.False(t, left)
	err = DeleteIssue(&Issue{ID: 2})
	assert.NoError(t, err)
	left, err = IssueNoDependenciesLeft(issue1)
	assert.NoError(t, err)
	assert.True(t, left)
}

func TestIssue_ResolveMentions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	testSuccess := func(owner, repo, doer string, mentions []string, expected []int64) {
		o := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: owner}).(*user_model.User)
		r := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: o.ID, LowerName: repo}).(*repo_model.Repository)
		issue := &Issue{RepoID: r.ID}
		d := unittest.AssertExistsAndLoadBean(t, &user_model.User{LowerName: doer}).(*user_model.User)
		resolved, err := ResolveIssueMentionsByVisibility(db.DefaultContext, issue, d, mentions)
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
	testSuccess("user17", "big_test_private_4", "user15", []string{"user17/owners"}, []int64{18})
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

	issueAmount := maxQueryParameters + 10

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
	total, ids, err := SearchIssueIDsByKeyword(context.TODO(), "Bugs are nasty", []int64{1}, issueAmount, 0)

	// Just to be sure.
	assert.NoError(t, err)
	assert.EqualValues(t, issueAmount, total)

	// Now we will call the GetIssueStats with these IDs and if working,
	// get the correct stats back.
	issueStats, err := GetIssueStats(&IssueStatsOptions{
		RepoID:   1,
		IssueIDs: ids,
	})

	// Now check the values.
	assert.NoError(t, err)
	assert.EqualValues(t, issueStats.OpenCount, issueAmount)
}

func TestIssueForeignReference(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	issue := unittest.AssertExistsAndLoadBean(t, &Issue{ID: 4}).(*Issue)
	assert.NotEqualValues(t, issue.Index, issue.ID) // make sure they are different to avoid false positive

	// it is fine for an issue to not have a foreign reference
	err := issue.LoadAttributes()
	assert.NoError(t, err)
	assert.Nil(t, issue.ForeignReference)

	var foreignIndex int64 = 12345
	_, err = GetIssueByForeignIndex(context.Background(), issue.RepoID, foreignIndex)
	assert.True(t, foreignreference.IsErrLocalIndexNotExist(err))

	_, err = db.GetEngine(db.DefaultContext).Insert(&foreignreference.ForeignReference{
		LocalIndex:   issue.Index,
		ForeignIndex: strconv.FormatInt(foreignIndex, 10),
		RepoID:       issue.RepoID,
		Type:         foreignreference.TypeIssue,
	})
	assert.NoError(t, err)

	err = issue.LoadAttributes()
	assert.NoError(t, err)

	assert.EqualValues(t, issue.ForeignReference.ForeignIndex, strconv.FormatInt(foreignIndex, 10))

	found, err := GetIssueByForeignIndex(context.Background(), issue.RepoID, foreignIndex)
	assert.NoError(t, err)
	assert.EqualValues(t, found.Index, issue.Index)
}

func TestMilestoneList_LoadTotalTrackedTimes(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	miles := issues_model.MilestoneList{
		unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}).(*issues_model.Milestone),
	}

	assert.NoError(t, miles.LoadTotalTrackedTimes())

	assert.Equal(t, int64(3682), miles[0].TotalTrackedTime)
}

func TestLoadTotalTrackedTime(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	milestone := unittest.AssertExistsAndLoadBean(t, &issues_model.Milestone{ID: 1}).(*issues_model.Milestone)

	assert.NoError(t, milestone.LoadTotalTrackedTime())

	assert.Equal(t, int64(3682), milestone.TotalTrackedTime)
}
