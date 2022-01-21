// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"context"
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// __________
// \______   \ ____ ______   ____
//  |       _// __ \\____ \ /  _ \
//  |    |   \  ___/|  |_> >  <_> )
//  |____|_  /\___  >   __/ \____/
//         \/     \/|__|
//    _____                .__                                     __
//   /  _  \   ______ _____|__| ____   ____   _____   ____   _____/  |_
//  /  /_\  \ /  ___//  ___/  |/ ___\ /    \ /     \_/ __ \ /    \   __\
// /    |    \\___ \ \___ \|  / /_/  >   |  \  Y Y  \  ___/|   |  \  |
// \____|__  /____  >____  >__\___  /|___|  /__|_|  /\___  >___|  /__|
//         \/     \/     \/  /_____/      \/      \/     \/     \/

// This file contains common functions relating to setting the Repository for the
// internal routes

// RepoAssignment assigns the repository and gitrepository to the private context
func RepoAssignment(ctx *gitea_context.PrivateContext) context.CancelFunc {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")

	repo := loadRepository(ctx, ownerName, repoName)
	if ctx.Written() {
		// Error handled in loadRepository
		return nil
	}

	gitRepo, err := git.OpenRepositoryCtx(ctx, repo.RepoPath())
	if err != nil {
		log.Error("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Failed to open repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return nil
	}

	ctx.Repo = &gitea_context.Repository{
		Repository: repo,
		GitRepo:    gitRepo,
	}

	// We opened it, we should close it
	cancel := func() {
		// If it's been set to nil then assume someone else has closed it.
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
		}
	}

	return cancel
}

func loadRepository(ctx *gitea_context.PrivateContext, ownerName, repoName string) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"Err": fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return nil
	}
	if repo.OwnerName == "" {
		repo.OwnerName = ownerName
	}
	return repo
}
