// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"

	"gitea.dev/modules/jsonlimit"
	"gitea.dev/services/audit"
	myCtx "gitea.dev/services/context"
	"gitea.dev/services/migrations"
)

// RestoreRepo restore a repository from data
func RestoreRepo(ctx *myCtx.PrivateContext) {
	params := struct {
		RepoDir    string
		OwnerName  string
		RepoName   string
		Units      []string
		Validation bool
	}{}
	if err := jsonlimit.DecodeFromRequest(ctx.Resp, ctx.Req, 10*1024*1024, &params); err != nil {
		ctx.PrivateUserErrorf(http.StatusBadRequest, "invalid request body: %v", err)
		return
	}

	if err := migrations.RestoreRepository(
		audit.WithCLIDoer(ctx),
		params.RepoDir,
		params.OwnerName,
		params.RepoName,
		params.Units,
		params.Validation,
	); err != nil {
		ctx.PrivateInternalErrorf("%v", err)
	} else {
		ctx.PlainText(http.StatusOK, "success")
	}
}
