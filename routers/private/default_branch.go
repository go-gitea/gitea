// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	gitea_context "code.gitea.io/gitea/services/context"
)

// SetDefaultBranch updates the default branch
func SetDefaultBranch(ctx *gitea_context.PrivateContext) {
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	branch := ctx.Params(":branch")

	ctx.Repo.Repository.DefaultBranch = branch
	if err := ctx.Repo.GitRepo.SetDefaultBranch(ctx.Repo.Repository.DefaultBranch); err != nil {
		if !git.IsErrUnsupportedVersion(err) {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}

	if err := repo_model.UpdateDefaultBranch(ctx, ctx.Repo.Repository); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to set default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	commitID, err := ctx.Repo.GitRepo.GetBranchCommitID(ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to get commit ID for new default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}
	if err := repo_model.UpdateIndexerStatus(ctx, ctx.Repo.Repository, repo_model.RepoIndexerTypeHook, commitID); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to update hook status for new default branch on repository: %s/%s Error: %v", ownerName, repoName, err),
		})
		return
	}

	ctx.PlainText(http.StatusOK, "success")
}
