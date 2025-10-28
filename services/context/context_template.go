// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"net/http"
	"time"

	"code.gitea.io/gitea/services/webtheme"
)

var _ context.Context = TemplateContext(nil)

func NewTemplateContext(ctx context.Context, req *http.Request) TemplateContext {
	return TemplateContext{"_ctx": ctx, "_req": req}
}

func (c TemplateContext) parentContext() context.Context {
	return c["_ctx"].(context.Context)
}

func (c TemplateContext) Deadline() (deadline time.Time, ok bool) {
	return c.parentContext().Deadline()
}

func (c TemplateContext) Done() <-chan struct{} {
	return c.parentContext().Done()
}

func (c TemplateContext) Err() error {
	return c.parentContext().Err()
}

func (c TemplateContext) Value(key any) any {
	return c.parentContext().Value(key)
}

func (c TemplateContext) CurrentWebTheme() *webtheme.ThemeMetaInfo {
	req := c["_req"].(*http.Request)
	var themeName string
	if webCtx := GetWebContext(c); webCtx != nil {
		if webCtx.Doer != nil {
			themeName = webCtx.Doer.Theme
		}
	}
	if themeName == "" {
		if cookieTheme, _ := req.Cookie("gitea_theme"); cookieTheme != nil {
			themeName = cookieTheme.Value
		}
	}
	return webtheme.GuaranteeGetThemeMetaInfo(themeName)
}
