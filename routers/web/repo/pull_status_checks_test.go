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
	checkData := &pullCommitStatusCheckData{Groups: groupStatusChecks([]*git_model.CommitStatus{
		{ID: 1, Context: "lint", State: commitstatus.CommitStatusFailure},
		{ID: 2, Context: "queued", State: commitstatus.CommitStatusPending},
		{ID: 3, Context: "running", State: commitstatus.CommitStatusPending},
		{ID: 4, Context: "test", State: commitstatus.CommitStatusSuccess},
		{ID: 5, Context: "audit", State: commitstatus.CommitStatusWarning},
		{ID: 6, Context: "docs", State: commitstatus.CommitStatusSkipped},
		{Context: "missing", State: commitstatus.CommitStatusPending},
	}, actions_module.CommitActionsStatusMap{3: actions_model.StatusRunning})}

	var kinds []statusCheckKind
	for _, group := range checkData.Groups {
		kinds = append(kinds, group.kind)
	}
	assert.Equal(t, statusCheckKinds, kinds)
	assert.Len(t, checkData.Groups[0].CommitStatuses, 2)
	assert.Equal(t, "audit", checkData.Groups[0].CommitStatuses[0].Context)
	assert.True(t, checkData.hasPending())
	section := checkData.Section(translation.MockLocale{})
	assert.EqualValues(t, "repo.pulls.status_checks_failure", section.Title)
	assert.Len(t, section.Ring, 4)
}
