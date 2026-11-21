// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v29

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"xorm.io/xorm"
)

func AddUniqueTeamNameIndex(_ctx context.Context, x base.EngineMigration) error {
	type Team struct {
		ID        int64  `xorm:"pk autoincr"`
		OrgID     int64  `xorm:"INDEX UNIQUE(s)"`
		LowerName string `xorm:"UNIQUE(s)"`
		Name      string
	}

	var teams []Team
	if err := x.Where("id NOT IN (SELECT MIN(id) FROM team GROUP BY org_id, lower_name)").Asc("id").Find(&teams); err != nil {
		return err
	}
	for _, team := range teams {
		for suffix := team.ID; ; suffix++ {
			ending := fmt.Sprintf("-%d", suffix)
			name := util.TruncateStringBytes(team.Name, 255-len(ending)) + ending
			exists, err := x.Where("org_id = ? AND lower_name = ?", team.OrgID, strings.ToLower(name)).Exist(new(Team))
			if err != nil {
				return err
			}
			if !exists {
				if _, err := x.ID(team.ID).Cols("name", "lower_name").Update(&Team{Name: name, LowerName: strings.ToLower(name)}); err != nil {
					return err
				}
				log.Warn("renamed duplicate team id=%d in org id=%d from %q to %q", team.ID, team.OrgID, team.Name, name)
				break
			}
		}
	}

	_, err := x.SyncWithOptions(xorm.SyncOptions{IgnoreDropIndices: true}, new(Team))
	return err
}
