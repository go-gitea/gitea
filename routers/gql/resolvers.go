// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"errors"

	"code.gitea.io/gitea/routers/api/v1/utils"
	"github.com/graphql-go/graphql"
	giteaCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
)

// RepositoryResolver resolves the repository
func RepositoryResolver(p graphql.ResolveParams) (interface{}, error) {
	ctx := p.Context.Value("giteaApiContext").(giteaCtx.APIContext)
	err := authorizeRepository(ctx)
	if err != nil {
		return nil, err
	}
	owner, ownerOk := p.Args["owner"].(string)
	name, nameOk := p.Args["name"].(string)
	if ownerOk && nameOk {
		repo, err := models.GetRepositoryByOwnerAndName(owner, name)
		if err != nil {
			return nil, err
		}

		//set repo into the root value map so child resolvers can access it
		rootValue := p.Info.RootValue.(map[string]interface{})
		rootValue["repo"] = repo

		gqlRepo := repo.GqlFormat(models.AccessModeRead)
		return *gqlRepo, nil
	}

	return nil, errors.New("both owner and repository name must be provided")
}

func authorizeRepository(ctx giteaCtx.APIContext) error {
	if !utils.IsAnyRepoReader(&ctx) {
		return errors.New("Must have permission to read repository")
	}
	return nil
}

func CollaboratorsResolver(p graphql.ResolveParams) (interface{}, error) {
	ctx := p.Context.Value("giteaApiContext").(giteaCtx.APIContext)
	err := authorizeCollaborators(ctx)
	if err != nil {
		return nil, err
	}

	rootValue := p.Info.RootValue.(map[string]interface{})
	repo := rootValue["repo"].(*models.Repository)

	limitOptions := models.ListOptions{
		Page:     0,
		PageSize: 50,
	}
	collaborators, err := repo.GetCollaborators(limitOptions)
	if err != nil {
		return nil, err
	}
	users := make([]*api.User, len(collaborators))
	for i, collaborator := range collaborators {
		users[i] = convert.ToUser(collaborator.User, ctx.IsSigned, ctx.User != nil && ctx.User.IsAdmin)
	}
	return users, nil
}

func authorizeCollaborators(ctx giteaCtx.APIContext) error {
	//TODO
	return nil
}

func BranchesResolver(p graphql.ResolveParams) (interface{}, error) {
	ctx := p.Context.Value("giteaApiContext").(giteaCtx.APIContext)
	err := authorizeBranches(ctx)
	if err != nil {
		return nil, err
	}

	rootValue := p.Info.RootValue.(map[string]interface{})
	repo := rootValue["repo"].(*models.Repository)

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
		apiBranches[i], err = convert.ToBranch(repo, branches[i], c, branchProtection, ctx.User, ctx.Repo.IsAdmin())
		if err != nil {
			return nil, err
		}
	}
	return apiBranches, nil
}

func authorizeBranches(ctx giteaCtx.APIContext) error {
	//TODO
	return nil
}
