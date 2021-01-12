// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/middlewares"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"golang.org/x/crypto/pbkdf2"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
	"github.com/go-chi/chi"
	"github.com/unknwon/com"
	"github.com/unrolled/render"
)

type response struct {
	http.ResponseWriter
	written bool
}

func (r *response) Write(bs []byte) (int, error) {
	size, err := r.ResponseWriter.Write(bs)
	if err != nil {
		return 0, err
	}
	r.written = true
	return size, nil
}

func (r *response) WriteHeader(statusCode int) {
	r.written = true
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *response) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Context represents context of a request.
type Context struct {
	Resp   *response
	Req    *http.Request
	Data   map[string]interface{}
	Render *render.Render
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
	ctx.Render.HTML(ctx.Resp, status, string(name), ctx.Data)
}

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

func (ctx *Context) PlainText(status int, bs []byte) {
	ctx.Render.Text(ctx.Resp, status, string(bs))
}

func (ctx *Context) Header() http.Header {
	return ctx.Resp.Header()
}

func (ctx *Context) QueryStrings(k string) []string {
	return ctx.Req.URL.Query()[k]
}

func (ctx *Context) Query(k string) string {
	s := ctx.QueryStrings(k)
	if len(s) == 0 {
		return ""
	}
	return s[0]
}

func (ctx *Context) QueryInt(k string) int {
	s := ctx.Query(k)
	if s == "" {
		return 0
	}
	r, _ := strconv.Atoi(s)
	return r
}

func (ctx *Context) QueryBool(k string) bool {
	s := ctx.Query(k)
	if s == "" {
		return false
	}
	r, _ := strconv.ParseBool(s)
	return r
}

// QueryTrim querys and trims spaces form parameter.
func (ctx *Context) QueryTrim(name string) string {
	return strings.TrimSpace(ctx.Query(name))
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

func (ctx *Context) JSON(status int, content interface{}) {
	ctx.Render.JSON(ctx.Resp, status, content)
}

func (ctx *Context) Redirect(location string, status ...int) {
	code := http.StatusFound
	if len(status) == 1 {
		code = status[0]
	}

	http.Redirect(ctx.Resp, ctx.Req, location, code)
}

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

func (ctx *Context) RemoteAddr() string {
	return ctx.Req.RemoteAddr
}

func (ctx *Context) Params(p string) string {
	return chi.URLParam(ctx.Req, p)
}

func (ctx *Context) ParamsInt64(p string) int64 {
	v, _ := strconv.ParseInt(ctx.Params(p), 10, 64)
	return v
}

func (ctx *Context) WriteHeader(status int) {
	ctx.Resp.WriteHeader(status)
}

func (ctx *Context) Write(bs []byte) (int, error) {
	return ctx.Resp.Write(bs)
}

func (ctx *Context) Written() bool {
	return ctx.Resp.written
}

func (ctx *Context) Status(status int) {
	ctx.Resp.WriteHeader(status)
}

// QueryInt64 returns int64 of query value
func (ctx *Context) QueryInt64(name string) int64 {
	vals := ctx.Req.URL.Query()[name]
	if len(vals) == 0 {
		return 0
	}
	v, _ := strconv.ParseInt(vals[0], 10, 64)
	return v
}

// FIXME?
func (ctx *Context) SetParams(k, v string) {

}

type Handler func(*Context)

var (
	ContextKey interface{} = "default_context"
)

// WithContext set up install context in request
func WithContext(req *http.Request, ctx *Context) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), ContextKey, ctx))
}

// GetContext retrieves install context from request
func GetContext(req *http.Request) *Context {
	return req.Context().Value(ContextKey).(*Context)
}

// Contexter initializes a classic context for a request.
func Contexter() func(next http.Handler) http.Handler {
	rnd := templates.HTMLRenderer()

	c, err := cache.NewCacher(cache.Options{
		Adapter:       setting.CacheService.Adapter,
		AdapterConfig: setting.CacheService.Conn,
		Interval:      setting.CacheService.Interval,
	})
	if err != nil {
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			var locale = middlewares.Locale(resp, req)
			var startTime = time.Now()
			var x CSRF
			var ctx = Context{
				Resp:    &response{resp, false},
				Req:     req,
				Cache:   c,
				csrf:    x,
				Flash:   &middlewares.Flash{},
				Locale:  locale,
				Link:    setting.AppSubURL + strings.TrimSuffix(req.URL.EscapedPath(), "/"),
				Render:  rnd,
				Session: session.GetSession(req),
				Repo: &Repository{
					PullRequest: &PullRequest{},
				},
				Org: &Organization{},
			}
			var data = map[string]interface{}{
				"i18n":          locale,
				"Language":      locale.Language(),
				"CurrentURL":    setting.AppSubURL + req.URL.RequestURI(),
				"PageStartTime": startTime,
				"TmplLoadTimes": func() string {
					return time.Since(startTime).String()
				},
				"Link": ctx.Link,
			}
			ctx.Data = data
			ctx.Req = WithContext(req, &ctx)

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
					ctx.WriteHeader(400)
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
				ctx.WriteHeader(http.StatusOK)
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

			// If request sends files, parse them here otherwise the Query() can't be parsed and the CsrfToken will be invalid.
			if ctx.Req.Method == "POST" && strings.Contains(ctx.Req.Header.Get("Content-Type"), "multipart/form-data") {
				if err := ctx.Req.ParseMultipartForm(setting.Attachment.MaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
					ctx.ServerError("ParseMultipartForm", err)
					return
				}
			}

			ctx.Resp.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

			ctx.Data["CsrfToken"] = html.EscapeString(x.GetToken())
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

			next.ServeHTTP(ctx.Resp, req)
		})
	}
}
