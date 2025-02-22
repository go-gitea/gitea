// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/reqctx"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web/middleware"
)

type BaseContextKeyType struct{}

var BaseContextKey BaseContextKeyType

// Base is the base context for all web handlers
// ATTENTION: This struct should never be manually constructed in routes/services,
// it has many internal details which should be carefully prepared by the framework.
// If it is abused, it would cause strange bugs like panic/resource-leak.
type Base struct {
	reqctx.RequestContext

	Resp ResponseWriter
	Req  *http.Request

	// Data is prepared by ContextDataStore middleware, this field only refers to the pre-created/prepared ContextData.
	// Although it's mainly used for MVC templates, sometimes it's also used to pass data between middlewares/handler
	Data reqctx.ContextData

	// Locale is mainly for Web context, although the API context also uses it in some cases: message response, form validation
	Locale translation.Locale
}

// AppendAccessControlExposeHeaders append headers by name to "Access-Control-Expose-Headers" header
func (b *Base) AppendAccessControlExposeHeaders(names ...string) {
	val := b.RespHeader().Get("Access-Control-Expose-Headers")
	if len(val) != 0 {
		b.RespHeader().Set("Access-Control-Expose-Headers", fmt.Sprintf("%s, %s", val, strings.Join(names, ", ")))
	} else {
		b.RespHeader().Set("Access-Control-Expose-Headers", strings.Join(names, ", "))
	}
}

// SetTotalCountHeader set "X-Total-Count" header
func (b *Base) SetTotalCountHeader(total int64) {
	b.RespHeader().Set("X-Total-Count", fmt.Sprint(total))
	b.AppendAccessControlExposeHeaders("X-Total-Count")
}

// Written returns true if there are something sent to web browser
func (b *Base) Written() bool {
	return b.Resp.WrittenStatus() != 0
}

func (b *Base) WrittenStatus() int {
	return b.Resp.WrittenStatus()
}

// Status writes status code
func (b *Base) Status(status int) {
	b.Resp.WriteHeader(status)
}

// Write writes data to web browser
func (b *Base) Write(bs []byte) (int, error) {
	return b.Resp.Write(bs)
}

// RespHeader returns the response header
func (b *Base) RespHeader() http.Header {
	return b.Resp.Header()
}

// HTTPError returned an error to web browser
func (b *Base) HTTPError(status int, contents ...string) {
	v := http.StatusText(status)
	if len(contents) > 0 {
		v = contents[0]
	}
	http.Error(b.Resp, v, status)
}

// JSON render content as JSON
func (b *Base) JSON(status int, content any) {
	b.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	b.Resp.WriteHeader(status)
	if err := json.NewEncoder(b.Resp).Encode(content); err != nil {
		log.Error("Render JSON failed: %v", err)
	}
}

// RemoteAddr returns the client machine ip address
func (b *Base) RemoteAddr() string {
	return b.Req.RemoteAddr
}

// PlainTextBytes renders bytes as plain text
func (b *Base) plainTextInternal(skip, status int, bs []byte) {
	statusPrefix := status / 100
	if statusPrefix == 4 || statusPrefix == 5 {
		log.Log(skip, log.TRACE, "plainTextInternal (status=%d): %s", status, string(bs))
	}
	b.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	b.Resp.Header().Set("X-Content-Type-Options", "nosniff")
	b.Resp.WriteHeader(status)
	_, _ = b.Resp.Write(bs)
}

// PlainTextBytes renders bytes as plain text
func (b *Base) PlainTextBytes(status int, bs []byte) {
	b.plainTextInternal(2, status, bs)
}

// PlainText renders content as plain text
func (b *Base) PlainText(status int, text string) {
	b.plainTextInternal(2, status, []byte(text))
}

// Redirect redirects the request
func (b *Base) Redirect(location string, status ...int) {
	code := http.StatusSeeOther
	if len(status) == 1 {
		code = status[0]
	}

	if !httplib.IsRelativeURL(location) {
		// Some browsers (Safari) have buggy behavior for Cookie + Cache + External Redirection, eg: /my-path => https://other/path
		// 1. the first request to "/my-path" contains cookie
		// 2. some time later, the request to "/my-path" doesn't contain cookie (caused by Prevent web tracking)
		// 3. Gitea's Sessioner doesn't see the session cookie, so it generates a new session id, and returns it to browser
		// 4. then the browser accepts the empty session, then the user is logged out
		// So in this case, we should remove the session cookie from the response header
		removeSessionCookieHeader(b.Resp)
	}
	// in case the request is made by htmx, have it redirect the browser instead of trying to follow the redirect inside htmx
	if b.Req.Header.Get("HX-Request") == "true" {
		b.Resp.Header().Set("HX-Redirect", location)
		// we have to return a non-redirect status code so XMLHTTPRequest will not immediately follow the redirect
		// so as to give htmx redirect logic a chance to run
		b.Status(http.StatusNoContent)
		return
	}
	http.Redirect(b.Resp, b.Req, location, code)
}

type ServeHeaderOptions httplib.ServeHeaderOptions

func (b *Base) SetServeHeaders(opt *ServeHeaderOptions) {
	httplib.ServeSetHeaders(b.Resp, (*httplib.ServeHeaderOptions)(opt))
}

// ServeContent serves content to http request
func (b *Base) ServeContent(r io.ReadSeeker, opts *ServeHeaderOptions) {
	httplib.ServeSetHeaders(b.Resp, (*httplib.ServeHeaderOptions)(opts))
	http.ServeContent(b.Resp, b.Req, opts.Filename, opts.LastModified, r)
}

func (b *Base) Tr(msg string, args ...any) template.HTML {
	return b.Locale.Tr(msg, args...)
}

func (b *Base) TrN(cnt any, key1, keyN string, args ...any) template.HTML {
	return b.Locale.TrN(cnt, key1, keyN, args...)
}

func NewBaseContext(resp http.ResponseWriter, req *http.Request) *Base {
	reqCtx := reqctx.FromContext(req.Context())
	b := &Base{
		RequestContext: reqCtx,

		Req:    req,
		Resp:   WrapResponseWriter(resp),
		Locale: middleware.Locale(resp, req),
		Data:   reqCtx.GetData(),
	}
	b.Req = b.Req.WithContext(b)
	reqCtx.SetContextValue(BaseContextKey, b)
	reqCtx.SetContextValue(translation.ContextKey, b.Locale)
	reqCtx.SetContextValue(httplib.RequestContextKey, b.Req)
	return b
}

func NewBaseContextForTest(resp http.ResponseWriter, req *http.Request) *Base {
	if !setting.IsInTesting {
		panic("This function is only for testing")
	}
	ctx := reqctx.NewRequestContextForTest(req.Context())
	*req = *req.WithContext(ctx)
	return NewBaseContext(resp, req)
}
