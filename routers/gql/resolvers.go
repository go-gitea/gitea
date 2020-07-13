// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"context"
	"errors"
	"strconv"
	"strings"

	giteaCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/routers/api/v1/utils"

	"github.com/graphql-go/graphql"
	"github.com/seripap/relay"
)

// RepositoryByIdResolver resolves a repository by id
func RepositoryByIdResolver(id string, goCtx context.Context) (interface{}, error)  {
	var err error

	internalId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, errors.New("Unable to parse id")
	}

	ctx := goCtx.Value("giteaApiContext").(*giteaCtx.APIContext)

	// Get repository.
	repo, err := models.GetRepositoryByID(internalId)
	if err != nil {
		return nil, err
	}
	repo.MustOwner()

	ctx.Repo.Owner = repo.Owner
	ctx.Repo.Repository = repo
	ctx.Repo.Permission, err = models.GetUserRepoPermission(repo, ctx.User)
	if err != nil {
		return nil, err
	}

	if !ctx.Repo.HasAccess() {
		return nil, errors.New("repo not found")
	}

	err = authorizeRepository(ctx)
	if err != nil {
		return nil, err
	}

	gqlRepo := repo.GqlFormat(models.AccessModeRead)
	return *gqlRepo, nil
}

// RepositoryResolver resolves a repository
func RepositoryResolver(p graphql.ResolveParams) (interface{}, error) {
	owner, ownerOk := p.Args["owner"].(string)
	name, nameOk := p.Args["name"].(string)
	if ownerOk && nameOk {
		ctx := p.Context.Value("giteaApiContext").(*giteaCtx.APIContext)

		var (
			repoOwner *models.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(owner) {
			repoOwner = ctx.User
		} else {
			repoOwner, err = models.GetUserByName(owner)
			if err != nil {
				return nil, err
			}
		}
		ctx.Repo.Owner = repoOwner

		// Get repository.
		repo, err := models.GetRepositoryByName(repoOwner.ID, name)
		if err != nil {
			return nil, err
		}

		repo.Owner = repoOwner
		ctx.Repo.Repository = repo

		ctx.Repo.Permission, err = models.GetUserRepoPermission(repo, ctx.User)
		if err != nil {
			return nil, err
		}

		if !ctx.Repo.HasAccess() {
			return nil, errors.New("repo not found")
		}

		err = authorizeRepository(ctx)
		if err != nil {
			return nil, err
		}

		gqlRepo := repo.GqlFormat(models.AccessModeRead)
		return *gqlRepo, nil
	}

	return nil, errors.New("both owner and repository name must be provided")
}

func authorizeRepository(ctx *giteaCtx.APIContext) error {
	if !utils.IsAnyRepoReader(ctx) {
		return errors.New("Must have permission to read repository")
	}
	return nil
}

// CollaboratorsResolver resolves collaborators list for a repository
func CollaboratorsResolver(p graphql.ResolveParams) (interface{}, error) {
	ctx := p.Context.Value("giteaApiContext").(*giteaCtx.APIContext)
	err := authorizeCollaborators(ctx)
	if err != nil {
		return nil, err
	}

	//TODO, have to refactor relay helper lib to work with gitea pagination model
	limitOptions := models.ListOptions{
		Page:     0,
		PageSize: 50,
	}
	collaborators, err := ctx.Repo.Repository.GetCollaborators(limitOptions)
	if err != nil {
		return nil, err
	}
	users := []interface{}{}
	for _, collaborator := range collaborators {
		user := convert.ToUser(collaborator.User, ctx.IsSigned, ctx.User != nil && ctx.User.IsAdmin)
		users = append(users, user)
	}

	args := relay.NewConnectionArguments(p.Args)
	return relay.ConnectionFromArray(users, args), nil
}

// UserByIdResolver resolves a user by id
func UserByIdResolver(id string, goCtx context.Context) (interface{}, error) {
	internalId, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, errors.New("Unable to parse id")
	}
	user, err := models.GetUserByID(internalId)
	if err != nil {
		return nil, errors.New("Unable to find user")
	}
	return user.APIFormat(), nil
}

func authorizeCollaborators(ctx *giteaCtx.APIContext) error {
	if _, found :=  ctx.Data["IsApiToken"]; !found {
		return errors.New("Api token missing or invalid")
	}
	if !utils.IsAnyRepoReader(ctx) {
		return errors.New("Must have permission to read repository")
	}
	return nil
}

// BranchesResolver resolves the branches of a repository
func BranchesResolver(p graphql.ResolveParams) (interface{}, error) {
	ctx := p.Context.Value("giteaApiContext").(*giteaCtx.APIContext)
	err := authorizeBranches(ctx)
	if err != nil {
		return nil, err
	}

	branches, err := repo_module.GetBranches(ctx.Repo.Repository)
	if err != nil {
		return nil, err
	}

	apiBranches := []interface{}{}
	for i := range branches {
		c, err := branches[i].GetCommit()
		if err != nil {
			return nil, err
		}
		branchProtection, err := ctx.Repo.Repository.GetBranchProtection(branches[i].Name)
		log.Info("branch name %d: %s", i, branches[i].Name)
		if err != nil {
			return nil, err
		}
		apiBranch, err := convert.ToBranch(ctx.Repo.Repository, branches[i], c, branchProtection, ctx.User, ctx.Repo.IsAdmin())
		if err != nil {
			return nil, err
		}
		apiBranches = append(apiBranches, apiBranch)
	}

	return apiBranches, nil
}

func authorizeBranches(ctx *giteaCtx.APIContext) error {
	if !utils.IsRepoReader(ctx, models.UnitTypeCode) {
		return errors.New("Must have read permission or be a repo or site admin")
	}
	return nil
}
