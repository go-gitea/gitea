// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/sso"
	"code.gitea.io/gitea/modules/base"
	mc "code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/middlewares"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
	"github.com/go-chi/chi"
	"github.com/unknwon/com"
	"github.com/unknwon/i18n"
	"github.com/unrolled/render"
	"golang.org/x/crypto/pbkdf2"
)

// Render represents a template render
type Render interface {
	TemplateLookup(tmpl string) *template.Template
	HTML(w io.Writer, status int, name string, binding interface{}, htmlOpt ...render.HTMLOptions) error
}

// Context represents context of a request.
type Context struct {
	Resp   ResponseWriter
	Req    *http.Request
	Data   map[string]interface{}
	Render Render
	translation.Locale
	Cache   cache.Cache
	csrf    CSRF
	Flash   *middlewares.Flash
	Session session.Store

	Link        string // current request URL
	EscapedLink string
	User        *models.User
	IsSigned    bool
	IsBasicAuth bool

	Repo *Repository
	Org  *Organization
}

// GetData returns the data
func (ctx *Context) GetData() map[string]interface{} {
	return ctx.Data
}

// IsUserSiteAdmin returns true if current user is a site admin
func (ctx *Context) IsUserSiteAdmin() bool {
	return ctx.IsSigned && ctx.User.IsAdmin
}

// IsUserRepoOwner returns true if current user owns current repo
func (ctx *Context) IsUserRepoOwner() bool {
	return ctx.Repo.IsOwner()
}

// IsUserRepoAdmin returns true if current user is admin in current repo
func (ctx *Context) IsUserRepoAdmin() bool {
	return ctx.Repo.IsAdmin()
}

// IsUserRepoWriter returns true if current user has write privilege in current repo
func (ctx *Context) IsUserRepoWriter(unitTypes []models.UnitType) bool {
	for _, unitType := range unitTypes {
		if ctx.Repo.CanWrite(unitType) {
			return true
		}
	}

	return false
}

// IsUserRepoReaderSpecific returns true if current user can read current repo's specific part
func (ctx *Context) IsUserRepoReaderSpecific(unitType models.UnitType) bool {
	return ctx.Repo.CanRead(unitType)
}

// IsUserRepoReaderAny returns true if current user can read any part of current repo
func (ctx *Context) IsUserRepoReaderAny() bool {
	return ctx.Repo.HasAccess()
}

// RedirectToUser redirect to a differently-named user
func RedirectToUser(ctx *Context, userName string, redirectUserID int64) {
	user, err := models.GetUserByID(redirectUserID)
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.Path,
		userName,
		user.Name,
		1,
	)
	if ctx.Req.URL.RawQuery != "" {
		redirectPath += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(path.Join(setting.AppSubURL, redirectPath))
}

// HasAPIError returns true if error occurs in form validation.
func (ctx *Context) HasAPIError() bool {
	hasErr, ok := ctx.Data["HasError"]
	if !ok {
		return false
	}
	return hasErr.(bool)
}

// GetErrMsg returns error message
func (ctx *Context) GetErrMsg() string {
	return ctx.Data["ErrorMsg"].(string)
}

// HasError returns true if error occurs in form validation.
func (ctx *Context) HasError() bool {
	hasErr, ok := ctx.Data["HasError"]
	if !ok {
		return false
	}
	ctx.Flash.ErrorMsg = ctx.Data["ErrorMsg"].(string)
	ctx.Data["Flash"] = ctx.Flash
	return hasErr.(bool)
}

// HasValue returns true if value of given name exists.
func (ctx *Context) HasValue(name string) bool {
	_, ok := ctx.Data[name]
	return ok
}

