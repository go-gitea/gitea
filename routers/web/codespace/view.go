// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	"gitea.dev/modules/templates"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

const (
	tplCodespaceList        templates.TplName = "codespace/list"
	tplCodespaceDetail      templates.TplName = "codespace/detail"
	tplCodespaceState       templates.TplName = "codespace/state"
	tplCodespaceRepoListing templates.TplName = "codespace/repository"
)

// List renders the current user's Codespaces.
func List(ctx *context.Context) {
	renderList(ctx, 0, tplCodespaceList)
}

// RepositoryList renders the current user's Codespaces for the current repository.
func RepositoryList(ctx *context.Context) {
	if ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound(nil)
		return
	}
	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	renderList(ctx, ctx.Repo.Repository.ID, tplCodespaceRepoListing)
}

func renderList(ctx *context.Context, repoID int64, tpl templates.TplName) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	result, err := codespace_service.ListCreatorCodespaces(ctx, codespace_service.CreatorListOptions{
		UserID: ctx.Doer.ID,
		RepoID: repoID,
	})
	if err != nil {
		ctx.ServerError("ListCreatorCodespaces", err)
		return
	}
	ctx.Data["Title"] = "Codespaces"
	ctx.Data["Codespaces"] = result.Rows
	ctx.Data["RefType"] = ctx.FormString("ref_type")
	ctx.Data["RefName"] = ctx.FormString("ref_name")
	ctx.HTML(http.StatusOK, tpl)
}

// Detail renders the current user's single Codespace page.
func Detail(ctx *context.Context) {
	view, ok := loadCreatorDetail(ctx)
	if !ok {
		return
	}
	ctx.Data["Title"] = "Codespace"
	ctx.Data["Codespace"] = view
	if !loadCreatorLogPreview(ctx, view.UUID) {
		return
	}
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.HTML(http.StatusOK, tplCodespaceDetail)
}

// State renders the live state fragment for a single Codespace.
func State(ctx *context.Context) {
	view, ok := loadCreatorDetail(ctx)
	if !ok {
		return
	}
	ctx.Data["Codespace"] = view
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.HTML(http.StatusOK, tplCodespaceState)
}

func loadCreatorDetail(ctx *context.Context) (*codespace_service.CreatorCodespaceView, bool) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return nil, false
	}
	view, err := codespace_service.GetCreatorCodespace(ctx, codespace_service.CreatorDetailOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrViewNotFound):
			ctx.NotFound(nil)
		case errors.Is(err, codespace_service.ErrViewPermissionDenied):
			ctx.PlainText(http.StatusForbidden, "permission_denied")
		default:
			ctx.ServerError("GetCreatorCodespace", err)
		}
		return nil, false
	}
	return view, true
}

func loadCreatorLogPreview(ctx *context.Context, codespaceUUID string) bool {
	result, err := codespace_service.ReadLog(ctx, codespace_service.ReadLogOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: codespaceUUID,
		Offset:        0,
		Limit:         codespace_service.LogReadMaxBytes,
	})
	if err != nil {
		switch {
		case errors.Is(err, codespace_service.ErrReadLogPermissionDenied):
			ctx.PlainText(http.StatusForbidden, "permission_denied")
		case errors.Is(err, codespace_service.ErrReadLogNotFound):
			ctx.NotFound(nil)
		default:
			ctx.ServerError("ReadLog", err)
		}
		return false
	}
	ctx.Data["LogLines"] = result.Lines
	ctx.Data["LogNextOffset"] = result.NextOffset
	ctx.Data["LogEOF"] = result.EOF
	return true
}
