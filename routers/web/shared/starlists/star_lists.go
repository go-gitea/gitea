package starlists

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

func GetByName(ctx *context.Context) {

}

func Create(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.StarListForm)

	log.Info("form: %+v", *form)

	err := repo_model.InsertStarList(ctx, &repo_model.StarList{
		UID:  ctx.Doer.ID,
		Name: form.Name,
		Desc: form.Desc,
	})
	if err != nil {
		ctx.ServerError("InsertStarList", err)
		return
	}

	ctx.Redirect(ctx.Doer.HomeLink() + "?tab=stars")
}

func UpdateByName(ctx *context.Context) {

}

func DeleteByName(ctx *context.Context) {

}

func List(ctx *context.Context) {

}

func UpdateListRepos(ctx *context.Context) {

}
