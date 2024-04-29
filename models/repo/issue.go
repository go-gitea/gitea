// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"

	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ___________.__             ___________                     __
// \__    ___/|__| _____   ___\__    ___/___________    ____ |  | __ ___________
// |    |   |  |/     \_/ __ \|    |  \_  __ \__  \ _/ ___\|  |/ // __ \_  __ \
// |    |   |  |  Y Y  \  ___/|    |   |  | \// __ \\  \___|    <\  ___/|  | \/
// |____|   |__|__|_|  /\___  >____|   |__|  (____  /\___  >__|_ \\___  >__|
// \/     \/                    \/     \/     \/    \/

// CanEnableTimetracker returns true when the server admin enabled time tracking
// This overrules IsTimetrackerEnabled
func (repo *Repository) CanEnableTimetracker() bool {
	return setting.Service.EnableTimetracking
}

// IsTimetrackerEnabled returns whether or not the timetracker is enabled. It returns the default value from config if an error occurs.
func (repo *Repository) IsTimetrackerEnabled(ctx context.Context) bool {
	if !setting.Service.EnableTimetracking {
		return false
	}

	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		return setting.Service.DefaultEnableTimetracking
	}
	return u.IssuesConfig().EnableTimetracker
}

// AllowOnlyContributorsToTrackTime returns value of IssuesConfig or the default value
func (repo *Repository) AllowOnlyContributorsToTrackTime(ctx context.Context) bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		return setting.Service.DefaultAllowOnlyContributorsToTrackTime
	}
	return u.IssuesConfig().AllowOnlyContributorsToTrackTime
}

// IsDependenciesEnabled returns if dependencies are enabled and returns the default setting if not set.
func (repo *Repository) IsDependenciesEnabled(ctx context.Context) bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(ctx, unit.TypeIssues); err != nil {
		log.Trace("IsDependenciesEnabled: %v", err)
		return setting.Service.DefaultEnableDependencies
	}
	return u.IssuesConfig().EnableDependencies
}
