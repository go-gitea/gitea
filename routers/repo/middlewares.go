package repo

import (
	"fmt"

	"github.com/go-gitea/gitea/models"
	"github.com/go-gitea/gitea/modules/context"
	"github.com/gogits/git-module"
)

func setEditorconfigIfExists(ctx *context.Context) {
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
