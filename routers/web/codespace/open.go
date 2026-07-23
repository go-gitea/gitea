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

const tplCodespaceOpen templates.TplName = "codespace/open"

// OpenView renders a confirmation page for opening the workspace Gateway target.
func OpenView(ctx *context.Context) {
	openEndpointView(ctx, "workspace")
}

// OpenEndpointView renders a confirmation page for opening a specific Endpoint Gateway target.
func OpenEndpointView(ctx *context.Context) {
	endpointID := ctx.PathParam("endpoint_id")
	if endpointID == "workspace" {
		ctx.NotFound(nil)
		return
	}
	openEndpointView(ctx, endpointID)
}

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
	result, err := codespace_service.OpenEndpoint(ctx, codespace_service.OpenEndpointOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
		EndpointID:    endpointID,
	})
	if err != nil {
		if errors.Is(err, codespace_service.ErrOpenEndpointUnavailable) {
			ctx.PlainText(http.StatusConflict, "Codespace endpoint is not currently available")
			return
		}
		ctx.ServerError("OpenEndpoint", err)
		return
	}
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.RespHeader().Set("Referrer-Policy", "no-referrer")
	ctx.Redirect(result.RedirectURL, http.StatusSeeOther)
}

func openEndpointView(ctx *context.Context, endpointID string) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	info, err := codespace_service.InspectOpenEndpoint(ctx, codespace_service.OpenEndpointOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
		EndpointID:    endpointID,
	})
	if err != nil {
		ctx.ServerError("InspectOpenEndpoint", err)
		return
	}
	ctx.Data["Title"] = "Open Codespace"
	ctx.Data["OpenEndpoint"] = info
	ctx.Data["OpenAction"] = ctx.Req.URL.EscapedPath()
	ctx.RespHeader().Set("Cache-Control", "no-store")
	ctx.HTML(http.StatusOK, tplCodespaceOpen)
}
