// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/webtheme"
)

var _ context.Context = TemplateContext(nil)

func NewTemplateContext(ctx context.Context, req *http.Request) TemplateContext {
	return TemplateContext{"_ctx": ctx, "_req": req}
}

func (c TemplateContext) req() *http.Request {
	return c["_req"].(*http.Request)
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
	var themeName string
	if webCtx := GetWebContext(c); webCtx != nil {
		if webCtx.Doer != nil {
			themeName = webCtx.Doer.Theme
		}
	}
	if themeName == "" {
		themeName = middleware.GetSiteCookie(c.req(), middleware.CookieTheme)
	}
	return webtheme.GuaranteeGetThemeMetaInfo(themeName)
}

func (c TemplateContext) CurrentWebBanner() *setting.WebBannerType {
	// Using revision as a simple approach to determine if the banner has been changed after the user dismissed it.
	// There could be some false-positives because revision can be changed even if the banner isn't.
	// While it should be still good enough (no admin would keep changing the settings) and doesn't really harm end users (just a few more times to see the banner)
	// So it doesn't need to make it more complicated by allocating unique IDs or using hashes.
	dismissedBannerRevision, _ := strconv.Atoi(middleware.GetSiteCookie(c.req(), middleware.CookieWebBannerDismissed))
	banner, revision, _ := setting.Config().Instance.WebBanner.ValueRevision(c)
	if banner.ShouldDisplay() && dismissedBannerRevision != revision {
		return &banner
	}
	return nil
}
