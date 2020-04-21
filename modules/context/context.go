// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"gitea.com/macaron/cache"
	"gitea.com/macaron/csrf"
	"gitea.com/macaron/i18n"
	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"
	"github.com/unknwon/com"
)

// Context represents context of a request.
type Context struct {
	*macaron.Context
	Cache   cache.Cache
	csrf    csrf.CSRF
	Flash   *session.Flash
	Session session.Store

	Link        string // current request URL
	EscapedLink string
	User        *models.User
	IsSigned    bool
	IsBasicAuth bool

	Repo *Repository
	Org  *Organization
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
	ctx.Context.HTML(status, string(name))
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *Context) RenderWithErr(msg string, tpl base.TplName, form interface{}) {
	if form != nil {
		auth.AssignForm(form, ctx.Data)
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
		if macaron.Env != macaron.PROD {
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
		if macaron.Env != macaron.PROD {
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
	http.ServeContent(ctx.Resp, ctx.Req.Request, name, modtime, r)
}

// Contexter initializes a classic context for a request.
func Contexter() macaron.Handler {
	return func(c *macaron.Context, l i18n.Locale, cache cache.Cache, sess session.Store, f *session.Flash, x csrf.CSRF) {
		ctx := &Context{
			Context: c,
			Cache:   cache,
			csrf:    x,
			Flash:   f,
			Session: sess,
			Link:    setting.AppSubURL + strings.TrimSuffix(c.Req.URL.EscapedPath(), "/"),
			Repo: &Repository{
				PullRequest: &PullRequest{},
			},
			Org: &Organization{},
		}
		ctx.Data["Language"] = ctx.Locale.Language()
		c.Data["Link"] = ctx.Link
		ctx.Data["PageStartTime"] = time.Now()
		// Quick responses appropriate go-get meta with status 200
		// regardless of if user have access to the repository,
		// or the repository does not exist at all.
		// This is particular a workaround for "go get" command which does not respect
		// .netrc file.
		if ctx.Query("go-get") == "1" {
			ownerName := c.Params(":username")
			repoName := c.Params(":reponame")
			trimmedRepoName := strings.TrimSuffix(repoName, ".git")

			if ownerName == "" || trimmedRepoName == "" {
				_, _ = c.Write([]byte(`<!doctype html>
<html>
	<body>
		invalid import path
	</body>
</html>
`))
				c.WriteHeader(400)
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
			c.Header().Set("Content-Type", "text/html")
			c.WriteHeader(http.StatusOK)
			_, _ = c.Write([]byte(com.Expand(`<!doctype html>
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
		ctx.User, ctx.IsBasicAuth = auth.SignedInUser(ctx.Context, ctx.Session)

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
			if err := ctx.Req.ParseMultipartForm(setting.AttachmentMaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
				ctx.ServerError("ParseMultipartForm", err)
				return
			}
		}

		ctx.Resp.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

		ctx.Data["CsrfToken"] = html.EscapeString(x.GetToken())
		ctx.Data["CsrfTokenHtml"] = template.HTML(`<input type="hidden" name="_csrf" value="` + ctx.Data["CsrfToken"].(string) + `">`)
		log.Debug("Session ID: %s", sess.ID())
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

		c.Map(ctx)
	}
}
