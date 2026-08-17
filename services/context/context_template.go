// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/htmlutil"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/public"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/web/middleware"
	"gitea.dev/services/webtheme"
)

type TemplateContext map[string]any

var _ context.Context = TemplateContext(nil)

func NewTemplateContext(ctx reqctx.RequestContext, req *http.Request) TemplateContext {
	return TemplateContext{"_ctx": ctx, "_req": req}
}

func (c TemplateContext) req() *http.Request {
	return c["_req"].(*http.Request) //nolint:forcetypeassert // must exist
}

func (c TemplateContext) parentContext() reqctx.RequestContext {
	return c["_ctx"].(reqctx.RequestContext) //nolint:forcetypeassert // must exist
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

func (c TemplateContext) ImpersonatedUser() *user_model.User {
	webCtx := GetWebContext(c)
	if webCtx == nil || webCtx.Doer == nil || !webCtx.DoerIsImpersonated() {
		return nil
	}
	return webCtx.Doer
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

// AppFullLink returns a full URL link with AppSubURL for the given app link
// If no link is given, it returns the current app full URL with sub-path but without trailing slash (that's why it is not named as AppURL)
func (c TemplateContext) AppFullLink(link ...string) template.URL {
	s := httplib.GuessCurrentAppURL(c.parentContext())
	s = strings.TrimSuffix(s, "/")
	if len(link) == 0 {
		return template.URL(s)
	}
	return template.URL(s + "/" + strings.TrimPrefix(link[0], "/"))
}

func (c TemplateContext) ScriptImport(path string, typ ...string) template.HTML {
	if len(typ) > 0 {
		if typ[0] == "module" {
			return template.HTML(`<script nonce="` + c.CspScriptNonce() + `" type="module" src="` + html.EscapeString(public.AssetURI(path)) + `"></script>`)
		}
		panic("unsupported script type: " + typ[0])
	}
	return template.HTML(`<script nonce="` + c.CspScriptNonce() + `" src="` + html.EscapeString(public.AssetURI(path)) + `"></script>`)
}

func (c TemplateContext) CspScriptNonce() (ret string) {
	return CspScriptNonce(c.parentContext())
}

func WebContentSecurityPolicy(scriptNonce string) string {
	if setting.Security.ContentSecurityPolicyGeneral == "unset" {
		return "" // if site admin disables the general CSP, then we don't use it
	}
	// The CSP problem is more complicated than it looks.
	// Gitea was designed to support various "customizations", including:
	// * custom themes (custom CSS and JS)
	// * custom assets URL (CDN)
	// * custom plugins and external renders (e.g.: PlantUML render, and the renders might also load some JS/CSS assets)
	// There is no easy way for end users to make the CSP "source" completely right.
	//
	// There can be 2 approaches in the future:
	// A. Let end users to configure their reverse proxy to add CSP header
	//    * Browsers will merge and use the stricter rules between Gitea and reverse proxy
	// B. Introduce some config options in "app.ini"
	//    * Maybe this approach should be avoided, don't make the config system too complex, just let users use A

	// allow all by default (the same as old releases with no CSP)
	// * maybe some images or markup (external) renders need "data:", need to investigate
	// * avatar upload editor needs "blob:", at least "img-src" and "content-src"
	return `default-src * data: blob:;` +

		// enforce nonce for all scripts, disallow inline scripts
		`script-src * 'nonce-` + scriptNonce + `';` +

		// it seems that Vue needs the unsafe-inline, and our custom colors (e.g.: label) also need it
		`style-src * 'unsafe-inline';`
}

func (c TemplateContext) HeadMetaContentSecurityPolicy() template.HTML {
	scriptNonce := c.CspScriptNonce()
	csp := WebContentSecurityPolicy(scriptNonce)
	if csp == "" {
		return ""
	}
	return htmlutil.HTMLFormat(`<meta http-equiv="Content-Security-Policy" content="%s">`, csp)
}
