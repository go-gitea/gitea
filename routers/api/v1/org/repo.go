package org

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/sdk/gitea"
)

// ListRepos list all of an organization's repositories.
func ListRepos(ctx *context.APIContext) {
	var apiRepos []*api.Repository
	if ctx.User != nil {
		// Find all repos a user has access to within an org.
		org := ctx.Org.Organization
		reposEnv, err := org.AccessibleReposEnv(ctx.User.ID)
		if err != nil {
			ctx.Error(500, "AccessibleReposEnv", err)
			return
		}
		repos, err := reposEnv.Repos(1, org.NumRepos)
		if err != nil {
			ctx.Error(500, "Repos", err)
			return
		}
		// Convert to API repos.
		apiRepos = make([]*api.Repository, len(repos))
		for i, repo := range repos {
			accessLevel, err := models.AccessLevel(ctx.User.ID, repo)
			if err != nil {
				ctx.Error(500, "AccessLevel", err)
				return
			}
			apiRepos[i] = repo.APIFormat(accessLevel)
		}
	}
	ctx.JSON(200, &apiRepos)
}
