package group

import (
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func LoadHeaderCount(ctx *context.Context) error {
	repoCount, err := repo_model.CountRepository(ctx, repo_model.SearchRepoOptions{
		Actor:              ctx.Doer,
		Private:            ctx.IsSigned,
		GroupID:            ctx.RepoGroup.Group.ID,
		Collaborate:        optional.Some(false),
		IncludeDescription: setting.UI.SearchRepoDescription,
	})
	if err != nil {
		return err
	}
	ctx.Data["RepoCount"] = repoCount

	return nil
}
