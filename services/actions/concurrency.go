// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"context"

	actions_model "code.gitea.io/gitea/models/actions"
)

func CancelActionRunByConcurrency(ctx context.Context, run *actions_model.ActionRun) error {
	return actions_model.CancelPreviousJobsWithOpts(ctx, &actions_model.FindRunOptions{
		ConcurrencyGroup: run.ConcurrencyGroup,
		Status: []actions_model.Status{
			actions_model.StatusRunning,
			actions_model.StatusWaiting,
			actions_model.StatusBlocked,
		},
	})
}
