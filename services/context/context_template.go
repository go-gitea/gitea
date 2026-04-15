// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/webtheme"
)

type TemplateContext map[string]any

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

// AppFullLink returns a full URL link with AppSubURL for the given app link (no AppSubURL)
// If no link is given, it returns the current app full URL with sub-path but without trailing slash (that's why it is not named as AppURL)
func (c TemplateContext) AppFullLink(link ...string) template.URL {
	s := httplib.GuessCurrentAppURL(c.parentContext())
	s = strings.TrimSuffix(s, "/")
	if len(link) == 0 {
		return template.URL(s)
	}
	return template.URL(s + strings.TrimPrefix(link[0], "/"))
}

var globalVars = sync.OnceValue(func() (ret struct {
	scriptImportRemainingPart string
},
) {
	// add onerror handler to alert users when the script fails to load:
	// * for end users: there were many users reporting that "UI doesn't work", actually they made mistakes in their config
	// * for developers: help them to remember to run "make watch-frontend" to build frontend assets
	// the message will be directly put in the onerror JS code's string
	onScriptErrorPrompt := `Please make sure the asset files can be accessed.`
	if !setting.IsProd {
		onScriptErrorPrompt += `\n\nFor development, run: make watch-frontend.`
	}
	onScriptErrorJS := fmt.Sprintf(`alert('Failed to load asset file from ' + this.src + '. %s')`, onScriptErrorPrompt)
	ret.scriptImportRemainingPart = `onerror="` + html.EscapeString(onScriptErrorJS) + `"></script>`
	return ret
})

func (c TemplateContext) ScriptImport(path string, typ ...string) template.HTML {
	if len(typ) > 0 {
		if typ[0] == "module" {
			return template.HTML(`<script nonce="` + c.CspScriptNonce() + `" type="module" src="` + html.EscapeString(public.AssetURI(path)) + `" ` + globalVars().scriptImportRemainingPart)
		}
		panic("unsupported script type: " + typ[0])
	}
	return template.HTML(`<script nonce="` + c.CspScriptNonce() + `" src="` + html.EscapeString(public.AssetURI(path)) + `" ` + globalVars().scriptImportRemainingPart)
}

func (c TemplateContext) CspScriptNonce() (ret string) {
	ret, _ = c["_cspScriptNonce"].(string)
	if ret == "" {
		ret = util.FastCryptoRandomHex(32) // 16 bytes / 128 bits entropy
		c["_cspScriptNonce"] = ret
	}
	return ret
}

func (c TemplateContext) HeadMetaContentSecurityPolicy() template.HTML {
	return template.HTML(`<meta http-equiv="Content-Security-Policy" content="` +
		`default-src *` + // allow all by default (the same as old releases with no CSP)
		`script-src * 'nonce-` + c.CspScriptNonce() + `';` +
		`style-src * 'unsafe-inline';` + // it seems that Vue needs it, need to investigate
		`">`)
}
