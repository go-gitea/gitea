// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"
)

// RedirectToUser redirect to a differently-named user
func RedirectToUser(ctx *Base, userName string, redirectUserID int64) {
	user, err := user_model.GetUserByID(ctx, redirectUserID)
	if err != nil {
		ctx.HTTPError(http.StatusInternalServerError, "unable to get user")
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

// RedirectToCurrentSite redirects to first not empty URL which belongs to current site
func (ctx *Context) RedirectToCurrentSite(location ...string) {
	for _, loc := range location {
		if len(loc) == 0 {
			continue
		}

		if !httplib.IsCurrentGiteaSiteURL(ctx, loc) {
			continue
		}

		ctx.Redirect(loc)
		return
	}

	ctx.Redirect(setting.AppSubURL + "/")
}

const tplStatus500 templates.TplName = "status/500"

// HTML calls Context.HTML and renders the template to HTTP response
func (ctx *Context) HTML(status int, name templates.TplName) {
	log.Debug("Template: %s", name)

	tmplStartTime := time.Now()
	if !setting.IsProd {
		ctx.Data["TemplateName"] = name
	}
	ctx.Data["TemplateLoadTimes"] = func() string {
		return strconv.FormatInt(time.Since(tmplStartTime).Nanoseconds()/1e6, 10) + "ms"
	}

	err := ctx.Render.HTML(ctx.Resp, status, name, ctx.Data, ctx.TemplateContext)
	if err == nil || errors.Is(err, syscall.EPIPE) {
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

// JSONTemplate renders the template as JSON response
// keep in mind that the template is processed in HTML context, so JSON-things should be handled carefully, eg: by JSEscape
func (ctx *Context) JSONTemplate(tmpl templates.TplName) {
	t, err := ctx.Render.TemplateLookup(string(tmpl), nil)
	if err != nil {
		ctx.ServerError("unable to find template", err)
		return
	}
	ctx.Resp.Header().Set("Content-Type", "application/json")
	if err = t.Execute(ctx.Resp, ctx.Data); err != nil {
		ctx.ServerError("unable to execute template", err)
	}
}

// RenderToHTML renders the template content to a HTML string
func (ctx *Context) RenderToHTML(name templates.TplName, data any) (template.HTML, error) {
	var buf strings.Builder
	err := ctx.Render.HTML(&buf, 0, name, data, ctx.TemplateContext)
	return template.HTML(buf.String()), err
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *Context) RenderWithErr(msg any, tpl templates.TplName, form any) {
	if form != nil {
		middleware.AssignForm(form, ctx.Data)
	}
	ctx.Flash.Error(msg, true)
	ctx.HTML(http.StatusOK, tpl)
}

// NotFound displays a 404 (Not Found) page and prints the given error, if any.
func (ctx *Context) NotFound(logErr error) {
	ctx.notFoundInternal("", logErr)
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
	ctx.HTML(http.StatusNotFound, templates.TplName("status/404"))
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
// TODO: remove the "errCheck" and use util.ErrNotFound to check
func (ctx *Context) NotFoundOrServerError(logMsg string, errCheck func(error) bool, logErr error) {
	if errCheck(logErr) {
		ctx.notFoundInternal(logMsg, logErr)
		return
	}
	ctx.serverErrorInternal(logMsg, logErr)
}
