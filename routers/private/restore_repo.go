// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"io"
	"net/http"

	myCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/services/migrations"
)

// RestoreRepo restore a repository from data
func RestoreRepo(ctx *myCtx.PrivateContext) {
	bs, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}
	params := struct {
		Type         string
		RepoFilePath string
		RepoDir      string
		OwnerName    string
		RepoName     string
		Units        []string
		Validation   bool
	}{}
	if err = json.Unmarshal(bs, &params); err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	if params.Type == "gitea" {
		if err := migrations.RestoreRepository(
			ctx,
			params.RepoDir,
			params.OwnerName,
			params.RepoName,
			params.Units,
			params.Validation,
		); err != nil {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
		} else {
			ctx.Status(http.StatusOK)
		}
	} else if params.Type == "github" {
		if err := migrations.RestoreFromGithubExportedData(
			ctx,
			params.RepoFilePath,
			params.OwnerName,
			params.RepoName,
			params.Units,
		); err != nil {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
		} else {
			ctx.Status(http.StatusOK)
		}
	}
}
