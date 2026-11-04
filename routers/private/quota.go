// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
	"gitea.dev/services/quota"
)

type quotaCompletion struct {
	Token string `json:"token"`
}

func completeQuota(ctx *context.PrivateContext) {
	repo, err := repo_model.GetRepositoryByID(ctx, ctx.PathParamInt64("id"))
	if err != nil {
		ctx.PrivateInternalErrorf("Reading quota repository: %v", err)
		return
	}
	if err = quota.Complete(ctx, repo, web.GetForm[*quotaCompletion](ctx).Token); err != nil {
		ctx.PrivateInternalErrorf("Completing quota operation: %v", err)
		return
	}
	ctx.PlainText(http.StatusOK, "ok")
}
