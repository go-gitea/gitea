// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	mc "code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
)

// Render represents a template render
type Render interface {
	TemplateLookup(tmpl string) (templates.TemplateExecutor, error)
	HTML(w io.Writer, status int, name string, data any) error
}

// Context represents context of a request.
type Context struct {
	*Base

	Render   Render
	PageData map[string]any // data used by JavaScript modules in one page, it's `window.config.pageData`

	Cache   cache.Cache
	Csrf    CSRFProtector
	Flash   *middleware.Flash
	Session session.Store

	Link string // current request URL (without query string)

	Doer        *user_model.User // current signed-in user
	IsSigned    bool
	IsBasicAuth bool

	ContextUser *user_model.User // the user which is being visited, in most cases it differs from Doer

	Repo    *Repository
	Org     *Organization
	Package *Package
}

// TrHTMLEscapeArgs runs ".Locale.Tr()" but pre-escapes all arguments with html.EscapeString.
// This is useful if the locale message is intended to only produce HTML content.
func (ctx *Context) TrHTMLEscapeArgs(msg string, args ...string) string {
	trArgs := make([]any, len(args))
	for i, arg := range args {
		trArgs[i] = html.EscapeString(arg)
	}
	return ctx.Locale.Tr(msg, trArgs...)
}

type webContextKeyType struct{}

var WebContextKey = webContextKeyType{}

func GetWebContext(req *http.Request) *Context {
	ctx, _ := req.Context().Value(WebContextKey).(*Context)
	return ctx
}

// ValidateContext is a special context for form validation middleware. It may be different from other contexts.
type ValidateContext struct {
	*Base
}

// GetValidateContext gets a context for middleware form validation
func GetValidateContext(req *http.Request) (ctx *ValidateContext) {
	if ctxAPI, ok := req.Context().Value(apiContextKey).(*APIContext); ok {
		ctx = &ValidateContext{Base: ctxAPI.Base}
	} else if ctxWeb, ok := req.Context().Value(WebContextKey).(*Context); ok {
		ctx = &ValidateContext{Base: ctxWeb.Base}
	} else {
		panic("invalid context, expect either APIContext or Context")
	}
	return ctx
}

