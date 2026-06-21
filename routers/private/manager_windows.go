// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build windows

package private

import (
	"net/http"

	"gitea.dev/modules/graceful"
	"gitea.dev/modules/private"
	"gitea.dev/services/context"
)

// Restart is not implemented for Windows based servers as they can't fork
func Restart(ctx *context.PrivateContext) {
	ctx.JSON(http.StatusNotImplemented, private.Response{
		UserMsg: "windows servers cannot be gracefully restarted - shutdown and restart manually",
	})
}

// Shutdown causes the server to perform a graceful shutdown
func Shutdown(ctx *context.PrivateContext) {
	graceful.GetManager().DoGracefulShutdown()
	ctx.PlainText(http.StatusOK, "success")
}
