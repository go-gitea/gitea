// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
)

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

// Written returns true if there are something sent to web browser
func (ctx *Context) Written() bool {
	return ctx.Resp.Status() > 0
}

// Status writes status code
func (ctx *Context) Status(status int) {
	ctx.Resp.WriteHeader(status)
}

// Write writes data to web browser
func (ctx *Context) Write(bs []byte) (int, error) {
	return ctx.Resp.Write(bs)
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

	err := ctx.Render.HTML(ctx.Resp, status, string(name), ctx.Data)
	if err == nil {
		return
	}

	// if rendering fails, show error page
	if name != tplStatus500 {
		err = fmt.Errorf("failed to render template: %s, error: %s", name, templates.HandleTemplateRenderingError(err))
		ctx.ServerError("Render failed", err) // show the 500 error page
	} else {
		ctx.PlainText(http.StatusInternalServerError, "Unable to render status/500 page, the template system is broken, or Gitea can't find your template files.")
		return
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
