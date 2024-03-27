// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

// WIP RequireAction

package setting

import (
	"code.gitea.io/gitea/modules/context"
)

func RedirectToRepoSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions/require_actions")
}
