// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	actions_model "gitea.dev/models/actions"
	git_model "gitea.dev/models/git"
	actions_module "gitea.dev/modules/actions"
	"gitea.dev/modules/commitstatus"

	"github.com/stretchr/testify/assert"
)

func TestBuildStatusCheckGroupsSeparatesUnfinishedStates(t *testing.T) {
	statuses := []*git_model.CommitStatus{
		{ID: 1, State: commitstatus.CommitStatusFailure},
		{ID: 2, State: commitstatus.CommitStatusPending},
		{ID: 3, State: commitstatus.CommitStatusPending},
		{ID: 4, State: commitstatus.CommitStatusPending},
		{ID: 5, State: commitstatus.CommitStatusSkipped},
		{ID: 6, State: commitstatus.CommitStatusSuccess},
	}
	groups, summaryCounts := buildStatusCheckGroups(statuses, actions_module.CommitActionsStatusMap{
		3: actions_model.StatusRunning,
		4: actions_model.StatusWaiting,
	}, []string{"required-check"})

	assert.Equal(t, []StatusCheckGroupKind{
		StatusCheckGroupFailed,
		StatusCheckGroupPending,
		StatusCheckGroupInProgress,
		StatusCheckGroupSkipped,
		StatusCheckGroupSuccess,
	}, []StatusCheckGroupKind{
		groups[0].Kind,
		groups[1].Kind,
		groups[2].Kind,
		groups[3].Kind,
		groups[4].Kind,
	})
	assert.Equal(t, 3, groups[1].Count())
	assert.Equal(t, 1, summaryCounts[StatusCheckGroupInProgress])
	assert.Equal(t, 1, summaryCounts[StatusCheckGroupQueued])
}
