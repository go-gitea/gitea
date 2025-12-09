// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

// IsUserSiteAdmin returns true if current user is a site admin
func (ctx *Context) IsUserSiteAdmin() bool {
	return ctx.IsSigned && ctx.Doer.IsAdmin
}
