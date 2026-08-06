// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/modules/setting"
	codespace_service "gitea.dev/services/codespace"
	"gitea.dev/services/context"
)

// Stop queues a user stop operation for the creator's Codespace.
func Stop(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	returnPath := codespaceActionReturnPath(codespaceID, ctx.FormString("return_to"), codespaceDetailPath(codespaceID))
	_, err := codespace_service.StopCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "StopCodespace", err, returnPath)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

// Resume queues a user resume operation for the creator's Codespace.
func Resume(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	returnPath := codespaceActionReturnPath(codespaceID, ctx.FormString("return_to"), codespaceDetailPath(codespaceID))
	_, err := codespace_service.ResumeCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "ResumeCodespace", err, returnPath)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

// Delete deletes or queues deletion for the creator's Codespace.
func Delete(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	returnPath := codespaceActionReturnPath(codespaceID, ctx.FormString("return_to"), codespaceListPath("", 1))
	_, err := codespace_service.DeleteCodespace(ctx, lifecycleActionOptions(ctx))
	if err != nil {
		handleLifecycleActionError(ctx, "DeleteCodespace", err, returnPath)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

// Continue records that the creator is still using the running Codespace.
func Continue(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	returnPath := codespaceActionReturnPath(codespaceID, ctx.FormString("return_to"), codespaceDetailPath(codespaceID))
	_, err := codespace_service.ContinueCodespace(ctx, codespace_service.ContinueCodespaceOptions{
		UserID:      ctx.Doer.ID,
		CodespaceID: codespaceID,
	})
	if err != nil {
		handleInteractionError(ctx, "ContinueCodespace", err, returnPath)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

// AutoStop saves the creator's auto-stop setting.
func AutoStop(ctx *context.Context) {
	if ctx.Doer == nil {
		ctx.NotFound(nil)
		return
	}
	codespaceID, ok := codespaceIDParam(ctx)
	if !ok {
		return
	}
	returnPath := codespaceActionReturnPath(codespaceID, ctx.FormString("return_to"), codespaceDetailPath(codespaceID))
	mode := ctx.FormString("mode")
	var timeout int64
	switch mode {
	case codespace_model.AutoStopModeDefault, codespace_model.AutoStopModeNever:
	case codespace_model.AutoStopModeCustom:
		var ok bool
		timeout, ok = parseAutoStopTimeoutForm(ctx)
		if !ok {
			ctx.Flash.Error(ctx.Tr("codespace.auto_stop_invalid_duration"))
			ctx.Redirect(returnPath, http.StatusSeeOther)
			return
		}
	default:
		ctx.Flash.Error(ctx.Tr("codespace.error.invalid_request"))
		ctx.Redirect(returnPath, http.StatusSeeOther)
		return
	}
	_, err := codespace_service.UpdateAutoStop(ctx, codespace_service.UpdateAutoStopOptions{
		UserID:               ctx.Doer.ID,
		CodespaceID:          codespaceID,
		Mode:                 mode,
		CustomTimeoutSeconds: timeout,
	})
	if err != nil {
		if errors.Is(err, codespace_service.ErrInteractionInvalidArgument) {
			ctx.Flash.Error(ctx.Tr("codespace.auto_stop_invalid_range"))
			ctx.Redirect(returnPath, http.StatusSeeOther)
			return
		}
		handleInteractionError(ctx, "UpdateAutoStop", err, returnPath)
		return
	}
	ctx.Flash.Success(ctx.Tr("settings.saved_successfully"))
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

func handleInteractionError(ctx *context.Context, name string, err error, returnPath string) {
	switch {
	case errors.Is(err, codespace_service.ErrInteractionInvalidArgument):
		ctx.Flash.Error(ctx.Tr("codespace.error.invalid_request"))
	case errors.Is(err, codespace_service.ErrInteractionNotFound), errors.Is(err, codespace_service.ErrInteractionPermissionDenied):
		ctx.NotFound(nil)
		return
	case errors.Is(err, codespace_service.ErrInteractionStateUnavailable):
		ctx.Flash.Warning(ctx.Tr("codespace.error.state_unavailable"))
	case errors.Is(err, codespace_service.ErrInteractionVersionExhausted):
		ctx.Flash.Warning(ctx.Tr("codespace.error.version_exhausted"))
	default:
		ctx.ServerError(name, err)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

func handleLifecycleActionError(ctx *context.Context, name string, err error, returnPath string) {
	switch {
	case errors.Is(err, codespace_service.ErrLifecycleActionNotFound), errors.Is(err, codespace_service.ErrLifecycleActionPermissionDenied):
		ctx.NotFound(nil)
		return
	case errors.Is(err, codespace_service.ErrLifecycleActionStateUnavailable):
		ctx.Flash.Warning(ctx.Tr("codespace.error.state_unavailable"))
	case errors.Is(err, codespace_service.ErrLifecycleActionVersionExhausted):
		ctx.Flash.Warning(ctx.Tr("codespace.error.version_exhausted"))
	default:
		ctx.ServerError(name, err)
		return
	}
	ctx.Redirect(returnPath, http.StatusSeeOther)
}

func lifecycleActionOptions(ctx *context.Context) codespace_service.LifecycleActionOptions {
	codespaceID, _ := codespaceIDParam(ctx)
	return codespace_service.LifecycleActionOptions{
		UserID:      ctx.Doer.ID,
		CodespaceID: codespaceID,
	}
}

func codespaceIDParam(ctx *context.Context) (int64, bool) {
	codespaceID, err := strconv.ParseInt(ctx.PathParam("codespace_id"), 10, 64)
	if err != nil || codespaceID <= 0 {
		ctx.NotFound(nil)
		return 0, false
	}
	return codespaceID, true
}

func codespaceDetailPath(codespaceID int64) string {
	return "/-/codespaces/" + strconv.FormatInt(codespaceID, 10)
}

func codespaceActionReturnPath(codespaceID int64, raw, fallback string) string {
	fallback = setting.AppSubURL + fallback
	if raw == "" {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Fragment != "" {
		return fallback
	}
	listPath := setting.AppSubURL + "/-/codespaces"
	detailPath := setting.AppSubURL + codespaceDetailPath(codespaceID)
	if parsed.Path != listPath && parsed.Path != detailPath {
		return fallback
	}
	return parsed.String()
}

func codespaceListPath(owner string, page int) string {
	values := url.Values{}
	if owner != "" {
		values.Set("owner", owner)
	}
	if page > 1 {
		values.Set("page", strconv.Itoa(page))
	}
	path := "/-/codespaces"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}
	return path
}

func parseAutoStopTimeoutForm(ctx *context.Context) (int64, bool) {
	value, err := strconv.ParseInt(ctx.FormString("timeout_value"), 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	var multiplier int64
	switch ctx.FormString("timeout_unit") {
	case "seconds":
		multiplier = 1
	case "minutes":
		multiplier = 60
	case "hours":
		multiplier = 60 * 60
	case "days":
		multiplier = 24 * 60 * 60
	default:
		return 0, false
	}
	if value > math.MaxInt64/multiplier {
		return 0, false
	}
	return value * multiplier, true
}
