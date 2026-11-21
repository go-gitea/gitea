// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html"
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
	if user.Visibility != structs.VisibleTypePublic && !user_model.DoerCanViewPrivate(doer) {
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
		ctx.ServerErrorAuto(err) // show the 500 error page
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
func (ctx *Context) RenderWithErrDeprecated[T string | template.HTML](msg T, tpl templates.TplName, form any) {
	if form != nil {
		middleware.AssignForm(form, ctx.Data)
	}
	ctx.Flash.Error(msg, true)
	ctx.HTML(http.StatusOK, tpl)
}

// NotFound displays a 404 (Not Found) page.
func (ctx *Context) NotFound() {
	ctx.renderError4xx(1, http.StatusNotFound, "")
}

func (ctx *Context) renderError4xx(skip, status int, msg string) {
	if !httplib.ClientAcceptHTML(ctx.Req) {
		msg = util.IfZero(msg, http.StatusText(status))
		ctx.plainTextInternal(skip+1, status, []byte(msg))
		return
	}

	ctx.Data["IsRepo"] = ctx.Repo.Repository != nil
	ctx.Data["Title"] = http.StatusText(status)
	ctx.Data["ErrorCodeStatus"] = status
	ctx.Data["ErrorCodeText"] = http.StatusText(status)
	ctx.Data["ErrorMsg"] = "" // TODO: USER-ERROR-HTML-DISPLAY: the template never renders this message, need to fix in the future (and show safe messages to end users)
	ctx.HTML(status, "status/4xx")
}

func (ctx *Context) buildServerErrorMessageForUser(msg string, err error) (userErrorMsg string) {
	// it's safe to show internal error to admin users, and it helps
	if !setting.IsProd || setting.IsInTesting || (ctx.Doer != nil && ctx.Doer.IsAdmin) {
		userErrorMsg = msg
		if err != nil {
			userErrorMsg += ", error: " + err.Error()
		}
	}
	return util.IfZero(userErrorMsg, ctx.Locale.TrString("error.occurred"))
}

func (ctx *Context) FlashErrorAuto(err error) {
	errMsg, _ := util.ErrorUnwrapForHTML(err, ctx.Locale)
	if errMsg != "" {
		ctx.Flash.Error(errMsg)
		return
	}

	logLevel := util.Iif(httplib.IsClientOrNetworkError(ctx, err), log.DEBUG, log.ERROR)
	log.Log(1, logLevel, "FlashErrorAuto: server internal error: %v", err)

	userErrorMsg := ctx.buildServerErrorMessageForUser("internal server error", err)
	ctx.Flash.Error(html.EscapeString(userErrorMsg)) // flash renders HTML with sanitization, so escape the message to keep original content
}

func (ctx *Context) JSONRedirect(redirect string) {
	ctx.JSON(http.StatusOK, map[string]any{"redirect": redirect})
}

func (ctx *Context) JSONOK() {
	ctx.JSON(http.StatusOK, map[string]any{"ok": true}) // this is only a dummy response, frontend seldom uses it
}

func buildJsonErrorMap[T string | template.HTML](msg T) map[string]any {
	switch v := any(msg).(type) {
	case string:
		return map[string]any{"errorMessage": v, "renderFormat": "text"}
	case template.HTML:
		return map[string]any{"errorMessage": v, "renderFormat": "html"}
	}
	panic(fmt.Sprintf("unsupported type: %T", msg))
}

func (ctx *Context) JSONErrorAuto(err error) {
	errMsg, httpCode := util.ErrorUnwrapForHTML(err, ctx.Locale)
	if errMsg != "" {
		ctx.JSON(httpCode, buildJsonErrorMap(errMsg))
		return
	}

	logLevel := util.Iif(httplib.IsClientOrNetworkError(ctx, err), log.DEBUG, log.ERROR)
	log.Log(1, logLevel, "JSONErrorAuto: server internal error: %v", err)

	userErrorMsg := ctx.buildServerErrorMessageForUser("internal server error", err)
	ctx.JSON(http.StatusInternalServerError, buildJsonErrorMap(userErrorMsg))
}

func (ctx *Context) JSONError[T string | template.HTML](msg T) {
	ctx.JSON(http.StatusBadRequest, buildJsonErrorMap(msg))
}

func (ctx *Context) JSONErrorWithField[T string | template.HTML](msg T, field string) {
	m := buildJsonErrorMap(msg)
	m["errorFields"] = []string{field}
	ctx.JSON(http.StatusBadRequest, m)
}

func (ctx *Context) JSONErrorNotFound(optMsg ...string) {
	msg := util.OptionalArg(optMsg)
	if msg == "" {
		msg = ctx.Locale.TrString("error.not_found")
	}
	ctx.JSON(http.StatusNotFound, buildJsonErrorMap(msg))
}

// ServerErrorAuto handles the error automatically
//   - If the error is controlled by our error system, display a 4xx page.
//     TODO: USER-ERROR-HTML-DISPLAY: To keep the same behavior as v28, HTML page still doesn't render the error message at the moment.
//   - Otherwise, display a 500 (Internal Server Error) page and log the given error
func (ctx *Context) ServerErrorAuto(err error) {
	if errMsg, httpCode := util.ErrorUnwrapForUser(err, ctx.Locale); errMsg != "" {
		log.Log(1, log.DEBUG, "ServerErrorAuto(for user): status=%d, %v", httpCode, err)
		ctx.renderError4xx(1, httpCode, errMsg)
		return
	}

	logLevel := util.Iif(httplib.IsClientOrNetworkError(ctx, err), log.DEBUG, log.ERROR)
	log.Log(1, logLevel, "ServerErrorAuto: %v", err)
	userErrorMsg := ctx.buildServerErrorMessageForUser("internal server error", err)
	if httplib.IsGiteaFetchActionRequest(ctx.Req) {
		ctx.JSON(http.StatusInternalServerError, buildJsonErrorMap(userErrorMsg))
		return
	}

	ctx.Data["Title"] = "Internal Server Error"
	ctx.Data["ErrorMsg"] = userErrorMsg
	ctx.HTML(http.StatusInternalServerError, tplStatus500)
}
