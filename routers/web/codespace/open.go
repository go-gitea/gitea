// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"

	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

// Open redirects an authenticated user to the workspace Gateway open-code exchange.
func Open(ctx *context.Context) {
	openEndpoint(ctx, "workspace")
}

// OpenEndpoint redirects an authenticated user to a specific Endpoint Gateway open-code exchange.
func OpenEndpoint(ctx *context.Context) {
	endpointID := ctx.PathParam("endpoint_id")
	if endpointID == "workspace" {
		ctx.NotFound(nil)
		return
	}
	openEndpoint(ctx, endpointID)
}

func openEndpoint(ctx *context.Context, endpointID string) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	result, err := codespace_service.OpenEndpoint(ctx, codespace_service.OpenEndpointOptions{
		UserID:      ctx.Doer.ID,
		CodespaceID: codespaceID,
		EndpointID:  endpointID,
	})
	if err != nil {
		if errors.Is(err, codespace_service.ErrOpenEndpointNotFound) {
			ctx.NotFound(nil)
			return
		}
		if errors.Is(err, codespace_service.ErrOpenEndpointUnavailable) {
			ctx.Flash.Warning(ctx.Tr("codespace.open.unavailable"))
			ctx.Redirect(codespaceDetailPath(codespaceID), http.StatusSeeOther)
			return
		}
		ctx.ServerError("OpenEndpoint", err)
		return
	}
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.RespHeader().Set("Referrer-Policy", "no-referrer")
	ctx.Redirect(result.RedirectURL, http.StatusSeeOther)
}
