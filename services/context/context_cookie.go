// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"gitea.dev/modules/web/middleware"
)

const CookieNameFlash = "gitea_flash"

// SetSiteCookie convenience function to set most cookies consistently
func (ctx *Context) SetSiteCookie(name, value string, maxAge int) {
	middleware.SetSiteCookie(ctx.Resp, name, value, maxAge)
}

// DeleteSiteCookie convenience function to delete most cookies consistently
func (ctx *Context) DeleteSiteCookie(name string) {
	middleware.SetSiteCookie(ctx.Resp, name, "", -1)
}

// GetSiteCookie returns given cookie value from request header.
func (ctx *Context) GetSiteCookie(name string) string {
	return middleware.GetSiteCookie(ctx.Req, name)
}
