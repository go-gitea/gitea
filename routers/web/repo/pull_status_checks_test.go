// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/translation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupStatusChecks(t *testing.T) {
	checkData := &pullCommitStatusCheckData{}
	checkData.groupStatuses([]*git_model.CommitStatus{
		{ID: 1, Context: "lint", State: commitstatus.CommitStatusFailure},
		{ID: 3, Context: "running", State: commitstatus.CommitStatusPending},
		{ID: 4, Context: "test", State: commitstatus.CommitStatusSuccess},
		{ID: 6, Context: "docs", State: commitstatus.CommitStatusSkipped},
		{ID: 7, Context: "CI10", State: commitstatus.CommitStatusPending},
		{ID: 8, Context: "ci2", State: commitstatus.CommitStatusPending},
		{ID: 9, Context: "e2e", State: commitstatus.CommitStatusFailure},
	}, []string{"[Cc][Ii]*", "missing", "missing*", "test"}, actions_module.CommitActionsStatusMap{3: actions_model.StatusRunning, 9: actions_model.StatusCancelled}, translation.MockLocale{})

	require.Len(t, checkData.StatusCheckGroups, 5)
	assert.Equal(t, statusCheckPending, checkData.StatusCheckGroups[1].CheckKind)
	for i, context := range []string{"ci2", "CI10", "missing", "missing*"} {
		assert.Equal(t, context, checkData.StatusCheckGroups[1].StatusChecks[i].StatusContext)
		assert.True(t, checkData.StatusCheckGroups[1].StatusChecks[i].IsRequired)
	}
	assert.Equal(t, &pullStatusCheck{
		StatusContext: "missing", StatusDescription: "repo.pulls.status_checks_waiting",
		StatusState: commitstatus.CommitStatusPending, IsRequired: true,
	}, checkData.StatusCheckGroups[1].StatusChecks[2])
	assert.Equal(t, "cancelled", checkData.StatusCheckGroups[0].StatusChecks[0].ActionStatus)
	assert.False(t, checkData.StatusCheckGroups[0].StatusChecks[1].IsRequired)
	assert.Equal(t, "running", checkData.StatusCheckGroups[2].StatusChecks[0].ActionStatus)
	assert.True(t, checkData.StatusCheckGroups[4].StatusChecks[0].IsRequired)
	assert.EqualValues(t, "repo.pulls.status_checks_count_1:repo.pulls.status_checks_failing:1, repo.pulls.status_checks_cancelled:1, repo.pulls.status_checks_pending:2, repo.pulls.status_checks_in_progress:1, repo.pulls.status_checks_skipped:1, repo.pulls.status_checks_expected:2, repo.pulls.status_checks_successful:1", checkData.StatusChecksSection(translation.MockLocale{}).SectionDetails[0])

	externalData := &pullCommitStatusCheckData{}
	externalData.groupStatuses([]*git_model.CommitStatus{
		{ID: 1, Context: "external", State: commitstatus.CommitStatusRunning},
		{ID: 2, Context: "custom", State: "stage-1"},
	}, nil, nil, translation.MockLocale{})
	assert.False(t, externalData.AllPassed())
	assert.Equal(t, 1, externalData.count(statusCheckInProgress))
	assert.Equal(t, 1, externalData.count(statusCheckPending))

	cancelledData := &pullCommitStatusCheckData{}
	cancelledData.groupStatuses([]*git_model.CommitStatus{{ID: 1, Context: "lint", State: commitstatus.CommitStatusFailure}}, nil, actions_module.CommitActionsStatusMap{1: actions_model.StatusCancelled}, translation.MockLocale{})
	assert.False(t, cancelledData.AllPassed())
	cancelledSection := cancelledData.StatusChecksSection(translation.MockLocale{})
	assert.EqualValues(t, "repo.pulls.status_checking", cancelledSection.SectionTitle)
	assert.Equal(t, []statusCheckProgressSegment{{StrokeClass: "tw-stroke-red", DashLength: 100}}, cancelledSection.ProgressSegments)

	approvalData := &pullCommitStatusCheckData{RequireApprovalRunCount: 1}
	approvalData.groupStatuses([]*git_model.CommitStatus{{ID: 1, Context: "lint", State: commitstatus.CommitStatusSuccess}}, nil, nil, translation.MockLocale{})
	assert.False(t, approvalData.AllPassed())
	assert.Equal(t, "octicon-alert-fill", approvalData.StatusChecksSection(translation.MockLocale{}).StatusIcon)

	assert.Equal(t, []statusCheckProgressSegment{
		{StrokeClass: "tw-stroke-green", DashLength: 19, DashOffset: -2.5},
		{StrokeClass: "tw-stroke-yellow", DashLength: 21, DashOffset: -26.5},
		{StrokeClass: "tw-stroke-red", DashLength: 45, DashOffset: -52.5},
	}, (&pullCommitStatusCheckData{counts: map[statusCheckKind]int{statusCheckFailing: 50, statusCheckPending: 25, statusCheckInProgress: 1, statusCheckSuccessful: 24}}).progressSegments())
	assert.Equal(t, []statusCheckProgressSegment{{StrokeClass: "tw-stroke-yellow", DashLength: 100, DashOffset: 0}},
		(&pullCommitStatusCheckData{counts: map[statusCheckKind]int{statusCheckPending: 1, statusCheckInProgress: 1, statusCheckExpected: 1}}).progressSegments())
}
