// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "code.gitea.io/gitea/modules/setting"

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
func (repo *Repository) IsTimetrackerEnabled() bool {
	if !setting.Service.EnableTimetracking {
		return false
	}

	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(UnitTypeIssues); err != nil {
		return setting.Service.DefaultEnableTimetracking
	}
	return u.IssuesConfig().EnableTimetracker
}

// AllowOnlyContributorsToTrackTime returns value of IssuesConfig or the default value
func (repo *Repository) AllowOnlyContributorsToTrackTime() bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(UnitTypeIssues); err != nil {
		return setting.Service.DefaultAllowOnlyContributorsToTrackTime
	}
	return u.IssuesConfig().AllowOnlyContributorsToTrackTime
}