// RedirectToFirst redirects to first not empty URL
func (ctx *Context) RedirectToFirst(location ...string) {
	for _, loc := range location {
		if len(loc) == 0 {
			continue
		}

		u, err := url.Parse(loc)
		if err != nil || ((u.Scheme != "" || u.Host != "") && !strings.HasPrefix(strings.ToLower(loc), strings.ToLower(setting.AppURL))) {
			continue
		}

		ctx.Redirect(loc)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}

// HTML calls Context.HTML and converts template name to string.
func (ctx *Context) HTML(status int, name base.TplName) {
	log.Debug("Template: %s", name)
	if err := ctx.Render.HTML(ctx.Resp, status, string(name), ctx.Data); err != nil {
		ctx.ServerError("Render failed", err)
	}
}

// HTMLString render content to a string but not http.ResponseWriter
func (ctx *Context) HTMLString(name string, data interface{}) (string, error) {
	var buf strings.Builder
	err := ctx.Render.HTML(&buf, 200, string(name), data)
	return buf.String(), err
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *Context) RenderWithErr(msg string, tpl base.TplName, form interface{}) {
	if form != nil {
		middlewares.AssignForm(form, ctx.Data)
	}
	ctx.Flash.ErrorMsg = msg
	ctx.Data["Flash"] = ctx.Flash
	ctx.HTML(200, tpl)
}

// NotFound displays a 404 (Not Found) page and prints the given error, if any.
func (ctx *Context) NotFound(title string, err error) {
	ctx.notFoundInternal(title, err)
}

func (ctx *Context) notFoundInternal(title string, err error) {
	if err != nil {
		log.ErrorWithSkip(2, "%s: %v", title, err)
		if !setting.IsProd() {
			ctx.Data["ErrorMsg"] = err
		}
	}

	ctx.Data["IsRepo"] = ctx.Repo.Repository != nil
	ctx.Data["Title"] = "Page Not Found"
	ctx.HTML(http.StatusNotFound, base.TplName("status/404"))
}

// ServerError displays a 500 (Internal Server Error) page and prints the given
// error, if any.
func (ctx *Context) ServerError(title string, err error) {
	ctx.serverErrorInternal(title, err)
}

func (ctx *Context) serverErrorInternal(title string, err error) {
	if err != nil {
		log.ErrorWithSkip(2, "%s: %v", title, err)
		if !setting.IsProd() {
			ctx.Data["ErrorMsg"] = err
		}
	}

	ctx.Data["Title"] = "Internal Server Error"
	ctx.HTML(http.StatusInternalServerError, base.TplName("status/500"))
}

// NotFoundOrServerError use error check function to determine if the error
// is about not found. It responses with 404 status code for not found error,
// or error context description for logging purpose of 500 server error.
func (ctx *Context) NotFoundOrServerError(title string, errck func(error) bool, err error) {
	if errck(err) {
		ctx.notFoundInternal(title, err)
		return
	}

	ctx.serverErrorInternal(title, err)
}

// Header returns a header
func (ctx *Context) Header() http.Header {
	return ctx.Resp.Header()
}

// FIXME: We should differ Query and Form, currently we just use form as query
// Currently to be compatible with macaron, we keep it.

// Query returns request form as string with default
func (ctx *Context) Query(key string, defaults ...string) string {
	return (*Forms)(ctx.Req).MustString(key, defaults...)
}

// QueryTrim returns request form as string with default and trimmed spaces
func (ctx *Context) QueryTrim(key string, defaults ...string) string {
	return (*Forms)(ctx.Req).MustTrimmed(key, defaults...)
}

// QueryStrings returns request form as strings with default
func (ctx *Context) QueryStrings(key string, defaults ...[]string) []string {
	return (*Forms)(ctx.Req).MustStrings(key, defaults...)
}

// QueryInt returns request form as int with default
func (ctx *Context) QueryInt(key string, defaults ...int) int {
	return (*Forms)(ctx.Req).MustInt(key, defaults...)
}

// QueryInt64 returns request form as int64 with default
func (ctx *Context) QueryInt64(key string, defaults ...int64) int64 {
	return (*Forms)(ctx.Req).MustInt64(key, defaults...)
}

// QueryBool returns request form as bool with default
func (ctx *Context) QueryBool(key string, defaults ...bool) bool {
	return (*Forms)(ctx.Req).MustBool(key, defaults...)
}

// HandleText handles HTTP status code
func (ctx *Context) HandleText(status int, title string) {
	if (status/100 == 4) || (status/100 == 5) {
		log.Error("%s", title)
	}
	ctx.PlainText(status, []byte(title))
}

// ServeContent serves content to http request
func (ctx *Context) ServeContent(name string, r io.ReadSeeker, params ...interface{}) {
	modtime := time.Now()
	for _, p := range params {
		switch v := p.(type) {
		case time.Time:
			modtime = v
		}
	}
	ctx.Resp.Header().Set("Content-Description", "File Transfer")
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	ctx.Resp.Header().Set("Content-Disposition", "attachment; filename="+name)
	ctx.Resp.Header().Set("Content-Transfer-Encoding", "binary")
	ctx.Resp.Header().Set("Expires", "0")
	ctx.Resp.Header().Set("Cache-Control", "must-revalidate")
	ctx.Resp.Header().Set("Pragma", "public")
	ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
	http.ServeContent(ctx.Resp, ctx.Req, name, modtime, r)
}

// PlainText render content as plain text
func (ctx *Context) PlainText(status int, bs []byte) {
	ctx.Resp.WriteHeader(status)
	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf8")
	if _, err := ctx.Resp.Write(bs); err != nil {
		ctx.ServerError("Render JSON failed", err)
	}
}

// ServeFile serves given file to response.
func (ctx *Context) ServeFile(file string, names ...string) {
	var name string
	if len(names) > 0 {
		name = names[0]
	} else {
		name = path.Base(file)
	}
	ctx.Resp.Header().Set("Content-Description", "File Transfer")
	ctx.Resp.Header().Set("Content-Type", "application/octet-stream")
	ctx.Resp.Header().Set("Content-Disposition", "attachment; filename="+name)
	ctx.Resp.Header().Set("Content-Transfer-Encoding", "binary")
	ctx.Resp.Header().Set("Expires", "0")
	ctx.Resp.Header().Set("Cache-Control", "must-revalidate")
	ctx.Resp.Header().Set("Pragma", "public")
	http.ServeFile(ctx.Resp, ctx.Req, file)
}

// Error returned an error to web browser
func (ctx *Context) Error(status int, contents ...string) {
	var v = http.StatusText(status)
	if len(contents) > 0 {
		v = contents[0]
	}
	http.Error(ctx.Resp, v, status)
}

// JSON render content as JSON
func (ctx *Context) JSON(status int, content interface{}) {
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf8")
	ctx.Resp.WriteHeader(status)
	if err := json.NewEncoder(ctx.Resp).Encode(content); err != nil {
		ctx.ServerError("Render JSON failed", err)
	}
}

// Redirect redirect the request
func (ctx *Context) Redirect(location string, status ...int) {
	code := http.StatusFound
	if len(status) == 1 {
		code = status[0]
	}

	http.Redirect(ctx.Resp, ctx.Req, location, code)
}

// SetCookie set cookies to web browser
func (ctx *Context) SetCookie(name string, value string, others ...interface{}) {
	middlewares.SetCookie(ctx.Resp, name, value, others...)
}

// GetCookie returns given cookie value from request header.
func (ctx *Context) GetCookie(name string) string {
	return middlewares.GetCookie(ctx.Req, name)
}

// GetSuperSecureCookie returns given cookie value from request header with secret string.
func (ctx *Context) GetSuperSecureCookie(secret, name string) (string, bool) {
	val := ctx.GetCookie(name)
	if val == "" {
		return "", false
	}

	text, err := hex.DecodeString(val)
	if err != nil {
		return "", false
	}

	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err = com.AESGCMDecrypt(key, text)
	return string(text), err == nil
}

// SetSuperSecureCookie sets given cookie value to response header with secret string.
func (ctx *Context) SetSuperSecureCookie(secret, name, value string, others ...interface{}) {
	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err := com.AESGCMEncrypt(key, []byte(value))
	if err != nil {
		panic("error encrypting cookie: " + err.Error())
	}

	ctx.SetCookie(name, hex.EncodeToString(text), others...)
}

// GetCookieInt returns cookie result in int type.
func (ctx *Context) GetCookieInt(name string) int {
	r, _ := strconv.Atoi(ctx.GetCookie(name))
	return r
}

// GetCookieInt64 returns cookie result in int64 type.
func (ctx *Context) GetCookieInt64(name string) int64 {
	r, _ := strconv.ParseInt(ctx.GetCookie(name), 10, 64)
	return r
}

// GetCookieFloat64 returns cookie result in float64 type.
func (ctx *Context) GetCookieFloat64(name string) float64 {
	v, _ := strconv.ParseFloat(ctx.GetCookie(name), 64)
	return v
}

// RemoteAddr returns the client machie ip address
func (ctx *Context) RemoteAddr() string {
	return ctx.Req.RemoteAddr
}

// Params returns the param on route
func (ctx *Context) Params(p string) string {
	s, _ := url.PathUnescape(chi.URLParam(ctx.Req, strings.TrimPrefix(p, ":")))
	return s
}

// ParamsInt64 returns the param on route as int64
func (ctx *Context) ParamsInt64(p string) int64 {
	v, _ := strconv.ParseInt(ctx.Params(p), 10, 64)
	return v
}

// SetParams set params into routes
func (ctx *Context) SetParams(k, v string) {
	chiCtx := chi.RouteContext(ctx.Req.Context())
	chiCtx.URLParams.Add(strings.TrimPrefix(k, ":"), url.PathEscape(v))
}

// Write writes data to webbrowser
func (ctx *Context) Write(bs []byte) (int, error) {
	return ctx.Resp.Write(bs)
}

// Written returns true if there are something sent to web browser
func (ctx *Context) Written() bool {
	return ctx.Resp.Status() > 0
}

// Status writes status code
func (ctx *Context) Status(status int) {
	ctx.Resp.WriteHeader(status)
}

// Handler represents a custom handler
type Handler func(*Context)

// enumerate all content
var (
	contextKey interface{} = "default_context"
)

// WithContext set up install context in request
func WithContext(req *http.Request, ctx *Context) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), contextKey, ctx))
}

