// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_22 //nolint

import (
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func FixMissingAdminTeamUnitRecords(x *xorm.Engine) error {
	type UnitType int
	type AccessMode int

	type Team struct {
		ID         int64      `xorm:"pk autoincr"`
		OrgID      int64      `xorm:"INDEX"`
		AccessMode AccessMode `xorm:"'authorize'"`
	}

	type TeamUnit struct {
		ID         int64    `xorm:"pk autoincr"`
		OrgID      int64    `xorm:"INDEX"`
		TeamID     int64    `xorm:"UNIQUE(s)"`
		Type       UnitType `xorm:"UNIQUE(s)"`
		AccessMode AccessMode
	}

	const (
		// AccessModeRead read access
		AccessModeRead = 1
		// AccessModeAdmin admin access
		AccessModeAdmin = 3

		// Unit Type
		TypeInvalid         UnitType = iota // 0 invalid
		TypeCode                            // 1 code
		TypeIssues                          // 2 issues
		TypePullRequests                    // 3 PRs
		TypeReleases                        // 4 Releases
		TypeWiki                            // 5 Wiki
		TypeExternalWiki                    // 6 ExternalWiki
		TypeExternalTracker                 // 7 ExternalTracker
		TypeProjects                        // 8 Kanban board
		TypePackages                        // 9 Packages
		TypeActions                         // 10 Actions
	)

	AllRepoUnitTypes := []UnitType{
		TypeCode,
		TypeIssues,
		TypePullRequests,
		TypeReleases,
		TypeWiki,
		TypeExternalWiki,
		TypeExternalTracker,
		TypeProjects,
		TypePackages,
		TypeActions,
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	start := 0

	for {
		var teams []*Team
		err := sess.Where("team.authorize = ?", AccessModeAdmin).Limit(limit, start).Find(&teams)
		if err != nil {
			return err
		}

		if len(teams) == 0 {
			break
		}
		start += len(teams)

		for _, team := range teams {
			// find all existing records
			teamunits := make([]*TeamUnit, 0, len(AllRepoUnitTypes))
			err := sess.Where("`team_unit`.team_id = ?", team.ID).Find(&teamunits)
			if err != nil {
				return err
			}
			existingUnitTypes := make(container.Set[UnitType], 0)
			for _, tu := range teamunits {
				if tu.Type > 0 {
					existingUnitTypes.Add(tu.Type)
				}
			}

			// insert or update records
			for _, u := range AllRepoUnitTypes {
				newTeamUnit := &TeamUnit{
					OrgID:  team.OrgID,
					TeamID: team.ID,
					Type:   u,
				}
				// external unit should be read
				if u == TypeExternalWiki || u == TypeExternalTracker {
					newTeamUnit.AccessMode = AccessModeRead
				} else {
					newTeamUnit.AccessMode = AccessModeAdmin
				}

				if existingUnitTypes.Contains(u) {
					sess.Cols("access_mode").Update(newTeamUnit)
				} else {
					sess.Insert(newTeamUnit)
				}
			}
		}

		if start%1000 == 0 { // avoid a too big transaction
			if err := sess.Commit(); err != nil {
				return err
			}
			if err := sess.Begin(); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}
