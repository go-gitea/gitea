package user

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// listUserRepos - List the repositories owned by the given user.
func listUserRepos(ctx *context.APIContext, u *models.User) {
	userID := u.ID
	showPrivateRepos := ctx.IsSigned && (ctx.User.ID == userID || ctx.User.IsAdmin)
	ownRepos, err := models.GetUserRepositories(userID, showPrivateRepos, 1, u.NumRepos, "")
	if err != nil {
		ctx.Error(500, "GetUserRepositories", err)
		return
	}
	var accessibleRepos []*api.Repository
	if ctx.User != nil {
		accessibleRepos, err = getAccessibleRepos(ctx)
		if err != nil {
			ctx.Error(500, "GetAccessibleRepos", err)
		}
	}
	apiRepos := make([]*api.Repository, len(ownRepos)+len(accessibleRepos))
	// Set owned repositories.
	for i := range ownRepos {
		apiRepos[i] = ownRepos[i].APIFormat(models.AccessModeOwner)
	}
	// Set repositories user has access to.
	for i := 0; i < len(accessibleRepos); i++ {
		apiRepos[i+len(ownRepos)] = accessibleRepos[i]
	}
	ctx.JSON(200, &apiRepos)
}

// ListUserRepos - list the repos owned and accessible by the given user.
func ListUserRepos(ctx *context.APIContext) {
	user := GetUserByParams(ctx)
	if ctx.Written() {
		return
	}
	listUserRepos(ctx, user)
}

// ListMyRepos - list the repositories owned by you.
// see https://github.com/gogits/go-gogs-client/wiki/Repositories#list-your-repositories
func ListMyRepos(ctx *context.APIContext) {
	listUserRepos(ctx, ctx.User)
}

// getAccessibleRepos - Get the repositories a user has access to.
func getAccessibleRepos(ctx *context.APIContext) ([]*api.Repository, error) {
	accessibleRepos, err := ctx.User.GetRepositoryAccesses()
	if err != nil {
		return nil, err
	}
	i := 0
	repos := make([]*api.Repository, len(accessibleRepos))
	for repo, access := range accessibleRepos {
		repos[i] = repo.APIFormat(access)
		i++
	}
	return repos, nil
}
