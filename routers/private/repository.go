// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"net/http"

	"code.gitea.io/gitea/models"

	macaron "gopkg.in/macaron.v1"
)

// GetRepository return the default branch of a repository
func GetRepository(ctx *macaron.Context) {
	repoID := ctx.ParamsInt64(":rid")
	repository, err := models.GetRepositoryByID(repoID)
	repository.MustOwnerName()
	allowPulls := repository.AllowsPulls()
	// put it back to nil because json unmarshal can't unmarshal it
	repository.Units = nil

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	if repository.IsFork {
		repository.GetBaseRepo()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
				"err": err.Error(),
			})
			return
		}
		repository.BaseRepo.MustOwnerName()
		allowPulls = repository.BaseRepo.AllowsPulls()
		// put it back to nil because json unmarshal can't unmarshal it
		repository.BaseRepo.Units = nil
	}

	ctx.JSON(http.StatusOK, struct {
		Repository       *models.Repository
		AllowPullRequest bool
	}{
		Repository:       repository,
		AllowPullRequest: allowPulls,
	})
}

// GetActivePullRequest return an active pull request when it exists or an empty object
func GetActivePullRequest(ctx *macaron.Context) {
	baseRepoID := ctx.QueryInt64("baseRepoID")
	headRepoID := ctx.QueryInt64("headRepoID")
	baseBranch := ctx.QueryTrim("baseBranch")
	if len(baseBranch) == 0 {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": "QueryTrim failed",
		})
		return
	}

	headBranch := ctx.QueryTrim("headBranch")
	if len(headBranch) == 0 {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": "QueryTrim failed",
		})
		return
	}

	pr, err := models.GetUnmergedPullRequest(headRepoID, baseRepoID, headBranch, baseBranch)
	if err != nil && !models.IsErrPullRequestNotExist(err) {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, pr)
}
