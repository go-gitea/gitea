// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	mc "code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/cache"
	"gitea.com/go-chi/session"
	chi "github.com/go-chi/chi/v5"
	"github.com/minio/sha256-simd"
	"golang.org/x/crypto/pbkdf2"
)

const CookieNameFlash = "gitea_flash"

// Render represents a template render
type Render interface {
	TemplateLookup(tmpl string) (templates.TemplateExecutor, error)
	HTML(w io.Writer, status int, name string, data interface{}) error
}

// Context represents context of a request.
type Context struct {
	Resp     ResponseWriter
	Req      *http.Request
	Data     map[string]interface{} // data used by MVC templates
	PageData map[string]interface{} // data used by JavaScript modules in one page, it's `window.config.pageData`
	Render   Render
	translation.Locale
	Cache   cache.Cache
	Csrf    CSRFProtector
	Flash   *middleware.Flash
	Session session.Store

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

// TrHTMLEscapeArgs runs Tr but pre-escapes all arguments with html.EscapeString.
// This is useful if the locale message is intended to only produce HTML content.
func (ctx *Context) TrHTMLEscapeArgs(msg string, args ...string) string {
	trArgs := make([]interface{}, len(args))
	for i, arg := range args {
		trArgs[i] = html.EscapeString(arg)
	}
	return ctx.Tr(msg, trArgs...)
}

// GetData returns the data
func (ctx *Context) GetData() map[string]interface{} {
	return ctx.Data
}

// IsUserSiteAdmin returns true if current user is a site admin
func (ctx *Context) IsUserSiteAdmin() bool {
	return ctx.IsSigned && ctx.Doer.IsAdmin
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
func (ctx *Context) IsUserRepoWriter(unitTypes []unit.Type) bool {
	for _, unitType := range unitTypes {
		if ctx.Repo.CanWrite(unitType) {
			return true
		}
	}

	return false
}

// IsUserRepoReaderSpecific returns true if current user can read current repo's specific part
func (ctx *Context) IsUserRepoReaderSpecific(unitType unit.Type) bool {
	return ctx.Repo.CanRead(unitType)
}

// IsUserRepoReaderAny returns true if current user can read any part of current repo
func (ctx *Context) IsUserRepoReaderAny() bool {
	return ctx.Repo.HasAccess()
}

// RedirectToUser redirect to a differently-named user
func RedirectToUser(ctx *Context, userName string, redirectUserID int64) {
	user, err := user_model.GetUserByID(ctx, redirectUserID)
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	redirectPath := strings.Replace(
		ctx.Req.URL.EscapedPath(),
		url.PathEscape(userName),
		url.PathEscape(user.Name),
		1,
	)
	if ctx.Req.URL.RawQuery != "" {
		redirectPath += "?" + ctx.Req.URL.RawQuery
	}
	ctx.Redirect(path.Join(setting.AppSubURL, redirectPath), http.StatusTemporaryRedirect)
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
// Attention: this function changes ctx.Data and ctx.Flash
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

		// Unfortunately browsers consider a redirect Location with preceding "//" and "/\" as meaning redirect to "http(s)://REST_OF_PATH"
		// Therefore we should ignore these redirect locations to prevent open redirects
		if len(loc) > 1 && loc[0] == '/' && (loc[1] == '/' || loc[1] == '\\') {
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

const tplStatus500 base.TplName = "status/500"

// HTML calls Context.HTML and renders the template to HTTP response
func (ctx *Context) HTML(status int, name base.TplName) {
	log.Debug("Template: %s", name)
	tmplStartTime := time.Now()
	if !setting.IsProd {
		ctx.Data["TemplateName"] = name
	}
	ctx.Data["TemplateLoadTimes"] = func() string {
		return strconv.FormatInt(time.Since(tmplStartTime).Nanoseconds()/1e6, 10) + "ms"
	}
	if err := ctx.Render.HTML(ctx.Resp, status, string(name), templates.BaseVars().Merge(ctx.Data)); err != nil {
		if status == http.StatusInternalServerError && name == tplStatus500 {
			ctx.PlainText(http.StatusInternalServerError, "Unable to find HTML templates, the template system is not initialized, or Gitea can't find your template files.")
			return
		}
		err = fmt.Errorf("failed to render template: %s, error: %s", name, templates.HandleTemplateRenderingError(err))
		ctx.ServerError("Render failed", err)
	}
}

// RenderToString renders the template content to a string
func (ctx *Context) RenderToString(name base.TplName, data map[string]interface{}) (string, error) {
	var buf strings.Builder
	err := ctx.Render.HTML(&buf, http.StatusOK, string(name), data)
	return buf.String(), err
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *Context) RenderWithErr(msg string, tpl base.TplName, form interface{}) {
	if form != nil {
		middleware.AssignForm(form, ctx.Data)
	}
	ctx.Flash.ErrorMsg = msg
	ctx.Data["Flash"] = ctx.Flash
	ctx.HTML(http.StatusOK, tpl)
}

// NotFound displays a 404 (Not Found) page and prints the given error, if any.
func (ctx *Context) NotFound(logMsg string, logErr error) {
	ctx.notFoundInternal(logMsg, logErr)
}

func (ctx *Context) notFoundInternal(logMsg string, logErr error) {
	if logErr != nil {
		log.Log(2, log.DEBUG, "%s: %v", logMsg, logErr)
		if !setting.IsProd {
			ctx.Data["ErrorMsg"] = logErr
		}
	}

	// response simple message if Accept isn't text/html
	showHTML := false
	for _, part := range ctx.Req.Header["Accept"] {
		if strings.Contains(part, "text/html") {
			showHTML = true
			break
		}
	}

	if !showHTML {
		ctx.plainTextInternal(3, http.StatusNotFound, []byte("Not found.\n"))
		return
	}

	ctx.Data["IsRepo"] = ctx.Repo.Repository != nil
	ctx.Data["Title"] = "Page Not Found"
	ctx.HTML(http.StatusNotFound, base.TplName("status/404"))
}

// ServerError displays a 500 (Internal Server Error) page and prints the given error, if any.
func (ctx *Context) ServerError(logMsg string, logErr error) {
	ctx.serverErrorInternal(logMsg, logErr)
}

func (ctx *Context) serverErrorInternal(logMsg string, logErr error) {
	if logErr != nil {
		log.ErrorWithSkip(2, "%s: %v", logMsg, logErr)
		if _, ok := logErr.(*net.OpError); ok || errors.Is(logErr, &net.OpError{}) {
			// This is an error within the underlying connection
			// and further rendering will not work so just return
			return
		}

		// it's safe to show internal error to admin users, and it helps
		if !setting.IsProd || (ctx.Doer != nil && ctx.Doer.IsAdmin) {
			ctx.Data["ErrorMsg"] = fmt.Sprintf("%s, %s", logMsg, logErr)
		}
	}

	ctx.Data["Title"] = "Internal Server Error"
	ctx.HTML(http.StatusInternalServerError, tplStatus500)
}

// NotFoundOrServerError use error check function to determine if the error
// is about not found. It responds with 404 status code for not found error,
// or error context description for logging purpose of 500 server error.
func (ctx *Context) NotFoundOrServerError(logMsg string, errCheck func(error) bool, logErr error) {
	if errCheck(logErr) {
		ctx.notFoundInternal(logMsg, logErr)
		return
	}
	ctx.serverErrorInternal(logMsg, logErr)
}

// PlainTextBytes renders bytes as plain text
func (ctx *Context) plainTextInternal(skip, status int, bs []byte) {
	statusPrefix := status / 100
	if statusPrefix == 4 || statusPrefix == 5 {
		log.Log(skip, log.TRACE, "plainTextInternal (status=%d): %s", status, string(bs))
	}
	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	ctx.Resp.WriteHeader(status)
	if _, err := ctx.Resp.Write(bs); err != nil {
		log.ErrorWithSkip(skip, "plainTextInternal (status=%d): write bytes failed: %v", status, err)
	}
}

// PlainTextBytes renders bytes as plain text
func (ctx *Context) PlainTextBytes(status int, bs []byte) {
	ctx.plainTextInternal(2, status, bs)
}

// PlainText renders content as plain text
func (ctx *Context) PlainText(status int, text string) {
	ctx.plainTextInternal(2, status, []byte(text))
}

// RespHeader returns the response header
func (ctx *Context) RespHeader() http.Header {
	return ctx.Resp.Header()
}

type ServeHeaderOptions struct {
	ContentType        string // defaults to "application/octet-stream"
	ContentTypeCharset string
	ContentLength      *int64
	Disposition        string // defaults to "attachment"
	Filename           string
	CacheDuration      time.Duration // defaults to 5 minutes
	LastModified       time.Time
}

// SetServeHeaders sets necessary content serve headers
func (ctx *Context) SetServeHeaders(opts *ServeHeaderOptions) {
	header := ctx.Resp.Header()

	contentType := typesniffer.ApplicationOctetStream
	if opts.ContentType != "" {
		if opts.ContentTypeCharset != "" {
			contentType = opts.ContentType + "; charset=" + strings.ToLower(opts.ContentTypeCharset)
		} else {
			contentType = opts.ContentType
		}
	}
	header.Set("Content-Type", contentType)
	header.Set("X-Content-Type-Options", "nosniff")

	if opts.ContentLength != nil {
		header.Set("Content-Length", strconv.FormatInt(*opts.ContentLength, 10))
	}

	if opts.Filename != "" {
		disposition := opts.Disposition
		if disposition == "" {
			disposition = "attachment"
		}

		backslashEscapedName := strings.ReplaceAll(strings.ReplaceAll(opts.Filename, `\`, `\\`), `"`, `\"`) // \ -> \\, " -> \"
		header.Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, backslashEscapedName, url.PathEscape(opts.Filename)))
		header.Set("Access-Control-Expose-Headers", "Content-Disposition")
	}

	duration := opts.CacheDuration
	if duration == 0 {
		duration = 5 * time.Minute
	}
	httpcache.SetCacheControlInHeader(header, duration)

	if !opts.LastModified.IsZero() {
		header.Set("Last-Modified", opts.LastModified.UTC().Format(http.TimeFormat))
	}
}

// ServeContent serves content to http request
func (ctx *Context) ServeContent(r io.ReadSeeker, opts *ServeHeaderOptions) {
	ctx.SetServeHeaders(opts)
	http.ServeContent(ctx.Resp, ctx.Req, opts.Filename, opts.LastModified, r)
}

// UploadStream returns the request body or the first form file
// Only form files need to get closed.
func (ctx *Context) UploadStream() (rd io.ReadCloser, needToClose bool, err error) {
	contentType := strings.ToLower(ctx.Req.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") || strings.HasPrefix(contentType, "multipart/form-data") {
		if err := ctx.Req.ParseMultipartForm(32 << 20); err != nil {
			return nil, false, err
		}
		if ctx.Req.MultipartForm.File == nil {
			return nil, false, http.ErrMissingFile
		}
		for _, files := range ctx.Req.MultipartForm.File {
			if len(files) > 0 {
				r, err := files[0].Open()
				return r, true, err
			}
		}
		return nil, false, http.ErrMissingFile
	}
	return ctx.Req.Body, false, nil
}

// Error returned an error to web browser
func (ctx *Context) Error(status int, contents ...string) {
	v := http.StatusText(status)
	if len(contents) > 0 {
		v = contents[0]
	}
	http.Error(ctx.Resp, v, status)
}

// JSON render content as JSON
func (ctx *Context) JSON(status int, content interface{}) {
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	ctx.Resp.WriteHeader(status)
	if err := json.NewEncoder(ctx.Resp).Encode(content); err != nil {
		ctx.ServerError("Render JSON failed", err)
	}
}

func removeSessionCookieHeader(w http.ResponseWriter) {
	cookies := w.Header()["Set-Cookie"]
	w.Header().Del("Set-Cookie")
	for _, cookie := range cookies {
		if strings.HasPrefix(cookie, setting.SessionConfig.CookieName+"=") {
			continue
		}
		w.Header().Add("Set-Cookie", cookie)
	}
}

// Redirect redirects the request
func (ctx *Context) Redirect(location string, status ...int) {
	code := http.StatusSeeOther
	if len(status) == 1 {
		code = status[0]
	}

	if strings.Contains(location, "://") || strings.HasPrefix(location, "//") {
		// Some browsers (Safari) have buggy behavior for Cookie + Cache + External Redirection, eg: /my-path => https://other/path
		// 1. the first request to "/my-path" contains cookie
		// 2. some time later, the request to "/my-path" doesn't contain cookie (caused by Prevent web tracking)
		// 3. Gitea's Sessioner doesn't see the session cookie, so it generates a new session id, and returns it to browser
		// 4. then the browser accepts the empty session, then the user is logged out
		// So in this case, we should remove the session cookie from the response header
		removeSessionCookieHeader(ctx.Resp)
	}
	http.Redirect(ctx.Resp, ctx.Req, location, code)
}

// SetSiteCookie convenience function to set most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) SetSiteCookie(name, value string, maxAge int) {
	middleware.SetSiteCookie(ctx.Resp, name, value, maxAge)
}

// DeleteSiteCookie convenience function to delete most cookies consistently
// CSRF and a few others are the exception here
func (ctx *Context) DeleteSiteCookie(name string) {
	middleware.SetSiteCookie(ctx.Resp, name, "", -1)
}

// GetSiteCookie returns given cookie value from request header.
func (ctx *Context) GetSiteCookie(name string) string {
	return middleware.GetSiteCookie(ctx.Req, name)
}

// GetSuperSecureCookie returns given cookie value from request header with secret string.
func (ctx *Context) GetSuperSecureCookie(secret, name string) (string, bool) {
	val := ctx.GetSiteCookie(name)
	return ctx.CookieDecrypt(secret, val)
}

// CookieDecrypt returns given value from with secret string.
func (ctx *Context) CookieDecrypt(secret, val string) (string, bool) {
	if val == "" {
		return "", false
	}

	text, err := hex.DecodeString(val)
	if err != nil {
		return "", false
	}

	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err = util.AESGCMDecrypt(key, text)
	return string(text), err == nil
}

// SetSuperSecureCookie sets given cookie value to response header with secret string.
func (ctx *Context) SetSuperSecureCookie(secret, name, value string, maxAge int) {
	text := ctx.CookieEncrypt(secret, value)
	ctx.SetSiteCookie(name, text, maxAge)
}

// CookieEncrypt encrypts a given value using the provided secret
func (ctx *Context) CookieEncrypt(secret, value string) string {
	key := pbkdf2.Key([]byte(secret), []byte(secret), 1000, 16, sha256.New)
	text, err := util.AESGCMEncrypt(key, []byte(value))
	if err != nil {
		panic("error encrypting cookie: " + err.Error())
	}

	return hex.EncodeToString(text)
}

// GetCookieInt returns cookie result in int type.
func (ctx *Context) GetCookieInt(name string) int {
	r, _ := strconv.Atoi(ctx.GetSiteCookie(name))
	return r
}

// GetCookieInt64 returns cookie result in int64 type.
func (ctx *Context) GetCookieInt64(name string) int64 {
	r, _ := strconv.ParseInt(ctx.GetSiteCookie(name), 10, 64)
	return r
}

// GetCookieFloat64 returns cookie result in float64 type.
func (ctx *Context) GetCookieFloat64(name string) float64 {
	v, _ := strconv.ParseFloat(ctx.GetSiteCookie(name), 64)
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
	chiCtx := chi.RouteContext(ctx)
	chiCtx.URLParams.Add(strings.TrimPrefix(k, ":"), url.PathEscape(v))
}

// Write writes data to web browser
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

// SetTotalCountHeader set "X-Total-Count" header
func (ctx *Context) SetTotalCountHeader(total int64) {
	ctx.RespHeader().Set("X-Total-Count", fmt.Sprint(total))
	ctx.AppendAccessControlExposeHeaders("X-Total-Count")
}

// AppendAccessControlExposeHeaders append headers by name to "Access-Control-Expose-Headers" header
func (ctx *Context) AppendAccessControlExposeHeaders(names ...string) {
	val := ctx.RespHeader().Get("Access-Control-Expose-Headers")
	if len(val) != 0 {
		ctx.RespHeader().Set("Access-Control-Expose-Headers", fmt.Sprintf("%s, %s", val, strings.Join(names, ", ")))
	} else {
		ctx.RespHeader().Set("Access-Control-Expose-Headers", strings.Join(names, ", "))
	}
}

// Handler represents a custom handler
type Handler func(*Context)

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

// GetContextUser returns context user
func GetContextUser(req *http.Request) *user_model.User {
	if apiContext, ok := req.Context().Value(apiContextKey).(*APIContext); ok {
		return apiContext.Doer
	}
	if ctx, ok := req.Context().Value(contextKey).(*Context); ok {
		return ctx.Doer
	}
	return nil
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
		SameSite:       setting.SessionConfig.SameSite,
	}
}

// Contexter initializes a classic context for a request.
func Contexter(ctx context.Context) func(next http.Handler) http.Handler {
	_, rnd := templates.HTMLRenderer(ctx)
	csrfOpts := getCsrfOpts()
	if !setting.IsProd {
		CsrfTokenRegenerationInterval = 5 * time.Second // in dev, re-generate the tokens more aggressively for debug purpose
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			locale := middleware.Locale(resp, req)
			startTime := time.Now()
			link := setting.AppSubURL + strings.TrimSuffix(req.URL.EscapedPath(), "/")

			ctx := Context{
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
					"Link":          link,
					"RunModeIsProd": setting.IsProd,
				},
			}
			defer ctx.Close()

			// PageData is passed by reference, and it will be rendered to `window.config.pageData` in `head.tmpl` for JavaScript modules
			ctx.PageData = map[string]interface{}{}
			ctx.Data["PageData"] = ctx.PageData
			ctx.Data["Context"] = &ctx

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
			ctx.Data["IsLandingPageHome"] = setting.LandingPageURL == setting.LandingPageHome
			ctx.Data["IsLandingPageExplore"] = setting.LandingPageURL == setting.LandingPageExplore
			ctx.Data["IsLandingPageOrganizations"] = setting.LandingPageURL == setting.LandingPageOrganizations

			ctx.Data["ShowRegistrationButton"] = setting.Service.ShowRegistrationButton
			ctx.Data["ShowMilestonesDashboardPage"] = setting.Service.ShowMilestonesDashboardPage
			ctx.Data["ShowFooterVersion"] = setting.Other.ShowFooterVersion

			ctx.Data["EnableSwagger"] = setting.API.EnableSwagger
			ctx.Data["EnableOpenIDSignIn"] = setting.Service.EnableOpenIDSignIn
			ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations
			ctx.Data["DisableStars"] = setting.Repository.DisableStars
			ctx.Data["EnableActions"] = setting.Actions.Enabled

			ctx.Data["ManifestData"] = setting.ManifestData

			ctx.Data["UnitWikiGlobalDisabled"] = unit.TypeWiki.UnitGlobalDisabled()
			ctx.Data["UnitIssuesGlobalDisabled"] = unit.TypeIssues.UnitGlobalDisabled()
			ctx.Data["UnitPullsGlobalDisabled"] = unit.TypePullRequests.UnitGlobalDisabled()
			ctx.Data["UnitProjectsGlobalDisabled"] = unit.TypeProjects.UnitGlobalDisabled()
			ctx.Data["UnitActionsGlobalDisabled"] = unit.TypeActions.UnitGlobalDisabled()

			ctx.Data["locale"] = locale
			ctx.Data["AllLangs"] = translation.AllLangs()

			next.ServeHTTP(ctx.Resp, ctx.Req)

			// Handle adding signedUserName to the context for the AccessLogger
			usernameInterface := ctx.Data["SignedUserName"]
			identityPtrInterface := ctx.Req.Context().Value(signedUserNameStringPointerKey)
			if usernameInterface != nil && identityPtrInterface != nil {
				username := usernameInterface.(string)
				identityPtr := identityPtrInterface.(*string)
				if identityPtr != nil && username != "" {
					*identityPtr = username
				}
			}
		})
	}
}

// SearchOrderByMap represents all possible search order
var SearchOrderByMap = map[string]map[string]db.SearchOrderBy{
	"asc": {
		"alpha":   db.SearchOrderByAlphabetically,
		"created": db.SearchOrderByOldest,
		"updated": db.SearchOrderByLeastUpdated,
		"size":    db.SearchOrderBySize,
		"id":      db.SearchOrderByID,
	},
	"desc": {
		"alpha":   db.SearchOrderByAlphabeticallyReverse,
		"created": db.SearchOrderByNewest,
		"updated": db.SearchOrderByRecentUpdated,
		"size":    db.SearchOrderBySizeReverse,
		"id":      db.SearchOrderByIDReverse,
	},
}
