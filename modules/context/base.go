// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/go-chi/chi/v5"
)

type contextValuePair struct {
	key     any
	valueFn func() any
}

type Base struct {
	originCtx     context.Context
	contextValues []contextValuePair

	Resp ResponseWriter
	Req  *http.Request

	// Data is prepared by ContextDataStore middleware, this field only refers to the pre-created/prepared ContextData.
	// Although it's mainly used for MVC templates, sometimes it's also used to pass data between middlewares/handler
	Data middleware.ContextData

	// Locale is mainly for Web context, although the API context also uses it in some cases: message response, form validation
	Locale translation.Locale
}

func (b *Base) Deadline() (deadline time.Time, ok bool) {
	return b.originCtx.Deadline()
}

func (b *Base) Done() <-chan struct{} {
	return b.originCtx.Done()
}

func (b *Base) Err() error {
	return b.originCtx.Err()
}

func (b *Base) Value(key any) any {
	for _, pair := range b.contextValues {
		if pair.key == key {
			return pair.valueFn()
		}
	}
	return b.originCtx.Value(key)
}

func (b *Base) AppendContextValueFunc(key any, valueFn func() any) any {
	b.contextValues = append(b.contextValues, contextValuePair{key, valueFn})
	return b
}

func (b *Base) AppendContextValue(key, value any) any {
	b.contextValues = append(b.contextValues, contextValuePair{key, func() any { return value }})
	return b
}

func (b *Base) GetData() middleware.ContextData {
	return b.Data
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

// Error returned an error to web browser
func (b *Base) Error(status int, contents ...string) {
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

func (b *Base) JSONRedirect(redirect string) {
	b.JSON(http.StatusOK, map[string]any{"redirect": redirect})
}

func (b *Base) JSONOK() {
	b.JSON(http.StatusOK, map[string]any{"ok": true}) // this is only a dummy response, frontend seldom uses it
}

func (b *Base) JSONError(msg string) {
	b.JSON(http.StatusBadRequest, map[string]any{"errorMessage": msg})
}

// RemoteAddr returns the client machine ip address
func (b *Base) RemoteAddr() string {
	return b.Req.RemoteAddr
}

// Params returns the param on route
func (b *Base) Params(p string) string {
	s, _ := url.PathUnescape(chi.URLParam(b.Req, strings.TrimPrefix(p, ":")))
	return s
}

// ParamsInt64 returns the param on route as int64
func (b *Base) ParamsInt64(p string) int64 {
	v, _ := strconv.ParseInt(b.Params(p), 10, 64)
	return v
}

// SetParams set params into routes
func (b *Base) SetParams(k, v string) {
	chiCtx := chi.RouteContext(b)
	chiCtx.URLParams.Add(strings.TrimPrefix(k, ":"), url.PathEscape(v))
}

// FormString returns the first value matching the provided key in the form as a string
func (b *Base) FormString(key string) string {
	return b.Req.FormValue(key)
}

// FormStrings returns a string slice for the provided key from the form
func (b *Base) FormStrings(key string) []string {
	if b.Req.Form == nil {
		if err := b.Req.ParseMultipartForm(32 << 20); err != nil {
			return nil
		}
	}
	if v, ok := b.Req.Form[key]; ok {
		return v
	}
	return nil
}

// FormTrim returns the first value for the provided key in the form as a space trimmed string
func (b *Base) FormTrim(key string) string {
	return strings.TrimSpace(b.Req.FormValue(key))
}

// FormInt returns the first value for the provided key in the form as an int
func (b *Base) FormInt(key string) int {
	v, _ := strconv.Atoi(b.Req.FormValue(key))
	return v
}

// FormInt64 returns the first value for the provided key in the form as an int64
func (b *Base) FormInt64(key string) int64 {
	v, _ := strconv.ParseInt(b.Req.FormValue(key), 10, 64)
	return v
}

// FormBool returns true if the value for the provided key in the form is "1", "true" or "on"
func (b *Base) FormBool(key string) bool {
	s := b.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return v
}

// FormOptionalBool returns an OptionalBoolTrue or OptionalBoolFalse if the value
// for the provided key exists in the form else it returns OptionalBoolNone
func (b *Base) FormOptionalBool(key string) util.OptionalBool {
	value := b.Req.FormValue(key)
	if len(value) == 0 {
		return util.OptionalBoolNone
	}
	s := b.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return util.OptionalBoolOf(v)
}

func (b *Base) SetFormString(key, value string) {
	_ = b.Req.FormValue(key) // force parse form
	b.Req.Form.Set(key, value)
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
	if _, err := b.Resp.Write(bs); err != nil {
		log.ErrorWithSkip(skip, "plainTextInternal (status=%d): write bytes failed: %v", status, err)
	}
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

	if strings.Contains(location, "://") || strings.HasPrefix(location, "//") {
		// Some browsers (Safari) have buggy behavior for Cookie + Cache + External Redirection, eg: /my-path => https://other/path
		// 1. the first request to "/my-path" contains cookie
		// 2. some time later, the request to "/my-path" doesn't contain cookie (caused by Prevent web tracking)
		// 3. Gitea's Sessioner doesn't see the session cookie, so it generates a new session id, and returns it to browser
		// 4. then the browser accepts the empty session, then the user is logged out
		// So in this case, we should remove the session cookie from the response header
		removeSessionCookieHeader(b.Resp)
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

// Close frees all resources hold by Context
func (b *Base) cleanUp() {
	if b.Req != nil && b.Req.MultipartForm != nil {
		_ = b.Req.MultipartForm.RemoveAll() // remove the temp files buffered to tmp directory
	}
}

func (b *Base) Tr(msg string, args ...any) string {
	return b.Locale.Tr(msg, args...)
}

func (b *Base) TrN(cnt any, key1, keyN string, args ...any) string {
	return b.Locale.TrN(cnt, key1, keyN, args...)
}

func NewBaseContext(resp http.ResponseWriter, req *http.Request) (b *Base, closeFunc func()) {
	b = &Base{
		originCtx: req.Context(),
		Req:       req,
		Resp:      WrapResponseWriter(resp),
		Locale:    middleware.Locale(resp, req),
		Data:      middleware.GetContextData(req.Context()),
	}
	b.AppendContextValue(translation.ContextKey, b.Locale)
	b.Req = b.Req.WithContext(b)
	return b, b.cleanUp
}
