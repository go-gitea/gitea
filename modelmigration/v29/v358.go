// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm/schemas"
)

type Team struct { //revive:disable-line:exported
	ID        int64 `xorm:"pk autoincr"`
	OrgID     int64
	LowerName string
}

// TableIndices implements xorm's TableIndices interface
func (t *Team) TableIndices() []*schemas.Index {
	indices := make([]*schemas.Index, 0, 1)
	teamUnique := schemas.NewIndex("unique_team_org_lower_name", schemas.UniqueType)
	teamUnique.AddColumn("org_id", "lower_name")
	indices = append(indices, teamUnique)
	return indices
}

// AddUniqueIndexForTeam adds a unique index on (org_id, lower_name) for the team table
func AddUniqueIndexForTeam(_ context.Context, x base.EngineMigration) error {
	// remove possible duplicated records in table team
	type result struct {
		OrgID     int64
		LowerName string
		Cnt       int
	}
	var results []result
	if err := x.Select("org_id, lower_name, count(*) as cnt").
		Table("team").
		GroupBy("org_id, lower_name").
		Having("count(*) > 1").
		Find(&results); err != nil {
		return err
	}
	for _, r := range results {
		if x.Dialect().URI().DBType == schemas.MSSQL {
			if _, err := x.Exec(fmt.Sprintf("delete from team where id in (SELECT top %d id FROM team WHERE org_id = ? and lower_name = ?)", r.Cnt-1), r.OrgID, r.LowerName); err != nil {
				return err
			}
		} else {
			var ids []int64
			if err := x.SQL("SELECT id FROM team WHERE org_id = ? and lower_name = ? limit ?", r.OrgID, r.LowerName, r.Cnt-1).Find(&ids); err != nil {
				return err
			}
			if _, err := x.Table("team").In("id", ids).Delete(); err != nil {
				return err
			}
		}
	}

	return x.Sync(new(Team))
}
