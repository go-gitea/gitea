package gql

import (
	"github.com/graphql-go/graphql"

	"code.gitea.io/gitea/models"
)

type Resolver struct {
}

// RepositoryResolver resolves our repository
func (r *Resolver) RepositoryResolver(p graphql.ResolveParams) (interface{}, error) {
	//owner, ownerOk := p.Args["name"].(string)
	owner, ownerOk := p.Args["owner"].(string)
	repo, repoOk := p.Args["repo"].(string)
	//if ownerOk && nameOk {
	if ownerOk && repoOk {
		//it would be great here if we could call routers/api/v1/repo/repo.go Search function as
		//that has all the logic. However, you pass a http context type object there and it is ued.
		//repositories := .GetUsersByName(name)
		//repositories := []api.Repository{}
		//s := make([]string, 3)
		//repositories := make([]api.Repository{}, 1)
		//results[i] = repo.APIFormat(accessMode)
		/*
			repo := api.Repository{
				ID:          0,
				Name:        repo,
				Owner:       &api.User{UserName: owner},
				FullName:    owner + "/" + repo,
				Description: "here is a description",
			}
			var repos = []api.Repository{repo}
			return repos, nil
		*/
		//ctx.JSON(http.StatusOK, ctx.Repo.Repository.APIFormat(ctx.Repo.AccessMode))
		repo, err := models.GetRepositoryByOwnerAndName(owner, repo)
		if err != nil {
			//TODO
		}

		gqlRepo := api.GqlRepository{
			RepoInfo: repo.GqlFormat(models.AccessModeRead),
		}
		var gqlRepos = []api.GqlRepository{gqlRepo}
		return gqlRepos, nil
	}

	return nil, nil
}
