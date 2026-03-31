// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"fmt"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToAPIIssue_StateReason_DefaultCompleted(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 4})
	require.True(t, issue.IsClosed)
	// Fixture closed issue has no close_reason persisted; API should expose "completed".
	assert.Empty(t, issue.CloseReason)

	apiIssue := ToAPIIssue(t.Context(), nil, issue)
	assert.Equal(t, api.StateClosed, apiIssue.State)
	assert.Equal(t, "completed", apiIssue.StateReason)
	assert.Nil(t, apiIssue.StateReasonParam)
}

func TestToAPIIssue_StateReason_WithParam(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})
	issue.IsClosed = true
	issue.CloseReason = issues_model.IssueCloseReasonDuplicate

	b, err := json.Marshal(map[string]int64{"issue_index": 4})
	require.NoError(t, err)
	issue.CloseReasonParam = string(b)

	apiIssue := ToAPIIssue(t.Context(), nil, issue)
	assert.Equal(t, api.StateClosed, apiIssue.State)
	assert.Equal(t, "duplicate", apiIssue.StateReason)

	param, ok := apiIssue.StateReasonParam.(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 4, param["issue_index"], 0)
}

func TestLabel_ToLabel(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	label := unittest.AssertExistsAndLoadBean(t, &issues_model.Label{ID: 1})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: label.RepoID})
	assert.Equal(t, &api.Label{
		ID:    label.ID,
		Name:  label.Name,
		Color: "abcdef",
		URL:   fmt.Sprintf("%sapi/v1/repos/user2/repo1/labels/%d", setting.AppURL, label.ID),
	}, ToLabel(label, repo, nil))
}

func TestMilestone_APIFormat(t *testing.T) {
	milestone := &issues_model.Milestone{
		ID:              3,
		RepoID:          4,
		Name:            "milestoneName",
		Content:         "milestoneContent",
		IsClosed:        false,
		NumOpenIssues:   5,
		NumClosedIssues: 6,
		CreatedUnix:     timeutil.TimeStamp(time.Date(1999, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()),
		UpdatedUnix:     timeutil.TimeStamp(time.Date(1999, time.March, 1, 0, 0, 0, 0, time.UTC).Unix()),
		DeadlineUnix:    timeutil.TimeStamp(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()),
	}
	assert.Equal(t, api.Milestone{
		ID:           milestone.ID,
		State:        api.StateOpen,
		Title:        milestone.Name,
		Description:  milestone.Content,
		OpenIssues:   milestone.NumOpenIssues,
		ClosedIssues: milestone.NumClosedIssues,
		Created:      milestone.CreatedUnix.AsTime(),
		Updated:      milestone.UpdatedUnix.AsTimePtr(),
		Deadline:     milestone.DeadlineUnix.AsTimePtr(),
	}, *ToAPIMilestone(milestone))
}

func TestToStopWatchesRespectsPermissions(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	publicSW := unittest.AssertExistsAndLoadBean(t, &issues_model.Stopwatch{ID: 1})
	privateIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 3})
	privateSW := &issues_model.Stopwatch{IssueID: privateIssue.ID, UserID: 5}
	assert.NoError(t, db.Insert(ctx, privateSW))
	assert.NotZero(t, privateSW.ID)

	regularUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	sws := []*issues_model.Stopwatch{publicSW, privateSW}

	visible, err := ToStopWatches(ctx, regularUser, sws)
	assert.NoError(t, err)
	assert.Len(t, visible, 1)
	assert.Equal(t, "repo1", visible[0].RepoName)

	visibleAdmin, err := ToStopWatches(ctx, adminUser, sws)
	assert.NoError(t, err)
	assert.Len(t, visibleAdmin, 2)
	assert.ElementsMatch(t, []string{"repo1", "repo3"}, []string{visibleAdmin[0].RepoName, visibleAdmin[1].RepoName})
}

func TestToTrackedTime(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	publicIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 1})
	privateIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: 3})
	regularUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	adminUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	publicTrackedTime := &issues_model.TrackedTime{IssueID: publicIssue.ID, UserID: regularUser.ID, Time: 3600}
	privateTrackedTime := &issues_model.TrackedTime{IssueID: privateIssue.ID, UserID: regularUser.ID, Time: 1800}
	require.NoError(t, db.Insert(ctx, publicTrackedTime))
	require.NoError(t, db.Insert(ctx, privateTrackedTime))

	t.Run("NilIssues", func(t *testing.T) {
		list := ToTrackedTimeList(ctx, regularUser, issues_model.TrackedTimeList{publicTrackedTime, privateTrackedTime})
		assert.Empty(t, list)
	})

	t.Run("NilRepo", func(t *testing.T) {
		badTrackedTime := &issues_model.TrackedTime{Issue: &issues_model.Issue{RepoID: 999999}}
		visible := ToTrackedTimeList(ctx, regularUser, issues_model.TrackedTimeList{badTrackedTime})
		assert.Empty(t, visible)
	})

	trackedTimes := issues_model.TrackedTimeList{publicTrackedTime, privateTrackedTime}
	require.NoError(t, trackedTimes.LoadAttributes(ctx))

	t.Run("ToRegularUser", func(t *testing.T) {
		list := ToTrackedTimeList(ctx, regularUser, trackedTimes)
		require.Len(t, list, 1)
		assert.Equal(t, "repo1", list[0].Issue.Repo.Name)
	})
	t.Run("ToAdminUser", func(t *testing.T) {
		list := ToTrackedTimeList(ctx, adminUser, trackedTimes)
		require.Len(t, list, 2)
		assert.ElementsMatch(t, []string{"repo1", "repo3"}, []string{list[0].Issue.Repo.Name, list[1].Issue.Repo.Name})
	})
}
