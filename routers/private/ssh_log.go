// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"

	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web"
	"gitea.dev/services/context"
)

// SSHLog hook to response ssh log
func SSHLog(ctx *context.PrivateContext) {
	if !setting.Log.EnableSSHLog {
		ctx.Status(http.StatusOK)
		return
	}

	opts := web.GetForm(ctx).(*private.SSHLogOption)

	if opts.IsError {
		log.Error("ssh: %v", opts.Message)
		ctx.Status(http.StatusOK)
		return
	}

	log.Debug("ssh: %v", opts.Message)
	ctx.Status(http.StatusOK)
}
