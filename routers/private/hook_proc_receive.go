// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"net/http"

	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/agit"
)

// HookProcReceive proc-receive hook - only handles agit Proc-Receive requests at present
func HookProcReceive(ctx *gitea_context.PrivateContext) {
	opts := web.GetForm(ctx).(*private.HookOptions)
	if !git.SupportProcReceive {
		ctx.Status(http.StatusNotFound)
		return
	}

	results := agit.ProcRecive(ctx, opts)
	if ctx.Written() {
		return
	}

	ctx.JSON(http.StatusOK, private.HookProcReceiveResult{
		Results: results,
	})
}
