// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package group

import (
	"context"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

func AddTeamToGroup(ctx context.Context, group *group_model.Group, tname string, unitMap map[string]string, canCreateIn *bool, accessMode *perm.AccessMode) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	t, err := org_model.GetTeam(ctx, group.OwnerID, tname)
	if err != nil {
		return err
	}
	has := group_model.HasTeamGroup(ctx, group.OwnerID, t.ID, group.ID)
	if has {
		return fmt.Errorf("team '%s' already exists in group[%d]", tname, group.ID)
	}
	parentGroup, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, t.ID)
	if err != nil {
		return err
	}
	mode := t.AccessMode
	canCreateInRepo := t.CanCreateOrgRepo
	if parentGroup == nil {
		parentGroup, err = group_model.GetNearestAncestorWithTeam(ctx, group.ID, t.ID)
		if err != nil {
			return err
		}
	}
	if parentGroup != nil {
		mode = max(t.AccessMode, parentGroup.AccessMode)
		canCreateInRepo = parentGroup.CanCreateIn || t.CanCreateOrgRepo
	}
	if accessMode != nil {
		mode = max(mode, *accessMode)
	}
	if canCreateIn != nil {
		canCreateInRepo = *canCreateIn
	}
	isNew := true
	if err = group.LoadParentGroup(ctx); err != nil {
		return err
	}
	err = group_model.AddTeamGroup(ctx, group.OwnerID, t.ID, group.ID, mode, canCreateInRepo)
	if err != nil {
		asString := strings.ToLower(err.Error())
		if strings.Contains(asString, "unique constraint failed") ||
			strings.Contains(asString, "uqe") ||
			strings.Contains(asString, "duplicate") {
			gt, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, t.ID)
			if err != nil {
				return err
			}
			gt.CanCreateIn = canCreateInRepo
			gt.AccessMode = mode
			isNew = false
			if err = UpdateGroupTeam(ctx, gt, unitMap); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if isNew {
		for unitName, unitPerm := range unitMap {
			err = UpdateOrCreateGroupUnit(ctx, group, t, unit.Units[unit.TypeFromKey(unitName)], perm.ParseAccessMode(unitPerm))
			if err != nil {
				return err
			}
		}
	}
	return committer.Commit()
}

func DeleteTeamFromGroup(ctx context.Context, group *group_model.Group, org int64, teamName string) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	team, err := org_model.GetTeam(ctx, org, teamName)
	if err != nil {
		return err
	}
	err = group_model.RemoveTeamGroup(ctx, org, team.ID, group.ID)
	if err != nil {
		return err
	}
	if _, err = db.GetEngine(ctx).Where("group_id = ?", group.ID).And("team_id = ?", team.ID).Delete(new(group_model.RepoGroupUnit)); err != nil {
		return err
	}
	return committer.Commit()
}

func UpdateGroupTeam(ctx context.Context, gt *group_model.RepoGroupTeam, unitMap map[string]string) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	if _, err = sess.ID(gt.ID).AllCols().Update(gt); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	for k, v := range unitMap {
		actualPerm := perm.ParseAccessMode(v)
		unitValue := unit.Units[unit.TypeFromKey(k)]
		groupUnit := &group_model.RepoGroupUnit{
			AccessMode: actualPerm,
			TeamID:     gt.TeamID,
			GroupID:    gt.GroupID,
			Type:       unitValue.Type,
		}
		if _, ex := gt.UnitAccessModeEx(ctx, unitValue.Type); ex {
			if _, err = sess.
				Where("team_id=?", gt.TeamID).
				And("group_id=?", gt.GroupID).
				And("type = ?", groupUnit.Type).AllCols().
				Update(groupUnit); err != nil {
				return err
			}
		} else {
			if _, err = sess.Insert(groupUnit); err != nil {
				return err
			}
		}
	}

	return committer.Commit()
}

type genericUnitAccess struct {
	Type       unit.Type
	AccessMode perm.AccessMode
}

