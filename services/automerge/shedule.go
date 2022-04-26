// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
	"context"

	"code.gitea.io/gitea/models"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ScheduleAutoMerge if schedule is false and no error, pull can be merged directly
func ScheduleAutoMerge(ctx context.Context, doer *user_model.User, pull *models.PullRequest, style repo_model.MergeStyle, message string) (scheduled bool, err error) {
	lastCommitStatus, err := pull_service.GetPullRequestCommitStatusState(ctx, pull)
	if err != nil {
		return false, err
	}

	// we dont need to schedule
	if lastCommitStatus.IsSuccess() {
		return false, nil
	}

	return true, models.ScheduleAutoMerge(ctx, doer, pull.ID, style, message)
}
