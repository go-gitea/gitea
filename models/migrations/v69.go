// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func moveTeamUnitsToTeamUnitTable(x *xorm.Engine) error {
	// Team see models/team.go
	type Team struct {
		ID        int64
		OrgID     int64
		UnitTypes []int `xorm:"json"`
	}

	// TeamUnit see models/org_team.go
	type TeamUnit struct {
		ID     int64 `xorm:"pk autoincr"`
		OrgID  int64 `xorm:"INDEX"`
		TeamID int64 `xorm:"UNIQUE(s)"`
		Type   int   `xorm:"UNIQUE(s)"`
	}

	if err := x.Sync2(new(TeamUnit)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	// Update team unit types
	const batchSize = 100
	for start := 0; ; start += batchSize {
		teams := make([]*Team, 0, batchSize)
		if err := x.Limit(batchSize, start).Find(&teams); err != nil {
			return err
		}
		if len(teams) == 0 {
			break
		}

		for _, team := range teams {
			var unitTypes []int
			if len(team.UnitTypes) == 0 {
				unitTypes = allUnitTypes
			} else {
				unitTypes = team.UnitTypes
			}

			// insert units for team
			var units = make([]TeamUnit, 0, len(unitTypes))
			for _, tp := range unitTypes {
				units = append(units, TeamUnit{
					OrgID:  team.OrgID,
					TeamID: team.ID,
					Type:   tp,
				})
			}

			if _, err := sess.Insert(&units); err != nil {
				return fmt.Errorf("Insert team units: %v", err)
			}

		}
	}

	// Commit and begin new transaction for dropping columns
	if err := sess.Commit(); err != nil {
		return err
	}
	if err := sess.Begin(); err != nil {
		return err
	}

	if err := dropTableColumns(sess, "team", "unit_types"); err != nil {
		return err
	}
	return sess.Commit()
}
