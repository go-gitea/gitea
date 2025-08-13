package group

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	"xorm.io/builder"
)

func AddTeamToGroup(ctx context.Context, group *group_model.Group, tname string) error {
	t, err := org_model.GetTeam(ctx, group.OwnerID, tname)
	if err != nil {
		return err
	}
	has := group_model.HasTeamGroup(ctx, group.OwnerID, t.ID, group.ID)
	if has {
		return fmt.Errorf("team '%s' already exists in group[%d]", tname, group.ID)
	} else {
		parentGroup, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, t.ID)
		if err != nil {
			return err
		}
		mode := t.AccessMode
		canCreateIn := t.CanCreateOrgRepo
		if parentGroup != nil {
			mode = max(t.AccessMode, parentGroup.AccessMode)
			canCreateIn = parentGroup.CanCreateIn || t.CanCreateOrgRepo
		}
		if err = group.LoadParentGroup(ctx); err != nil {
			return err
		}
		err = group_model.AddTeamGroup(ctx, group.ID, t.ID, group.ID, mode, canCreateIn)
		if err != nil {
			return err
		}
	}
	return nil
}

func DeleteTeamFromGroup(ctx context.Context, group *group_model.Group, org int64, teamName string) error {
	team, err := org_model.GetTeam(ctx, org, teamName)
	if err != nil {
		return err
	}
	if err = group_model.RemoveTeamGroup(ctx, org, team.ID, group.ID); err != nil {
		return err
	}
	return nil
}

func UpdateGroupTeam(ctx context.Context, gt *group_model.GroupTeam) (err error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return err
	}
	defer committer.Close()
	sess := db.GetEngine(ctx)

	if _, err = sess.ID(gt.ID).AllCols().Update(gt); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	for _, unit := range gt.Units {
		unit.TeamID = gt.TeamID
		if _, err = sess.
			Where("team_id=?", gt.TeamID).
			And("group_id=?", gt.GroupID).
			And("type = ?", unit.Type).
			Update(unit); err != nil {
			return
		}
	}
	return committer.Commit()
}

// RecalculateGroupAccess recalculates team access to a group.
// should only be called if and only if a group was moved from another group.
func RecalculateGroupAccess(ctx context.Context, g *group_model.Group, isNew bool) (err error) {
	sess := db.GetEngine(ctx)
	if err = g.LoadParentGroup(ctx); err != nil {
		return
	}
	var teams []*org_model.Team
	if g.ParentGroup == nil {
		teams, err = org_model.FindOrgTeams(ctx, g.OwnerID)
		if err != nil {
			return
		}
	} else {
		teams, err = org_model.GetTeamsWithAccessToGroup(ctx, g.OwnerID, g.ParentGroupID, perm.AccessModeRead)
	}
	for _, t := range teams {

		var gt *group_model.GroupTeam = nil
		if gt, err = group_model.FindGroupTeamByTeamID(ctx, g.ParentGroupID, t.ID); err != nil {
			return
		}
		if gt != nil {
			if err = group_model.UpdateTeamGroup(ctx, g.OwnerID, t.ID, g.ID, gt.AccessMode, gt.CanCreateIn, isNew); err != nil {
				return
			}
		} else {
			if err = group_model.UpdateTeamGroup(ctx, g.OwnerID, t.ID, g.ID, t.AccessMode, t.IsOwnerTeam() || t.AccessMode >= perm.AccessModeAdmin || t.CanCreateOrgRepo, isNew); err != nil {
				return
			}
		}

		if err = t.LoadUnits(ctx); err != nil {
			return
		}
		for _, u := range t.Units {

			newAccessMode := u.AccessMode
			if g.ParentGroup == nil {
				gu, err := group_model.GetGroupUnit(ctx, g.ID, t.ID, u.Type)
				if err != nil {
					return err
				}
				newAccessMode = min(newAccessMode, gu.AccessMode)
			}
			if isNew {
				if _, err = sess.Table("group_unit").Insert(&group_model.GroupUnit{
					Type:       u.Type,
					TeamID:     t.ID,
					GroupID:    g.ID,
					AccessMode: newAccessMode,
				}); err != nil {
					return
				}
			} else {
				if _, err = sess.Table("group_unit").Where(builder.Eq{
					"type":     u.Type,
					"team_id":  t.ID,
					"group_id": g.ID,
				}).Cols("access_mode").Update(&group_model.GroupUnit{
					AccessMode: newAccessMode,
				}); err != nil {
					return err
				}
			}
		}
	}
	return
}
