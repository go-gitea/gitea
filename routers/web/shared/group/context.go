package group

import (
	"gitea.dev/models/db"
	group_model "gitea.dev/models/group"
	"gitea.dev/services/context"
)

func LoadSelectableGroups(ctx *context.Context) error {
	var oid int64
	if ctx.RepoGroup.Group != nil {
		oid = ctx.RepoGroup.Group.OwnerID
	} else if ctx.Repo.Owner != nil {
		oid = ctx.Repo.Owner.ID
	} else if ctx.ContextUser != nil {
		oid = ctx.ContextUser.ID
	} else {
		return nil
	}
	opts := group_model.FindGroupsOptions{
		ActorID: ctx.Doer.ID,
		OwnerID: oid,
	}
	cond := group_model.AccessibleGroupCondition(ctx.Doer)
	cond = cond.And(opts.ToConds())
	groups, err := group_model.FindGroupsByCond(ctx, &group_model.FindGroupsOptions{
		ListOptions: db.ListOptions{
			ListAll: true,
		},
		ParentGroupID: -1,
	}, cond)
	if err != nil {
		return err
	}
	for _, g := range groups {
		err = g.LoadAccessibleSubgroups(ctx, true, ctx.Doer, false)
		if err != nil {
			return err
		}
	}

	ctx.Data["Groups"] = groups
	return nil
}
