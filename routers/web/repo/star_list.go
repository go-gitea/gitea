package repo

import (
	"slices"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

func StarListPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.StarListRepoEditForm)

	starListSlice, err := repo_model.GetStarListsByUserID(ctx, ctx.Doer.ID, true)
	if err != nil {
		ctx.ServerError("GetStarListsByUserID", err)
		return
	}

	err = starListSlice.LoadRepoIDs(ctx)
	if err != nil {
		ctx.ServerError("LoadRepoIDs", err)
		return
	}

	for _, starList := range starListSlice {
		if slices.Contains(form.StarListID, starList.ID) {
			err = starList.AddRepo(ctx, ctx.Repo.Repository.ID)
			if err != nil {
				ctx.ServerError("StarListAddRepo", err)
				return
			}
		} else {
			err = starList.RemoveRepo(ctx, ctx.Repo.Repository.ID)
			if err != nil {
				ctx.ServerError("StarListRemoveRepo", err)
				return
			}
		}
	}

	ctx.Redirect(ctx.Repo.Repository.Link())
}
