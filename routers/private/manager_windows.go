// +build windows

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
)

// Restart is not implemented for Windows based servers as they can't fork
func Restart(ctx *context.PrivateContext) {
	ctx.JSON(http.StatusNotImplemented, map[string]interface{}{
		"err": "windows servers cannot be gracefully restarted - shutdown and restart manually",
	})
}

// Shutdown causes the server to perform a graceful shutdown
func Shutdown(ctx *context.PrivateContext) {
	graceful.GetManager().DoGracefulShutdown()
	ctx.PlainText(http.StatusOK, []byte("success"))
}
