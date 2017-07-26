// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// ___________.__             ___________                     __
// \__    ___/|__| _____   ___\__    ___/___________    ____ |  | __ ___________
// |    |   |  |/     \_/ __ \|    |  \_  __ \__  \ _/ ___\|  |/ // __ \_  __ \
// |    |   |  |  Y Y  \  ___/|    |   |  | \// __ \\  \___|    <\  ___/|  | \/
// |____|   |__|__|_|  /\___  >____|   |__|  (____  /\___  >__|_ \\___  >__|
// \/     \/                    \/     \/     \/    \/

// IsTimetrackerEnabled returns whether or not the timetracker is enabled. It also returns false if an error occurs.
func (repo *Repository) IsTimetrackerEnabled() bool {
	var u *RepoUnit
	var err error
	if u, err = repo.GetUnit(UnitTypeIssues); err != nil {
		return false
	}
	return u.IssuesConfig().EnableTimetracker
}
