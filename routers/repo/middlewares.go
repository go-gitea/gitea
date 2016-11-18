package repo

import (
	"fmt"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
)

func SetEditorconfigIfExists(ctx *context.Context) {
	ec, err := ctx.Repo.GetEditorconfig()

	if err != nil && !git.IsErrNotExist(err) {
		description := fmt.Sprintf("Error while getting .editorconfig file: %v", err)
		if err := models.CreateRepositoryNotice(description); err != nil {
			ctx.Handle(500, "ErrCreatingReporitoryNotice", err)
		}
		return
	}

	ctx.Data["Editorconfig"] = ec
}

func SetDiffViewStyle(ctx *context.Context) {
	var (
		userStyle  = ctx.User.DiffViewStyle
		queryStyle = ctx.Query("style")
		style      string
	)

	if queryStyle == "unified" || queryStyle == "split" {
		style = queryStyle
	} else if userStyle == "unified" || userStyle == "split" {
		style = userStyle
	} else {
		style = "unified"
	}

	ctx.Data["IsSplitStyle"] = style == "split"
	if err := ctx.User.UpdateDiffViewStyle(style); err != nil {
		ctx.Handle(500, "ErrUpdateDiffViewStyle", err)
	}
}
