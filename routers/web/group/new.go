package group

import (
	"code.gitea.io/gitea/models/perm"
	"net/http"

	"code.gitea.io/gitea/models/db"
	group_model "code.gitea.io/gitea/models/group"
	unit_model "code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/web"
	shared_user "code.gitea.io/gitea/routers/web/shared/user"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
	group_service "code.gitea.io/gitea/services/group"
)

const tplGroupNew = "group/create"

func NewGroup(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsNewGroup"] = true
	if ctx.RepoGroup.Group != nil {
		ctx.Data["Group"] = &group_model.Group{ParentGroupID: ctx.RepoGroup.Group.ID}
	} else {
		ctx.Data["Group"] = &group_model.Group{}
	}
	ctx.Data["Units"] = unit_model.Units
	if _, err := shared_user.RenderUserOrgHeader(ctx); err != nil {
		ctx.ServerError("LoadHeaderCount", err)
		return
	}

	opts := group_model.FindGroupsOptions{
		ActorID:     ctx.Doer.ID,
		CanCreateIn: optional.Some(true),
		OwnerID:     ctx.Org.Organization.ID,
	}
	cond := group_model.AccessibleGroupCondition(ctx.Doer, unit_model.TypeInvalid, perm.AccessModeWrite)
	cond = cond.And(opts.ToConds())
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		ParentGroupID: -1,
	}, cond)
	for _, g := range groups {
		err = g.LoadAccessibleSubgroups(ctx, true, ctx.Doer)
		if err != nil {
			ctx.ServerError("LoadAccessibleSubgroups", err)
			return
		}
	}
	if err != nil {
		ctx.ServerError("FindGroupsByCond", err)
		return
	}
	ctx.Data["Groups"] = groups
	ctx.HTML(http.StatusOK, tplGroupNew)
}
func NewGroupPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateGroupForm)
	log.GetLogger(log.DEFAULT).Info("what? %+v", form)
	g := &group_model.Group{
		OwnerID:       ctx.Org.Organization.ID,
		Name:          form.GroupName,
		Description:   form.Description,
		OwnerName:     ctx.Org.Organization.Name,
		ParentGroupID: form.ParentGroupID,
	}
	ctx.Data["Title"] = ctx.Org.Organization.FullName
	ctx.Data["PageIsGroupNew"] = true
	ctx.Data["Units"] = unit_model.Units
	ctx.Data["Group"] = g

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplGroupNew)
		return
	}

	if err := group_service.NewGroup(ctx, g); err != nil {
		ctx.Data["Err_GroupName"] = true
		ctx.ServerError("NewGroup", err)
		return
	}
	log.Trace("Group created: %s/%s", ctx.Org.Organization.Name, g.Name)
	ctx.Redirect(g.GroupLink())
}
