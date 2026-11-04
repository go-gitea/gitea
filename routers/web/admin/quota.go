// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	quota_model "gitea.dev/models/quota"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	"gitea.dev/services/quota"
)

func StorageQuota(ctx *context.Context) {
	status, err := quota.Status(ctx, quota_model.Subject(ctx.PathParam("subject")), ctx.PathParamInt64("id"))
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			ctx.NotFound(err)
		} else {
			ctx.ServerError("StorageQuota", err)
		}
		return
	}
	ctx.Data["Title"] = ctx.Tr("quota.title")
	ctx.Data["StorageQuota"] = status
	for _, field := range []struct {
		name  string
		value *int64
	}{
		{"GitOverride", status.Overrides.GitLimit}, {"LFSOverride", status.Overrides.LFSLimit}, {"TotalOverride", status.Overrides.TotalLimit},
	} {
		if field.value != nil {
			ctx.Data[field.name] = strconv.FormatInt(*field.value, 10)
		}
	}
	ctx.HTML(http.StatusOK, "admin/quota")
}

func StorageQuotaPost(ctx *context.Context) {
	var options api.StorageQuotaOptions
	for _, field := range []struct {
		name  string
		value **int64
	}{
		{"git_limit", &options.GitLimit}, {"lfs_limit", &options.LFSLimit}, {"total_limit", &options.TotalLimit},
	} {
		value := strings.TrimSpace(ctx.FormString(field.name))
		if value == "" {
			continue
		}
		limit, err := setting.ParseQuotaLimit(value)
		if err != nil {
			ctx.JSONError(err.Error())
			return
		}
		*field.value = &limit
	}
	if err := quota.UpdatePolicy(ctx, quota_model.Subject(ctx.PathParam("subject")), ctx.PathParamInt64("id"), options); err != nil {
		ctx.JSONError(err.Error())
		return
	}
	ctx.JSONRedirect("")
}
