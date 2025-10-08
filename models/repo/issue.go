// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// CanEnableTimetracker returns true when the server admin enabled time tracking
// This overrules IsTimetrackerEnabled
func (repo *Repository) CanEnableTimetracker(ctx context.Context) bool {
	return setting.Config().Service.EnableTimeTracking.Value(ctx)
}

// IsTimetrackerEnabled returns whether or not the timetracker is enabled. It returns the default value from config if an error occurs.
func (repo *Repository) IsTimetrackerEnabled(ctx context.Context) bool {
	if !setting.Config().Service.EnableTimeTracking.Value(ctx) {
		return false
	}

	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		return setting.Config().Service.DefaultEnableTimeTracking.Value(ctx)
	}
	return u.IssuesConfig().EnableTimetracker
}

// AllowOnlyContributorsToTrackTime returns value of IssuesConfig or the default value
func (repo *Repository) AllowOnlyContributorsToTrackTime(ctx context.Context) bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		return setting.Config().Service.DefaultAllowOnlyContributorsToTrackTime.Value(ctx)
	}
	return u.IssuesConfig().AllowOnlyContributorsToTrackTime
}

// IsDependenciesEnabled returns if dependencies are enabled and returns the default setting if not set.
func (repo *Repository) IsDependenciesEnabled(ctx context.Context) bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		log.Trace("IsDependenciesEnabled: %v", err)
		return setting.Config().Service.DefaultEnableDependencies.Value(ctx)
	}
	return u.IssuesConfig().EnableDependencies
}
