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
	}, []string{"[Cc][Ii]*", "missing", "test"}, actions_module.CommitActionsStatusMap{3: actions_model.StatusRunning}, translation.MockLocale{})

	require.Len(t, checkData.Groups, 5)
	assert.Equal(t, statusCheckPending, checkData.Groups[1].Kind)
	for i, context := range []string{"ci2", "CI10", "missing"} {
		assert.Equal(t, context, checkData.Groups[1].Checks[i].Context)
		assert.True(t, checkData.Groups[1].Checks[i].IsRequired)
	}
	assert.Equal(t, &pullStatusCheck{
		Context: "missing", Description: "repo.pulls.status_checks_waiting",
		State: commitstatus.CommitStatusPending, Kind: statusCheckExpected, IsRequired: true,
	}, checkData.Groups[1].Checks[2])
	assert.False(t, checkData.Groups[0].Checks[0].IsRequired)
	assert.Equal(t, "running", checkData.Groups[2].Checks[0].ActionStatus)
	assert.True(t, checkData.Groups[4].Checks[0].IsRequired)
	assert.EqualValues(t, "repo.pulls.status_checks_count_1:repo.pulls.status_checks_failing:1, repo.pulls.status_checks_pending:2, repo.pulls.status_checks_in_progress:1, repo.pulls.status_checks_skipped:1, repo.pulls.status_checks_expected:1, repo.pulls.status_checks_successful:1", checkData.Section(translation.MockLocale{}).SectionDetails[0])

	approvalData := &pullCommitStatusCheckData{RequireApprovalRunCount: 1}
	approvalData.groupStatuses([]*git_model.CommitStatus{{ID: 1, Context: "lint", State: commitstatus.CommitStatusSuccess}}, nil, nil, translation.MockLocale{})
	assert.False(t, approvalData.AllPassed())
	assert.Equal(t, "octicon-alert-fill", approvalData.Section(translation.MockLocale{}).StatusIcon)

	missingData := &pullCommitStatusCheckData{}
	missingData.groupStatuses(nil, []string{"CI/linux", "[A-Z]I/*"}, nil, translation.MockLocale{})
	assert.Equal(t, 2, missingData.count(statusCheckExpected))

	assert.Equal(t, []statusCheckProgressSegment{
		{StrokeClass: "tw-stroke-green", DashLength: 19, DashOffset: -2.5},
		{StrokeClass: "tw-stroke-yellow", DashLength: 21, DashOffset: -26.5},
		{StrokeClass: "tw-stroke-red", DashLength: 45, DashOffset: -52.5},
	}, (&pullCommitStatusCheckData{counts: map[statusCheckKind]int{statusCheckFailing: 50, statusCheckPending: 25, statusCheckInProgress: 1, statusCheckSuccessful: 24}}).progressSegments())
	assert.Equal(t, []statusCheckProgressSegment{{StrokeClass: "tw-stroke-yellow", DashLength: 100, DashOffset: 0}},
		(&pullCommitStatusCheckData{counts: map[statusCheckKind]int{statusCheckPending: 1, statusCheckInProgress: 1, statusCheckExpected: 1}}).progressSegments())
}
