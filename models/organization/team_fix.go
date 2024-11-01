// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package organization

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"

	"xorm.io/builder"
)

// CountInconsistentOwnerTeams returns the amount of owner teams that any of
// their access modes not set to the right value. For external trackers and external wikis
// it should be Read and otherwise it should be Owner.
func CountInconsistentOwnerTeams(ctx context.Context) (int64, error) {
	cnt1, err := db.GetEngine(ctx).Table("team").
		Join("INNER", "team_unit", "`team`.id = `team_unit`.team_id").
		Where("`team`.lower_name = ?", strings.ToLower(OwnerTeamName)).
		And(builder.Or(builder.Eq{"`team_unit`.`type`": unit.TypeExternalTracker}, builder.Eq{"`team_unit`.`type`": unit.TypeExternalWiki})).
		And("`team_unit`.`access_mode` <> ?", perm.AccessModeRead).
		GroupBy("`team`.`id`").
		Count()
	if err != nil {
		return 0, err
	}

	cnt2, err := db.GetEngine(ctx).Table("team").
		Join("INNER", "team_unit", "`team`.id = `team_unit`.team_id").
		Where("`team`.lower_name = ?", strings.ToLower(OwnerTeamName)).
		And(builder.And(builder.Neq{"`team_unit`.`type`": unit.TypeExternalTracker}, builder.Neq{"`team_unit`.`type`": unit.TypeExternalWiki})).
		And("`team_unit`.`access_mode` <> ?", perm.AccessModeOwner).
		GroupBy("`team_unit`.team_id").
		Count()
	if err != nil {
		return 0, err
	}
	return cnt1 + cnt2, nil
}

// FixInconsistentOwnerTeams fixes inconsistent owner teams that  of
// their access modes not set to the right value. For external trackers and external wikis
// it should be Read and otherwise it should be Owner.
func FixInconsistentOwnerTeams(ctx context.Context) (int64, error) {
	subQuery := builder.Select("id").From("team").Where(builder.Eq{"lower_name": strings.ToLower(OwnerTeamName)})
	updated1, err := db.GetEngine(ctx).Table("team_unit").
		Where(builder.Or(builder.Eq{"`team_unit`.`type`": unit.TypeExternalTracker}, builder.Eq{"`team_unit`.`type`": unit.TypeExternalWiki})).
		In("team_id", subQuery).
		Cols("access_mode").
		Update(&TeamUnit{
			AccessMode: perm.AccessModeRead,
		})
	if err != nil {
		return 0, err
	}

	updated2, err := db.GetEngine(ctx).Table("team_unit").
		Where(builder.And(builder.Neq{"`team_unit`.`type`": unit.TypeExternalTracker}, builder.Neq{"`team_unit`.`type`": unit.TypeExternalWiki})).
		In("team_id", subQuery).
		Cols("access_mode").
		Update(&TeamUnit{
			AccessMode: perm.AccessModeOwner,
		})
	if err != nil {
		return 0, err
	}
	return updated1 + updated2, nil
}
