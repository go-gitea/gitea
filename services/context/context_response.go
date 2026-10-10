// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/structs"
	"gitea.dev/modules/templates"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web/middleware"
)

// RedirectToUser redirect to a differently-named user
func RedirectToUser(ctx *Base, doer *user_model.User, userName string, redirectUserID int64) {
	user, err := user_model.GetUserByID(ctx, redirectUserID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			ctx.HTTPError(http.StatusNotFound, "user does not exist")
		} else {
			ctx.HTTPError(http.StatusInternalServerError, "unable to get user")
		}
		return
	}

	// Handle Visibility
	if user.Visibility != structs.VisibleTypePublic && doer == nil {
		// We must be signed in to see limited or private organizations
		ctx.HTTPError(http.StatusNotFound, "user does not exist")
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
	if err == nil || httplib.IsClientOrNetworkError(ctx, err) {
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

// RenderToHTML renders the template content to a HTML string
func (ctx *Context) RenderToHTML(name templates.TplName, data any) (template.HTML, error) {
	var buf strings.Builder
	err := ctx.Render.HTML(&buf, 0, name, data, ctx.TemplateContext)
	return template.HTML(buf.String()), err
}

// RenderWithErrDeprecated render the page with form validation when it needs to prompt error to users.
// Deprecated: use "form-fetch-action" and JSON response instead.
// WARNING: in many cases, this function is not able to render the page or recover the form fields correctly.
// And it is very difficult to test the page rendered by this function.
// DO NOT USE IT ANYMORE.
func (ctx *Context) RenderWithErrDeprecated(msg any, tpl templates.TplName, form any) {
	if form != nil {
		middleware.AssignForm(form, ctx.Data)
	}
	ctx.Flash.Error(msg, true)
	ctx.HTML(http.StatusOK, tpl)
}

// NotFound displays a 404 (Not Found) page and prints the given error, if any.
func (ctx *Context) NotFound(logErr error) {
	ctx.notFoundInternal(1, "", logErr)
}

func (ctx *Context) notFoundInternal(skip int, logMsg string, logErr error) {
	// TODO: it's safe to show the error message to end users if the error is fully controlled by our error system
	if logErr != nil {
		log.Log(skip+1, log.DEBUG, "%s: %v", logMsg, logErr)
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
	ctx.Data["ErrorMsg"] = "" // FIXME: the template never renders this message, need to fix in the future (and show safe messages to end users)
	ctx.HTML(http.StatusNotFound, "status/404")
}

func (ctx *Context) buildUserErrorMessage(msg string, err error) (userErrorMsg string) {
	// it's safe to show internal error to admin users, and it helps
	if !setting.IsProd || setting.IsInTesting || (ctx.Doer != nil && ctx.Doer.IsAdmin) {
		userErrorMsg = msg
		if err != nil {
			userErrorMsg += ", error: " + err.Error()
		}
	}
	return util.IfZero(userErrorMsg, ctx.Locale.TrString("error.occurred"))
}

// ServerError displays a 500 (Internal Server Error) page and prints the given error, if any.
// If the error is controlled by our error system, a related 404 page can be displayed instead.
func (ctx *Context) ServerError(logMsg string, logErr error) {
	if errors.Is(logErr, util.ErrNotExist) {
		ctx.notFoundInternal(1, logMsg, logErr)
		return
	}
	ctx.serverErrorInternal(1, logMsg, logErr)
}

func (ctx *Context) serverErrorInternal(skip int, logMsg string, logErr error) {
	if logErr != nil {
		logLevel := util.Iif(httplib.IsClientOrNetworkError(ctx, logErr), log.DEBUG, log.ERROR)
		log.Log(skip+1, logLevel, "%s: %v", logMsg, logErr)
	}

	userErrorMsg := ctx.buildUserErrorMessage(logMsg, logErr)
	if httplib.IsGiteaFetchActionRequest(ctx.Req) {
		ctx.JSON(http.StatusInternalServerError, buildJsonErrorMap(userErrorMsg))
		return
	}

	ctx.Data["Title"] = "Internal Server Error"
	ctx.Data["ErrorMsg"] = userErrorMsg
	ctx.HTML(http.StatusInternalServerError, tplStatus500)
}

// NotFoundOrServerError use error check function to determine if the error
// is about not found. It responds with 404 status code for not found error,
// or error context description for logging purpose of 500 server error.
// TODO: remove the "errCheck" and use util.ErrNotFound to check
func (ctx *Context) NotFoundOrServerError(logMsg string, errCheck func(error) bool, logErr error) {
	if errCheck(logErr) {
		ctx.notFoundInternal(1, logMsg, logErr)
		return
	}
	ctx.serverErrorInternal(1, logMsg, logErr)
}
