// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

// Stop queues a user stop operation for the creator's Codespace.
func Stop(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	_, err := codespace_service.StopCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "StopCodespace", err)
		return
	}
	ctx.Redirect(codespaceDetailPath(ctx.PathParam("uuid")), http.StatusSeeOther)
}

// Resume queues a user resume operation for the creator's Codespace.
func Resume(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	_, err := codespace_service.ResumeCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "ResumeCodespace", err)
		return
	}
	ctx.Redirect(codespaceDetailPath(ctx.PathParam("uuid")), http.StatusSeeOther)
}

// Delete deletes or queues deletion for the creator's Codespace.
func Delete(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	_, err := codespace_service.DeleteCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "DeleteCodespace", err)
		return
	}
	ctx.Redirect(validCodespaceReturnTo(ctx.FormString("return_to")), http.StatusSeeOther)
}

// Continue records that the creator is still using the running Codespace.
func Continue(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	_, err := codespace_service.ContinueCodespace(ctx, codespace_service.ContinueCodespaceOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
	})
	if err != nil {
		handleInteractionError(ctx, "ContinueCodespace", err)
		return
	}
	ctx.Redirect(codespaceDetailPath(ctx.PathParam("uuid")), http.StatusSeeOther)
}

// AutoStop saves the creator's auto-stop setting.
func AutoStop(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	timeout, ok := parseOptionalTimeoutSecondsForm(ctx, 0)
	if !ok {
		ctx.PlainText(http.StatusBadRequest, "invalid_argument")
		return
	}
	_, err := codespace_service.UpdateAutoStop(ctx, codespace_service.UpdateAutoStopOptions{
		UserID:               ctx.Doer.ID,
		CodespaceUUID:        ctx.PathParam("uuid"),
		Mode:                 ctx.FormString("mode"),
		CustomTimeoutSeconds: timeout,
	})
	if err != nil {
		handleInteractionError(ctx, "UpdateAutoStop", err)
		return
	}
	ctx.Redirect(codespaceDetailPath(ctx.PathParam("uuid")), http.StatusSeeOther)
}

func handleInteractionError(ctx *context.Context, name string, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrInteractionInvalidArgument):
		ctx.PlainText(http.StatusBadRequest, "invalid_argument")
	case errors.Is(err, codespace_service.ErrInteractionNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrInteractionPermissionDenied):
		ctx.PlainText(http.StatusForbidden, "permission_denied")
	case errors.Is(err, codespace_service.ErrInteractionStateUnavailable):
		ctx.PlainText(http.StatusConflict, "state_unavailable")
	case errors.Is(err, codespace_service.ErrInteractionVersionExhausted):
		ctx.PlainText(http.StatusConflict, "version_exhausted")
	default:
		ctx.ServerError(name, err)
	}
}

func handleLifecycleActionError(ctx *context.Context, name string, err error) {
	switch {
	case errors.Is(err, codespace_service.ErrLifecycleActionNotFound):
		ctx.NotFound(nil)
	case errors.Is(err, codespace_service.ErrLifecycleActionPermissionDenied):
		ctx.PlainText(http.StatusForbidden, "permission_denied")
	case errors.Is(err, codespace_service.ErrLifecycleActionStateUnavailable):
		ctx.PlainText(http.StatusConflict, "state_unavailable")
	case errors.Is(err, codespace_service.ErrLifecycleActionVersionExhausted):
		ctx.PlainText(http.StatusConflict, "version_exhausted")
	default:
		ctx.ServerError(name, err)
	}
}

func lifecycleActionOptions(ctx *context.Context) codespace_service.LifecycleActionOptions {
	return codespace_service.LifecycleActionOptions{
		UserID:        ctx.Doer.ID,
		CodespaceUUID: ctx.PathParam("uuid"),
	}
}

func codespaceDetailPath(codespaceUUID string) string {
	return "/-/codespaces/" + codespaceUUID
}

func validCodespaceReturnTo(raw string) string {
	if raw == "" {
		return "/-/codespaces"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" {
		return "/-/codespaces"
	}
	if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "/-/codespaces"
	}
	return parsed.String()
}

func parseOptionalTimeoutSecondsForm(ctx *context.Context, def int64) (int64, bool) {
	raw := ctx.FormString("timeout_seconds")
	if raw == "" {
		return def, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil
}