// RecalculateGroupAccess recalculates team access to a group.
// should only be called if and only if a group was moved from another group.
func RecalculateGroupAccess(ctx context.Context, g *group_model.Group, isNew bool) error {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	if err = g.LoadParentGroup(ctx); err != nil {
		return err
	}
	var teams []*org_model.Team
	if g.ParentGroup == nil {
		if isNew {
			teams, err = org_model.FindOrgTeams(ctx, g.OwnerID)
		} else {
			teams, err = org_model.GetTeamsWithAccessToGroup(ctx, g.OwnerID, g.ID, perm.AccessModeRead)
		}
	} else {
		if isNew {
			teams, err = org_model.GetTeamsWithAccessToGroup(ctx, g.OwnerID, g.ParentGroupID, perm.AccessModeRead)
		} else {
			teams, err = org_model.GetTeamsWithAccessToGroup(ctx, g.OwnerID, g.ID, perm.AccessModeRead)
		}
	}
	if err != nil {
		return err
	}
	for _, t := range teams {
		if t.IsOwnerTeam() {
			continue
		}
		var gt *group_model.RepoGroupTeam
		if isNew {
			if gt, err = group_model.FindGroupTeamByTeamID(ctx, g.ParentGroupID, t.ID); err != nil {
				return err
			}
			if gt == nil {
				if gt, err = group_model.GetNearestAncestorWithTeam(ctx, g.ParentGroupID, t.ID); err != nil {
					return err
				}
			}
		} else {
			if gt, err = group_model.FindGroupTeamByTeamID(ctx, g.ID, t.ID); err != nil {
				return err
			}
			if gt == nil {
				if gt, err = group_model.GetNearestAncestorWithTeam(ctx, g.ID, t.ID); err != nil {
					return err
				}
			}
		}
		var (
			newGroupTeamAccessMode perm.AccessMode
			canCreateIn            bool
			units                  []genericUnitAccess
		)
		if gt != nil {
			newGroupTeamAccessMode = gt.AccessMode
			canCreateIn = gt.CanCreateIn
			if err = gt.LoadGroupUnits(ctx); err != nil {
				return err
			}
			units = util.SliceMap(gt.Units, func(it *group_model.RepoGroupUnit) genericUnitAccess {
				return genericUnitAccess{
					Type:       it.Type,
					AccessMode: it.AccessMode,
				}
			})
		} else {
			newGroupTeamAccessMode = t.AccessMode
			canCreateIn = t.CanCreateOrgRepo
			if err = t.LoadUnits(ctx); err != nil {
				return err
			}
			units = util.SliceMap(t.Units, func(it *org_model.TeamUnit) genericUnitAccess {
				return genericUnitAccess{
					Type:       it.Type,
					AccessMode: it.AccessMode,
				}
			})
		}
		if err = group_model.UpdateTeamGroup(ctx, g.OwnerID, t.ID, g.ID, newGroupTeamAccessMode, canCreateIn, isNew); err != nil {
			return err
		}

		for _, u := range units {
			newAccessMode := u.AccessMode
			var gu *group_model.RepoGroupUnit
			if g.ParentGroup == nil {
				gu, err = group_model.GetGroupUnit(ctx, g.ID, t.ID, u.Type)
				if err != nil {
					return err
				}
			}
			if gu == nil {
				gid := g.ID
				if gt != nil {
					gid = gt.GroupID
				}
				if gu, err = group_model.GetGroupUnit(ctx, gid, t.ID, u.Type); err != nil {
					return err
				}
			}
			if gu != nil {
				newAccessMode = gu.AccessMode
			}
			err = UpdateOrCreateGroupUnit(ctx, g, t, unit.Units[u.Type], newAccessMode)
			if err != nil {
				return err
			}
		}
	}
	return committer.Commit()
}

func UpdateOrCreateGroupUnit(ctx context.Context, group *group_model.Group, team *org_model.Team, unit unit.Unit, mode perm.AccessMode) error {
	sess := db.GetEngine(ctx)
	var isNew bool
	gt, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, team.ID)
	if err != nil {
		return err
	}
	if err = gt.LoadGroupUnits(ctx); err != nil {
		return err
	}
	if gt == nil {
		isNew = true
	} else {
		_, ex := gt.UnitAccessModeEx(ctx, unit.Type)
		isNew = !ex
	}

	if isNew {
		if _, err = sess.Table("repo_group_unit").MustCols("access_mode").Insert(&group_model.RepoGroupUnit{
			Type:       unit.Type,
			TeamID:     team.ID,
			GroupID:    group.ID,
			AccessMode: mode,
		}); err != nil {
			return err
		}
	} else {
		if _, err = sess.Table("repo_group_unit").Where(builder.Eq{
			"type":     unit.Type,
			"team_id":  team.ID,
			"group_id": group.ID,
		}).MustCols("access_mode").Update(&group_model.RepoGroupUnit{
			AccessMode: mode,
		}); err != nil {
			return err
		}
	}
	return nil
}
