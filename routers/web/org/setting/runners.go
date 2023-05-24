// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"code.gitea.io/gitea/modules/context"
)

func RedirectToDefaultSetting(ctx *context.Context) {
	ctx.Redirect(ctx.Org.OrgLink + "/settings/actions/runners")
}