// Contexter initializes a classic context for a request.
func Contexter() func(next http.Handler) http.Handler {
	rnd := templates.HTMLRenderer()
	csrfOpts := CsrfOptions{
		Secret:         setting.SecretKey,
		Cookie:         setting.CSRFCookieName,
		SetCookie:      true,
		Secure:         setting.SessionConfig.Secure,
		CookieHTTPOnly: setting.CSRFCookieHTTPOnly,
		Header:         "X-Csrf-Token",
		CookieDomain:   setting.SessionConfig.Domain,
		CookiePath:     setting.SessionConfig.CookiePath,
		SameSite:       setting.SessionConfig.SameSite,
	}
	if !setting.IsProd {
		CsrfTokenRegenerationInterval = 5 * time.Second // in dev, re-generate the tokens more aggressively for debug purpose
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			base, baseCleanUp := NewBaseContext(resp, req)
			ctx := &Context{
				Base:    base,
				Cache:   mc.GetCache(),
				Link:    setting.AppSubURL + strings.TrimSuffix(req.URL.EscapedPath(), "/"),
				Render:  rnd,
				Session: session.GetSession(req),
				Repo:    &Repository{PullRequest: &PullRequest{}},
				Org:     &Organization{},
			}
			defer baseCleanUp()

			ctx.Data.MergeFrom(middleware.CommonTemplateContextData())
			ctx.Data["Context"] = &ctx
			ctx.Data["CurrentURL"] = setting.AppSubURL + req.URL.RequestURI()
			ctx.Data["Link"] = ctx.Link
			ctx.Data["locale"] = ctx.Locale

			// PageData is passed by reference, and it will be rendered to `window.config.pageData` in `head.tmpl` for JavaScript modules
			ctx.PageData = map[string]any{}
			ctx.Data["PageData"] = ctx.PageData

			ctx.Base.AppendContextValue(WebContextKey, ctx)
			ctx.Base.AppendContextValueFunc(git.RepositoryContextKey, func() any { return ctx.Repo.GitRepo })

			ctx.Csrf = PrepareCSRFProtector(csrfOpts, ctx)

			// Get the last flash message from cookie
			lastFlashCookie := middleware.GetSiteCookie(ctx.Req, CookieNameFlash)
			if vals, _ := url.ParseQuery(lastFlashCookie); len(vals) > 0 {
				// store last Flash message into the template data, to render it
				ctx.Data["Flash"] = &middleware.Flash{
					DataStore:  ctx,
					Values:     vals,
					ErrorMsg:   vals.Get("error"),
					SuccessMsg: vals.Get("success"),
					InfoMsg:    vals.Get("info"),
					WarningMsg: vals.Get("warning"),
				}
			}

			// prepare an empty Flash message for current request
			ctx.Flash = &middleware.Flash{DataStore: ctx, Values: url.Values{}}
			ctx.Resp.Before(func(resp ResponseWriter) {
				if val := ctx.Flash.Encode(); val != "" {
					middleware.SetSiteCookie(ctx.Resp, CookieNameFlash, val, 0)
				} else if lastFlashCookie != "" {
					middleware.SetSiteCookie(ctx.Resp, CookieNameFlash, "", -1)
				}
			})

			// If request sends files, parse them here otherwise the Query() can't be parsed and the CsrfToken will be invalid.
			if ctx.Req.Method == "POST" && strings.Contains(ctx.Req.Header.Get("Content-Type"), "multipart/form-data") {
				if err := ctx.Req.ParseMultipartForm(setting.Attachment.MaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
					ctx.ServerError("ParseMultipartForm", err)
					return
				}
			}

			httpcache.SetCacheControlInHeader(ctx.Resp.Header(), 0, "no-transform")
			ctx.Resp.Header().Set(`X-Frame-Options`, setting.CORSConfig.XFrameOptions)

			ctx.Data["CsrfToken"] = ctx.Csrf.GetToken()
			ctx.Data["CsrfTokenHtml"] = template.HTML(`<input type="hidden" name="_csrf" value="` + ctx.Data["CsrfToken"].(string) + `">`)

			// FIXME: do we really always need these setting? There should be someway to have to avoid having to always set these
			ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations
			ctx.Data["DisableStars"] = setting.Repository.DisableStars
			ctx.Data["EnableActions"] = setting.Actions.Enabled

			ctx.Data["ManifestData"] = setting.ManifestData

			ctx.Data["UnitWikiGlobalDisabled"] = unit.TypeWiki.UnitGlobalDisabled()
			ctx.Data["UnitIssuesGlobalDisabled"] = unit.TypeIssues.UnitGlobalDisabled()
			ctx.Data["UnitPullsGlobalDisabled"] = unit.TypePullRequests.UnitGlobalDisabled()
			ctx.Data["UnitProjectsGlobalDisabled"] = unit.TypeProjects.UnitGlobalDisabled()
			ctx.Data["UnitActionsGlobalDisabled"] = unit.TypeActions.UnitGlobalDisabled()

			ctx.Data["AllLangs"] = translation.AllLangs()

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// HasError returns true if error occurs in form validation.
// Attention: this function changes ctx.Data and ctx.Flash
func (ctx *Context) HasError() bool {
	hasErr, ok := ctx.Data["HasError"]
	if !ok {
		return false
	}
	ctx.Flash.ErrorMsg = ctx.GetErrMsg()
	ctx.Data["Flash"] = ctx.Flash
	return hasErr.(bool)
}

// GetErrMsg returns error message in form validation.
func (ctx *Context) GetErrMsg() string {
	msg, _ := ctx.Data["ErrorMsg"].(string)
	if msg == "" {
		msg = "invalid form data"
	}
	return msg
}
