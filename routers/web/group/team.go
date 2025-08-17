package group

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	org_model "code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/perm"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	web_org "code.gitea.io/gitea/routers/web/org"
	shared_group "code.gitea.io/gitea/routers/web/shared/group"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	"code.gitea.io/gitea/services/forms"
	group_service "code.gitea.io/gitea/services/group"
)

const (
	tplTeamEdit = "group/team/new"
	tplTeams    = "group/team/teams"
)

func MinUnitAccessMode(unitsMap map[unit_model.Type]perm.AccessMode) perm.AccessMode {
	res := perm.AccessModeNone
	for t, mode := range unitsMap {
		// Don't allow `TypeExternal{Tracker,Wiki}` to influence this as they can only be set to READ perms.
		if t == unit_model.TypeExternalTracker || t == unit_model.TypeExternalWiki {
			continue
		}

		// get the minial permission great than AccessModeNone except all are AccessModeNone
		if mode > perm.AccessModeNone && (res == perm.AccessModeNone || mode < res) {
			res = mode
		}
	}
	return res
}

func SearchTeamCandidates(ctx *context.Context) {
	teams, _, err := org_model.SearchTeam(ctx, &org_model.SearchTeamOptions{
		OrgID:   ctx.Org.Organization.ID,
		Keyword: ctx.FormTrim("q"),
		ListOptions: db.ListOptions{
			PageSize: setting.UI.MembersPagingNum,
		},
	})
	if err != nil {
		ctx.ServerError("Unable to search teams", err)
		return
	}
	apiTeams, err := convert.ToTeams(ctx, teams, true)
	if err != nil {
		ctx.ServerError("Unable to search teams", err)
		return
	}
	ctx.JSON(http.StatusOK, map[string]any{"data": apiTeams})
}

func Teams(ctx *context.Context) {
	group := ctx.RepoGroup.Group
	ctx.Data["Title"] = group.Name
	ctx.Data["PageIsGroupTeams"] = true
	for _, t := range ctx.RepoGroup.Teams {
		if err := t.LoadMembers(ctx); err != nil {
			ctx.ServerError("GetMembers", err)
			return
		}
	}
	ctx.Data["Teams"] = ctx.RepoGroup.Teams

	err := shared_group.LoadHeaderCount(ctx)
	if err != nil {
		ctx.ServerError("RenderOrgHeader", err)
		return
	}

	ctx.HTML(http.StatusOK, tplTeams)
}

func EditTeam(ctx *context.Context) {
	ctx.Data["Title"] = ctx.RepoGroup.Group.Name
	ctx.Data["PageIsGroupTeams"] = true
	if err := ctx.RepoGroup.Team.LoadUnits(ctx); err != nil {
		ctx.ServerError("LoadUnits", err)
		return
	}
	if err := shared_group.LoadHeaderCount(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}
	ctx.Data["GroupTeam"] = ctx.RepoGroup.GroupTeam
	ctx.Data["Team"] = ctx.RepoGroup.Team
	ctx.Data["Units"] = unit_model.Units
	ctx.HTML(http.StatusOK, tplTeamEdit)
}

func EditTeamPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateGroupTeamForm)
	t := ctx.RepoGroup.Team
	gt := ctx.RepoGroup.GroupTeam
	newAccessMode := perm.ParseAccessMode(form.Permission)
	unitPerms := web_org.GetUnitPerms(ctx.Req.Form, newAccessMode)
	if newAccessMode < perm.AccessModeAdmin {
		newAccessMode = MinUnitAccessMode(unitPerms)
	}
	ctx.Data["Title"] = ctx.RepoGroup.Group.Name
	ctx.Data["PageIsGroupTeams"] = true
	ctx.Data["Team"] = t
	ctx.Data["GroupTeam"] = gt
	ctx.Data["Units"] = unit_model.Units
	if !t.IsOwnerTeam() {
		if gt.AccessMode != newAccessMode {
			gt.AccessMode = newAccessMode
		}
		if gt.CanCreateIn != form.CanCreateRepoOrSubGroup {
			gt.CanCreateIn = form.CanCreateRepoOrSubGroup
		}
	} else {
		gt.CanCreateIn = true
	}
	units := make([]*group_model.RepoGroupUnit, 0, len(unitPerms))
	for tp, perm := range unitPerms {
		units = append(units, &group_model.RepoGroupUnit{
			GroupID:    gt.GroupID,
			TeamID:     t.ID,
			Type:       tp,
			AccessMode: perm,
		})
	}
	gt.Units = units
	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTeamEdit)
		return
	}
	if gt.AccessMode < perm.AccessModeAdmin && len(unitPerms) == 0 {
		ctx.RenderWithErr(ctx.Tr("form.team_no_units_error"), tplTeamEdit, &form)
		return
	}
	if err := group_service.UpdateGroupTeam(ctx, gt); err != nil {
		ctx.ServerError("UpdateGroupTeam", err)
		return
	}
	ctx.Redirect(ctx.Org.OrgLink + "/teams/")
}

func TeamAddPost(ctx *context.Context) {
	if !ctx.RepoGroup.IsGroupAdmin || !ctx.RepoGroup.IsOwner {
		ctx.NotFound(nil)
		return
	}
	group := ctx.RepoGroup.Group
	tname := strings.ToLower(ctx.FormTrim("tname"))
	t, err := org_model.GetTeam(ctx, group.OwnerID, tname)
	if err != nil {
		if org_model.IsErrTeamNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.team_not_exist"))
			ctx.Redirect(ctx.RepoGroup.OrgGroupLink + "/teams")
		} else {
			ctx.ServerError("GetTeam", err)
		}
		return
	}
	has := group_model.HasTeamGroup(ctx, group.OwnerID, t.ID, group.ID)
	if has {
		ctx.Flash.Error(ctx.Tr("org.group.add_duplicate_team"))
	} else {
		parentGroup, err := group_model.FindGroupTeamByTeamID(ctx, group.ID, t.ID)
		if err != nil {
			ctx.ServerError("FindGroupTeamByTeamID", err)
			return
		}
		mode := t.AccessMode
		canCreateIn := t.CanCreateOrgRepo
		if parentGroup != nil {
			mode = max(t.AccessMode, parentGroup.AccessMode)
			canCreateIn = parentGroup.CanCreateIn || t.CanCreateOrgRepo
		}
		if err = group.LoadParentGroup(ctx); err != nil {
			ctx.ServerError("LoadParentGroup", err)
			return
		}
		err = group_model.AddTeamGroup(ctx, ctx.RepoGroup.Group.OwnerID, t.ID, ctx.RepoGroup.Group.ID, mode, canCreateIn)
		if err != nil {
			ctx.ServerError("AddTeamGroup", err)
			return
		}
	}
	ctx.Redirect(group.OrgGroupLink() + "/teams")
}

func TeamRemove(ctx *context.Context) {
	if !ctx.RepoGroup.IsGroupAdmin || !ctx.RepoGroup.IsOwner {
		ctx.NotFound(nil)
		return
	}
	org := ctx.Org.Organization.ID
	group := ctx.RepoGroup.Group
	team, err := org_model.GetTeam(ctx, org, ctx.PathParam("team"))
	if err != nil {
		if org_model.IsErrTeamNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("GetTeam", err)
		}
		return
	}

	if err = group_model.RemoveTeamGroup(ctx, org, team.ID, group.ID); err != nil {
		ctx.ServerError("RemoveTeamGroup", err)
		return
	}
	ctx.JSONRedirect(ctx.RepoGroup.OrgGroupLink + "/teams")
}
