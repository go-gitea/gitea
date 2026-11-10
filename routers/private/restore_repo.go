// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"io"
	"net/http"

	audit_model "gitea.dev/models/audit"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/json"
	"gitea.dev/modules/log"
	"gitea.dev/services/audit"
	myCtx "gitea.dev/services/context"
	"gitea.dev/services/migrations"
)

// RestoreRepo restore a repository from data
func RestoreRepo(ctx *myCtx.PrivateContext) {
	bs, err := io.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}
	params := struct {
		RepoDir    string
		OwnerName  string
		RepoName   string
		Units      []string
		Validation bool
	}{}
	if err = json.Unmarshal(bs, &params); err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}

	if err := migrations.RestoreRepository(
		ctx,
		params.RepoDir,
		params.OwnerName,
		params.RepoName,
		params.Units,
		params.Validation,
	); err != nil {
		ctx.PrivateInternalErrorf("%v", err)
		return
	}
	if repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, params.OwnerName, params.RepoName); err != nil {
		log.Error("GetRepositoryByOwnerAndName: %v", err)
	} else {
		audit.Record(audit.WithCLIDoer(ctx), audit_model.RepositoryCreate, repo)
	}
	ctx.PlainText(http.StatusOK, "success")
}
