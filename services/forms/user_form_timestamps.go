// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"code.gitea.io/gitea/modules/context"
)

// UserTimestampsFromRequest parse the form to hidden comment types bitset
func UserTimestampsFromRequest(ctx *context.Context) *UpdateTimestampsForm {
	timestampsForm := &UpdateTimestampsForm{ForceAbsoluteTimestamps: ctx.FormBool("force_absolute_timestamps")}
	return timestampsForm
}
