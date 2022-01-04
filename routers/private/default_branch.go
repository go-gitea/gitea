// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
)

// ________          _____             .__   __
// \______ \   _____/ ____\____   __ __|  |_/  |_
//  |    |  \_/ __ \   __\\__  \ |  |  \  |\   __\
//  |    `   \  ___/|  |   / __ \|  |  /  |_|  |
// /_______  /\___  >__|  (____  /____/|____/__|
//         \/     \/           \/
// __________                             .__
// \______   \____________    ____   ____ |  |__
//  |    |  _/\_  __ \__  \  /    \_/ ___\|  |  \
//  |    |   \ |  | \// __ \|   |  \  \___|   Y  \
//  |______  / |__|  (____  /___|  /\___  >___|  /
//         \/             \/     \/     \/     \/

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	branch := ctx.Params(":branch")
	repo, err := repo_model.GetRepositoryByOwnerAndName(ownerName, repoName)
	if err != nil {
		log.Error("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to get repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if repo.OwnerName == "" {
		repo.OwnerName = ownerName
	}

	repo.DefaultBranch = branch
	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Failed to get git repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if err := gitRepo.SetDefaultBranch(repo.DefaultBranch); err != nil {
		if !git.IsErrUnsupportedVersion(err) {
			gitRepo.Close()
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}
	gitRepo.Close()

	if err := repo_model.UpdateDefaultBranch(repo); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	ctx.PlainText(http.StatusOK, "success")
}
