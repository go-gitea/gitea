// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graphql

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	repo_service "code.gitea.io/gitea/services/repository"
)

// CreateRepository Create a new repository
func (m *Mutation) CreateRepository(ctx context.Context, input CreateRepositoryInput) (*Repository, error) {
	apiCtx := ctx.Value("default_api_context").(*api.APIContext)
	if apiCtx == nil {
		return nil, fmt.Errorf("ctx is empty")
	}
	if !apiCtx.IsSigned {
		return nil, fmt.Errorf("user is not login")
	}

	var (
		owner *models.User
		err   error
	)
	if input.OwnerID != nil {
		if *input.OwnerID == apiCtx.User.ID {
			owner = apiCtx.User
		} else {
			owner, err = models.GetUserByID(*input.OwnerID)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					return nil, fmt.Errorf("owner %v is not exist", *input.OwnerID)
				}
				log.Error("gql: GetUserByID: %v", err)
				return nil, fmt.Errorf("Internal Server Error")
			}
		}
	} else if input.Owner != nil {
		if *input.Owner == apiCtx.User.Name {
			owner = apiCtx.User
		} else {
			owner, err = models.GetUserByName(*input.Owner)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					return nil, fmt.Errorf("owner %s is not exist", *input.Owner)
				}
				log.Error("gql: GetUserByName: %v", err)
				return nil, fmt.Errorf("Internal Server Error")
			}
		}
	} else {
		owner = apiCtx.User
	}

	if owner.ID != apiCtx.User.ID {
		if !owner.IsOrganization() {
			return nil, fmt.Errorf("not allow create repo for other user")
		}
		if !apiCtx.User.IsAdmin {
			canCreate, err := owner.CanCreateOrgRepo(apiCtx.User.ID)
			if err != nil {
				log.Error("gql: CanCreateOrgRepo: %v", err)
				return nil, fmt.Errorf("Internal Server Error")
			} else if !canCreate {
				return nil, fmt.Errorf("Given user is not allowed to create repository in organization %s", owner.Name)
			}
		}
	}

	opts := models.CreateRepoOptions{
		Name:          input.Name,
		IsPrivate:     input.Visibility == RepositoryVisibilityPrivate,
		AutoInit:      input.AutoInit != nil && *input.AutoInit,
		DefaultBranch: input.DefaultBranch,
		TrustModel:    models.ToTrustModel(string(input.TrustModel)),
		IsTemplate:    input.Template != nil && *input.Template,
	}

	if input.AutoInit != nil && *input.AutoInit && input.Readme == nil {
		opts.Readme = "Default"
	}
	if input.Description != nil {
		opts.Description = *input.Description
	}
	if input.IssueLabels != nil {
		opts.IssueLabels = *input.IssueLabels
	}
	if input.Gitignores != nil {
		opts.Gitignores = *input.Gitignores
	}
	if input.License != nil {
		opts.License = *input.License
	}

	repo, err := repo_service.CreateRepository(apiCtx.User, owner, opts)
	if err != nil {
		if models.IsErrRepoAlreadyExist(err) {
			return nil, fmt.Errorf("The repository with the same name already exists")
		} else if models.IsErrNameReserved(err) ||
			models.IsErrNamePatternNotAllowed(err) {
			return nil, err
		}
		log.Error("gql: CreateRepository: %v", err)
		return nil, fmt.Errorf("Internal Server Error")
	}

	// reload repo from db to get a real state after creation
	repo, err = models.GetRepositoryByID(repo.ID)
	if err != nil {
		log.Error("gql: GetRepositoryByID: %v", err)
		return nil, fmt.Errorf("Internal Server Error")
	}

	return convertRepository(repo), nil
}

func convertRepository(repo *models.Repository) *Repository {
	return &Repository{
		Name: repo.Name,
	}
}
