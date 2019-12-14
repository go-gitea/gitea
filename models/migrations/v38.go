// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"time"

	"code.gitea.io/gitea/models"

	"xorm.io/core"
	"xorm.io/xorm"
)

func removeCommitsUnitType(x *xorm.Engine) (err error) {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64 `xorm:"INDEX(s)"`
		Type        int   `xorm:"INDEX(s)"`
		Index       int
		Config      core.Conversion `xorm:"TEXT"`
		CreatedUnix int64           `xorm:"INDEX CREATED"`
		Created     time.Time       `xorm:"-"`
	}

	type Team struct {
		ID        int64
		UnitTypes []int `xorm:"json"`
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
			ut := make([]int, 0, len(team.UnitTypes))
			for _, u := range team.UnitTypes {
				if u < V16UnitTypeCommits {
					ut = append(ut, u)
				} else if u > V16UnitTypeSettings {
					ut = append(ut, u-2)
				} else if u > V16UnitTypeCommits && u != V16UnitTypeSettings {
					ut = append(ut, u-1)
				}
			}
			team.UnitTypes = ut
			if _, err := x.ID(team.ID).Cols("unit_types").Update(team); err != nil {
				return err
			}
		}
	}

	// Delete commits and settings unit types
	if _, err = x.In("`type`", []models.UnitType{V16UnitTypeCommits, V16UnitTypeSettings}).Delete(new(RepoUnit)); err != nil {
		return err
	}
	// Fix renumber unit types that where in enumeration after settings unit type
	if _, err = x.Where("`type` > ?", V16UnitTypeSettings).Decr("type").Decr("index").Update(new(RepoUnit)); err != nil {
		return err
	}
	// Fix renumber unit types that where in enumeration after commits unit type
	if _, err = x.Where("`type` > ?", V16UnitTypeCommits).Decr("type").Decr("index").Update(new(RepoUnit)); err != nil {
		return err
	}

	return nil
}
