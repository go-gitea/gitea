// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"gitea.dev/models/unit"
	"gitea.dev/modules/setting"
)

// CanEnableTimetracker returns true when the server admin enabled time tracking
// This overrules IsTimetrackerEnabled
func (repo *Repository) CanEnableTimetracker() bool {
	return setting.Service.EnableTimetracking
}

func (repo *Repository) IsTimetrackerEnabled(ctx context.Context) bool {
	if !setting.Service.EnableTimetracking {
		return false
	}
	return repo.MustGetUnit(ctx, unit.TypeIssues).IssuesConfig().EnableTimetracker
}

func (repo *Repository) AllowOnlyContributorsToTrackTime(ctx context.Context) bool {
	return repo.MustGetUnit(ctx, unit.TypeIssues).IssuesConfig().AllowOnlyContributorsToTrackTime
}

func (repo *Repository) IsDependenciesEnabled(ctx context.Context) bool {
	return repo.MustGetUnit(ctx, unit.TypeIssues).IssuesConfig().EnableDependencies
}
