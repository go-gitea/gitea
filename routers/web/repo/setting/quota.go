// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"time"

	quota_model "gitea.dev/models/quota"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/templates"
	"gitea.dev/services/context"
	"gitea.dev/services/quota"
	repo_service "gitea.dev/services/repository"
)

func Storage(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.storage")
	ctx.Data["PageIsSettingsStorage"] = true
	storageQuota, limited, err := quota.StatusIfLimited(ctx, quota_model.Repository, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("Storage", err)
		return
	}
	if limited {
		ctx.Data["StorageQuota"] = storageQuota
	}
	ctx.HTML(http.StatusOK, templates.TplName("repo/settings/storage"))
}

func QuotaGC(ctx *context.Context) {
	quotaCtx, op, err := quota.Begin(ctx, ctx.Repo.Repository)
	if err != nil {
		ctx.JSONError(err.Error())
		return
	}
	defer op.Finish()
	if op == nil {
		ctx.JSONError(ctx.Tr("quota.gc_unavailable"))
		return
	}
	allowed, err := quota_model.RequestGC(quotaCtx, ctx.Repo.Repository.ID)
	if err != nil {
		ctx.ServerError("RequestGC", err)
		return
	}
	if !allowed {
		ctx.JSONError(ctx.Tr("quota.gc_rate_limit"))
		return
	}
	timeout := time.Duration(setting.Git.Timeout.GC) * time.Second
	if err = repo_service.GitGcRepo(quotaCtx, ctx.Repo.Repository, timeout, nil); err != nil {
		ctx.ServerError("GitGcRepo", err)
		return
	}
	if exists, err := git.IsRepositoryExist(quotaCtx, ctx.Repo.Repository.WikiStorageRepo()); err != nil {
		ctx.ServerError("IsRepositoryExist", err)
		return
	} else if exists {
		if err = gitcmd.NewCommand("gc").WithRepo(ctx.Repo.Repository.WikiStorageRepo()).WithTimeout(timeout).Run(quotaCtx); err != nil {
			ctx.ServerError("WikiGC", err)
			return
		}
	}
	ctx.JSONRedirect(ctx.Repo.RepoLink + "/settings/storage")
}
