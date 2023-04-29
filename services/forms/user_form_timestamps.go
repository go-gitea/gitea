// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"code.gitea.io/gitea/modules/context"
)

// UserTimestampsFromRequest parses the form for the absolute timestamps preference
func UserTimestampsFromRequest(ctx *context.Context) *UpdateTimestampsForm {
	timestampsForm := &UpdateTimestampsForm{PreferAbsoluteTimestamps: ctx.FormBool("prefer_absolute_timestamps")}
	return timestampsForm
}
