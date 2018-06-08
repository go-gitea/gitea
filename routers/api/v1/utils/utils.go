// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import "code.gitea.io/gitea/modules/context"

// UserID user ID of authenticated user, or 0 if not authenticated
func UserID(ctx *context.APIContext) int64 {
	if ctx.User == nil {
		return 0
	}
	return ctx.User.ID
}
