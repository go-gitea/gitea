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
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
)

// RedirectToUser redirect to a differently-named user
func RedirectToUser(ctx *Base, userName string, redirectUserID int64) {
	user, err := user_model.GetUserByID(ctx, redirectUserID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "unable to get user")
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

		// Unfortunately browsers consider a redirect Location with preceding "//", "\\" and "/\" as meaning redirect to "http(s)://REST_OF_PATH"
		// Therefore we should ignore these redirect locations to prevent open redirects
		if len(loc) > 1 && (loc[0] == '/' || loc[0] == '\\') && (loc[1] == '/' || loc[1] == '\\') {
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
func (ctx *Context) RenderToString(name base.TplName, data map[string]any) (string, error) {
	var buf strings.Builder
	err := ctx.Render.HTML(&buf, http.StatusOK, string(name), data)
	return buf.String(), err
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *Context) RenderWithErr(msg string, tpl base.TplName, form any) {
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