// GetContext retrieves install context from request
func GetContext(req *http.Request) *Context {
	return req.Context().Value(contextKey).(*Context)
}

// SignedUserName returns signed user's name via context
func SignedUserName(req *http.Request) string {
	if middlewares.IsInternalPath(req) {
		return ""
	}
	if middlewares.IsAPIPath(req) {
		ctx, ok := req.Context().Value(apiContextKey).(*APIContext)
		if ok {
			v := ctx.Data["SignedUserName"]
			if res, ok := v.(string); ok {
				return res
			}
		}
	} else {
		ctx, ok := req.Context().Value(contextKey).(*Context)
		if ok {
			v := ctx.Data["SignedUserName"]
			if res, ok := v.(string); ok {
				return res
			}
		}
	}
	return ""
}

func getCsrfOpts() CsrfOptions {
	return CsrfOptions{
		Secret:         setting.SecretKey,
		Cookie:         setting.CSRFCookieName,
		SetCookie:      true,
		Secure:         setting.SessionConfig.Secure,
		CookieHTTPOnly: setting.CSRFCookieHTTPOnly,
		Header:         "X-Csrf-Token",
		CookieDomain:   setting.SessionConfig.Domain,
		CookiePath:     setting.SessionConfig.CookiePath,
	}
}

// Contexter initializes a classic context for a request.
func Contexter() func(next http.Handler) http.Handler {
	var rnd = templates.HTMLRenderer()
	var csrfOpts = getCsrfOpts()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			var locale = middlewares.Locale(resp, req)
			var startTime = time.Now()
			var link = setting.AppSubURL + strings.TrimSuffix(req.URL.EscapedPath(), "/")
			var ctx = Context{
				Resp:    NewResponse(resp),
				Cache:   mc.GetCache(),
				Locale:  locale,
				Link:    link,
				Render:  rnd,
				Session: session.GetSession(req),
				Repo: &Repository{
					PullRequest: &PullRequest{},
				},
				Org: &Organization{},
				Data: map[string]interface{}{
					"CurrentURL":    setting.AppSubURL + req.URL.RequestURI(),
					"PageStartTime": startTime,
					"TmplLoadTimes": func() string {
						return time.Since(startTime).String()
					},
					"Link": link,
				},
			}

			ctx.Req = WithContext(req, &ctx)
			ctx.csrf = Csrfer(csrfOpts, &ctx)

			// Get flash.
			flashCookie := ctx.GetCookie("macaron_flash")
			vals, _ := url.ParseQuery(flashCookie)
			if len(vals) > 0 {
				f := &middlewares.Flash{
					DataStore:  &ctx,
					Values:     vals,
					ErrorMsg:   vals.Get("error"),
					SuccessMsg: vals.Get("success"),
					InfoMsg:    vals.Get("info"),
					WarningMsg: vals.Get("warning"),
				}
				ctx.Data["Flash"] = f
			}

			f := &middlewares.Flash{
				DataStore:  &ctx,
				Values:     url.Values{},
				ErrorMsg:   "",
				WarningMsg: "",
				InfoMsg:    "",
				SuccessMsg: "",
			}
			ctx.Resp.Before(func(resp ResponseWriter) {
				if flash := f.Encode(); len(flash) > 0 {
					middlewares.SetCookie(resp, "macaron_flash", flash, 0,
						setting.SessionConfig.CookiePath,
						middlewares.Domain(setting.SessionConfig.Domain),
						middlewares.HTTPOnly(true),
						middlewares.Secure(setting.SessionConfig.Secure),
						//middlewares.SameSite(opt.SameSite), FIXME: we need a samesite config
					)
					return
				}

				ctx.SetCookie("macaron_flash", "", -1,
					setting.SessionConfig.CookiePath,
					middlewares.Domain(setting.SessionConfig.Domain),
					middlewares.HTTPOnly(true),
					middlewares.Secure(setting.SessionConfig.Secure),
					//middlewares.SameSite(), FIXME: we need a samesite config
				)
			})

			ctx.Flash = f

			// Quick responses appropriate go-get meta with status 200
			// regardless of if user have access to the repository,
			// or the repository does not exist at all.
			// This is particular a workaround for "go get" command which does not respect
			// .netrc file.
			if ctx.Query("go-get") == "1" {
				ownerName := ctx.Params(":username")
				repoName := ctx.Params(":reponame")
				trimmedRepoName := strings.TrimSuffix(repoName, ".git")

				if ownerName == "" || trimmedRepoName == "" {
					_, _ = ctx.Write([]byte(`<!doctype html>
<html>
	<body>
		invalid import path
	</body>
</html>
`))
					ctx.Status(400)
					return
				}
				branchName := "master"

				repo, err := models.GetRepositoryByOwnerAndName(ownerName, repoName)
				if err == nil && len(repo.DefaultBranch) > 0 {
					branchName = repo.DefaultBranch
				}
				prefix := setting.AppURL + path.Join(url.PathEscape(ownerName), url.PathEscape(repoName), "src", "branch", util.PathEscapeSegments(branchName))

				appURL, _ := url.Parse(setting.AppURL)

				insecure := ""
				if appURL.Scheme == string(setting.HTTP) {
					insecure = "--insecure "
				}
				ctx.Header().Set("Content-Type", "text/html")
				ctx.Status(http.StatusOK)
				_, _ = ctx.Write([]byte(com.Expand(`<!doctype html>
<html>
	<head>
		<meta name="go-import" content="{GoGetImport} git {CloneLink}">
		<meta name="go-source" content="{GoGetImport} _ {GoDocDirectory} {GoDocFile}">
	</head>
	<body>
		go get {Insecure}{GoGetImport}
	</body>
</html>
`, map[string]string{
					"GoGetImport":    ComposeGoGetImport(ownerName, trimmedRepoName),
					"CloneLink":      models.ComposeHTTPSCloneURL(ownerName, repoName),
					"GoDocDirectory": prefix + "{/dir}",
					"GoDocFile":      prefix + "{/dir}/{file}#L{line}",
					"Insecure":       insecure,
				})))
				return
			}

			// If request sends files, parse them here otherwise the Query() can't be parsed and the CsrfToken will be invalid.
			if ctx.Req.Method == "POST" && strings.Contains(ctx.Req.Header.Get("Content-Type"), "multipart/form-data") {
				if err := ctx.Req.ParseMultipartForm(setting.Attachment.MaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
					ctx.ServerError("ParseMultipartForm", err)
					return
				}
			}

			// Get user from session if logged in.
			ctx.User, ctx.IsBasicAuth = sso.SignedInUser(ctx.Req, ctx.Resp, &ctx, ctx.Session)

			if ctx.User != nil {
				ctx.IsSigned = true
				ctx.Data["IsSigned"] = ctx.IsSigned
				ctx.Data["SignedUser"] = ctx.User
				ctx.Data["SignedUserID"] = ctx.User.ID
				ctx.Data["SignedUserName"] = ctx.User.Name
				ctx.Data["IsAdmin"] = ctx.User.IsAdmin
			} else {
				ctx.Data["SignedUserID"] = int64(0)
				ctx.Data["SignedUserName"] = ""
			}

			ctx.Resp.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

			ctx.Data["CsrfToken"] = html.EscapeString(ctx.csrf.GetToken())
			ctx.Data["CsrfTokenHtml"] = template.HTML(`<input type="hidden" name="_csrf" value="` + ctx.Data["CsrfToken"].(string) + `">`)
			log.Debug("Session ID: %s", ctx.Session.ID())
			log.Debug("CSRF Token: %v", ctx.Data["CsrfToken"])

			ctx.Data["IsLandingPageHome"] = setting.LandingPageURL == setting.LandingPageHome
			ctx.Data["IsLandingPageExplore"] = setting.LandingPageURL == setting.LandingPageExplore
			ctx.Data["IsLandingPageOrganizations"] = setting.LandingPageURL == setting.LandingPageOrganizations

			ctx.Data["ShowRegistrationButton"] = setting.Service.ShowRegistrationButton
			ctx.Data["ShowMilestonesDashboardPage"] = setting.Service.ShowMilestonesDashboardPage
			ctx.Data["ShowFooterBranding"] = setting.ShowFooterBranding
			ctx.Data["ShowFooterVersion"] = setting.ShowFooterVersion

			ctx.Data["EnableSwagger"] = setting.API.EnableSwagger
			ctx.Data["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
			ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations

			ctx.Data["ManifestData"] = setting.ManifestData

			ctx.Data["i18n"] = locale
			ctx.Data["Tr"] = i18n.Tr
			ctx.Data["Lang"] = locale.Language()
			ctx.Data["AllLangs"] = translation.AllLangs()
			for _, lang := range translation.AllLangs() {
				if lang.Lang == locale.Language() {
					ctx.Data["LangName"] = lang.Name
					break
				}
			}

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
