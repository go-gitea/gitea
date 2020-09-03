// +build !windows

// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"net/http"

	"code.gitea.io/gitea/modules/graceful"

	"gitea.com/macaron/macaron"
)

// Restart causes the server to perform a graceful restart if possible but otherwise a graceful shutdown and restart
func Restart(ctx *macaron.Context) {
	graceful.GetManager().DoForcedRestart()
	ctx.PlainText(http.StatusOK, []byte("success"))

}

// Shutdown causes the server to perform a graceful shutdown
func Shutdown(ctx *macaron.Context) {
	graceful.GetManager().DoGracefulShutdown()
	ctx.PlainText(http.StatusOK, []byte("success"))
}
