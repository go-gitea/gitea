package gql

import (
	"github.com/graphql-go/graphql"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
)

type Resolver struct {
}

// RepositoryResolver resolves our repository
func (r *Resolver) RepositoryResolver(p graphql.ResolveParams) (interface{}, error) {
	owner, ownerOk := p.Args["owner"].(string)
	repo, repoOk := p.Args["repo"].(string)
	if ownerOk && repoOk {
		repo, err := models.GetRepositoryByOwnerAndName(owner, repo)
		if err != nil {
			//TODO
		}

		gqlRepo := repo.GqlFormat(models.AccessModeRead)
		//TODO how do you only get this when query asked for it?
		gqlRepo.Branches, err = getBranches(repo)
		if err != nil {
			//TODO
		}

		var gqlRepos = []api.GqlRepository{*gqlRepo}
		return gqlRepos, nil
	}

	return nil, nil
}

func getBranches(repo *models.Repository) ([]*api.Branch, error) {
	branches, err := repo_module.GetBranches(repo)
	if err != nil {
		return nil, err
	}

	apiBranches := make([]*api.Branch, len(branches))
	for i := range branches {
		c, err := branches[i].GetCommit()
		if err != nil {
			return nil, err
		}
		branchProtection, err := repo.GetBranchProtection(branches[i].Name)
		if err != nil {
			return nil, err
		}
		//TODO how to get user that is calling the api?
		//apiBranches[i], err = convert.ToBranch(repo, branches[i], c, branchProtection, ctx.User, ctx.Repo.IsAdmin())
		apiBranches[i], err = convert.ToBranch(repo, branches[i], c, branchProtection, nil, true)
		if err != nil {
			return nil, err
		}
	}
	return apiBranches, nil
}
