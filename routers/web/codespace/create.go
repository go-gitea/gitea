// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

// Create creates a Codespace from the current repository page.
func Create(ctx *context.Context) {
	if ctx.Doer == nil || ctx.Repo == nil || ctx.Repo.Repository == nil {
		ctx.NotFound(nil)
		return
	}
	result, err := codespace_service.CreateCodespace(ctx, codespace_service.CreateCodespaceOptions{
		User:    ctx.Doer,
		Repo:    ctx.Repo.Repository,
		RefType: ctx.FormString("ref_type"),
		RefName: ctx.FormString("ref_name"),
	})
	if err != nil {
		handleCreateError(ctx, err)
		return
	}
	ctx.Redirect(codespaceDetailPath(result.CodespaceUUID), http.StatusSeeOther)
}

func handleCreateError(ctx *context.Context, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrCreatePermissionDenied):
		ctx.PlainText(http.StatusForbidden, "permission_denied")
	case errors.Is(err, codespace_service.ErrCreateStateUnavailable):
		ctx.PlainText(http.StatusConflict, "state_unavailable")
	default:
		ctx.PlainText(http.StatusBadRequest, "invalid_request")
	}
}
