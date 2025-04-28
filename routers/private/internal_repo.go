// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	gitea_context "code.gitea.io/gitea/services/context"
)

// This file contains common functions relating to setting the Repository for the internal routes

// RepoAssignment assigns the repository and git repository to the private context
func RepoAssignment(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.PathParam("owner")
	repoName := ctx.PathParam("repo")

	repo := loadRepository(ctx, ownerName, repoName)
	if ctx.Written() {
		// Error handled in loadRepository
		return
	}

	gitRepo, err := gitrepo.RepositoryFromRequestContextOrOpen(ctx, repo)
	if err != nil {
		log.Error("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	ctx.Repo = &gitea_context.Repository{
		Repository: repo,
		GitRepo:    gitRepo,
	}
}

func loadRepository(ctx *gitea_context.PrivateContext, ownerName, repoName string) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, ownerName, repoName)
	if err != nil {
		log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return nil
	}
	if repo.OwnerName == "" {
		repo.OwnerName = ownerName
	}
	return repo
}
