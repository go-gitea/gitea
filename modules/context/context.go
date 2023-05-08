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
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
)

// Render represents a template render
type Render interface {
	TemplateLookup(tmpl string) (templates.TemplateExecutor, error)
	HTML(w io.Writer, status int, name string, data interface{}) error
}

// Context represents context of a request.
type Context struct {
	Resp     ResponseWriter
	Req      *http.Request
	Data     middleware.ContextData // data used by MVC templates
	PageData map[string]any         // data used by JavaScript modules in one page, it's `window.config.pageData`
	Render   Render
	Locale   translation.Locale
	Cache    cache.Cache
	Csrf     CSRFProtector
	Flash    *middleware.Flash
	Session  session.Store

	Link        string // current request URL
	EscapedLink string
	Doer        *user_model.User
	IsSigned    bool
	IsBasicAuth bool

	ContextUser *user_model.User
	Repo        *Repository
	Org         *Organization
	Package     *Package
}

// Close frees all resources hold by Context
func (ctx *Context) Close() error {
	var err error
	if ctx.Req != nil && ctx.Req.MultipartForm != nil {
		err = ctx.Req.MultipartForm.RemoveAll() // remove the temp files buffered to tmp directory
	}
	// TODO: close opened repo, and more
	return err
}

// TrHTMLEscapeArgs runs ".Locale.Tr()" but pre-escapes all arguments with html.EscapeString.
// This is useful if the locale message is intended to only produce HTML content.
func (ctx *Context) TrHTMLEscapeArgs(msg string, args ...string) string {
	trArgs := make([]interface{}, len(args))
	for i, arg := range args {
		trArgs[i] = html.EscapeString(arg)
	}
	return ctx.Locale.Tr(msg, trArgs...)
}

func (ctx *Context) Tr(msg string, args ...any) string {
	return ctx.Locale.Tr(msg, args...)
}

func (ctx *Context) TrN(cnt any, key1, keyN string, args ...any) string {
	return ctx.Locale.TrN(cnt, key1, keyN, args...)
}

// Deadline is part of the interface for context.Context and we pass this to the request context
func (ctx *Context) Deadline() (deadline time.Time, ok bool) {
	return ctx.Req.Context().Deadline()
}

// Done is part of the interface for context.Context and we pass this to the request context
func (ctx *Context) Done() <-chan struct{} {
	return ctx.Req.Context().Done()
}

// Err is part of the interface for context.Context and we pass this to the request context
func (ctx *Context) Err() error {
	return ctx.Req.Context().Err()
}

// Value is part of the interface for context.Context and we pass this to the request context
func (ctx *Context) Value(key interface{}) interface{} {
	if key == git.RepositoryContextKey && ctx.Repo != nil {
		return ctx.Repo.GitRepo
	}
	if key == translation.ContextKey && ctx.Locale != nil {
		return ctx.Locale
	}
	return ctx.Req.Context().Value(key)
}

type contextKeyType struct{}

var contextKey interface{} = contextKeyType{}

// WithContext set up install context in request
func WithContext(req *http.Request, ctx *Context) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), contextKey, ctx))
}

// GetContext retrieves install context from request
func GetContext(req *http.Request) *Context {
	if ctx, ok := req.Context().Value(contextKey).(*Context); ok {
		return ctx
	}
	return nil
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
			ctx := Context{
				Resp:    NewResponse(resp),
				Cache:   mc.GetCache(),
				Locale:  middleware.Locale(resp, req),
				Link:    setting.AppSubURL + strings.TrimSuffix(req.URL.EscapedPath(), "/"),
				Render:  rnd,
				Session: session.GetSession(req),
				Repo: &Repository{
					PullRequest: &PullRequest{},
				},
				Org:  &Organization{},
				Data: middleware.GetContextData(req.Context()),
			}
			defer ctx.Close()

			ctx.Data.MergeFrom(middleware.CommonTemplateContextData())
			ctx.Data["Context"] = &ctx
			ctx.Data["CurrentURL"] = setting.AppSubURL + req.URL.RequestURI()
			ctx.Data["Link"] = ctx.Link
			ctx.Data["locale"] = ctx.Locale

			// PageData is passed by reference, and it will be rendered to `window.config.pageData` in `head.tmpl` for JavaScript modules
			ctx.PageData = map[string]any{}
			ctx.Data["PageData"] = ctx.PageData

			ctx.Req = WithContext(req, &ctx)
			ctx.Csrf = PrepareCSRFProtector(csrfOpts, &ctx)

			// Get the last flash message from cookie
			lastFlashCookie := middleware.GetSiteCookie(ctx.Req, CookieNameFlash)
			if vals, _ := url.ParseQuery(lastFlashCookie); len(vals) > 0 {
				// store last Flash message into the template data, to render it
				ctx.Data["Flash"] = &middleware.Flash{
					DataStore:  &ctx,
					Values:     vals,
					ErrorMsg:   vals.Get("error"),
					SuccessMsg: vals.Get("success"),
					InfoMsg:    vals.Get("info"),
					WarningMsg: vals.Get("warning"),
				}
			}

			// prepare an empty Flash message for current request
			ctx.Flash = &middleware.Flash{DataStore: &ctx, Values: url.Values{}}
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
