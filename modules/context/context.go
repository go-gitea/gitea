// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
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
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	web_types "code.gitea.io/gitea/modules/web/types"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
)

// Render represents a template render
type Render interface {
	TemplateLookup(tmpl string, templateCtx context.Context) (templates.TemplateExecutor, error)
	HTML(w io.Writer, status int, name string, data any, templateCtx context.Context) error
}

// Context represents context of a request.
type Context struct {
	*Base

	TemplateContext TemplateContext

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

type TemplateContext map[string]any

func init() {
	web.RegisterResponseStatusProvider[*Context](func(req *http.Request) web_types.ResponseStatusProvider {
		return req.Context().Value(WebContextKey).(*Context)
	})
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

func NewTemplateContextForWeb(ctx *Context) TemplateContext {
	tmplCtx := NewTemplateContext(ctx)
	tmplCtx["Locale"] = ctx.Base.Locale
	tmplCtx["AvatarUtils"] = templates.NewAvatarUtils(ctx)
	return tmplCtx
}

func NewWebContext(base *Base, render Render, session session.Store) *Context {
	ctx := &Context{
		Base:    base,
		Render:  render,
		Session: session,

		Cache: mc.GetCache(),
		Link:  setting.AppSubURL + strings.TrimSuffix(base.Req.URL.EscapedPath(), "/"),
		Repo:  &Repository{PullRequest: &PullRequest{}},
		Org:   &Organization{},
	}
	ctx.TemplateContext = NewTemplateContextForWeb(ctx)
	ctx.Flash = &middleware.Flash{DataStore: ctx, Values: url.Values{}}
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
			defer baseCleanUp()
			ctx := NewWebContext(base, rnd, session.GetSession(req))

			ctx.Data.MergeFrom(middleware.CommonTemplateContextData())
			ctx.Data["Context"] = ctx // TODO: use "ctx" in template and remove this
			ctx.Data["CurrentURL"] = setting.AppSubURL + req.URL.RequestURI()
			ctx.Data["Link"] = ctx.Link

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

			// if there are new messages in the ctx.Flash, write them into cookie
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

func (ctx *Context) JSONRedirect(redirect string) {
	ctx.JSON(http.StatusOK, map[string]any{"redirect": redirect})
}

func (ctx *Context) JSONOK() {
	ctx.JSON(http.StatusOK, map[string]any{"ok": true}) // this is only a dummy response, frontend seldom uses it
}

func (ctx *Context) JSONError(msg string) {
	ctx.JSON(http.StatusBadRequest, map[string]any{"errorMessage": msg})
}
