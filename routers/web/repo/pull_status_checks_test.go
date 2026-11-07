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
)

func TestGroupStatusChecks(t *testing.T) {
	checkData := &pullCommitStatusCheckData{ActionsStatuses: actions_module.CommitActionsStatusMap{3: actions_model.StatusRunning}}
	checkData.groupStatuses([]*git_model.CommitStatus{
		{ID: 1, Context: "lint", State: commitstatus.CommitStatusFailure},
		{ID: 3, Context: "running", State: commitstatus.CommitStatusPending},
		{ID: 4, Context: "test", State: commitstatus.CommitStatusSuccess},
		{ID: 6, Context: "docs", State: commitstatus.CommitStatusSkipped},
		{Context: "missing", State: commitstatus.CommitStatusPending},
		{ID: 7, Context: "CI10", State: commitstatus.CommitStatusPending},
		{ID: 8, Context: "ci2", State: commitstatus.CommitStatusPending},
	})

	assert.Len(t, checkData.Groups, 5)
	assert.Equal(t, statusCheckPending, checkData.Groups[1].Kind)
	for i, context := range []string{"ci2", "CI10", "missing"} {
		assert.Equal(t, context, checkData.Groups[1].CommitStatuses[i].Context)
	}
	assert.EqualValues(t, "repo.pulls.status_checks_count_1:repo.pulls.status_checks_failing:1, repo.pulls.status_checks_pending:2, repo.pulls.status_checks_in_progress:1, repo.pulls.status_checks_skipped:1, repo.pulls.status_checks_expected:1, repo.pulls.status_checks_successful:1", checkData.Section(translation.MockLocale{}).Details[0])
}
