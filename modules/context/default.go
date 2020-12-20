// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/translation"

	"gitea.com/go-chi/binding"
	"gitea.com/go-chi/session"
	"github.com/unrolled/render"
)

// flashes enumerates all the flash types
const (
	SuccessFlash = "SuccessMsg"
	ErrorFlash   = "ErrorMsg"
	WarnFlash    = "WarningMsg"
	InfoFlash    = "InfoMsg"
)

// Flash represents flashs
type Flash map[string]string

// DefaultContext represents a context for basic routes, all other context should
// be derived from the context but not add more fields on this context
type DefaultContext struct {
	Resp    http.ResponseWriter
	Req     *http.Request
	Data    map[string]interface{}
	Render  *render.Render
	Session session.Store
	translation.Locale
	flash Flash
}

// HTML wraps render HTML
func (ctx *DefaultContext) HTML(statusCode int, tmpl string) error {
	return ctx.Render.HTML(ctx.Resp, statusCode, tmpl, ctx.Data)
}

// Bind binding a form to a struct
func (ctx *DefaultContext) Bind(form interface{}) binding.Errors {
	return binding.Bind(ctx.Req, form)
}

// HasError returns true if error occurs in form validation.
func (ctx *DefaultContext) HasError() bool {
	hasErr, ok := ctx.Data["HasError"]
	if !ok {
		return false
	}
	return hasErr.(bool)
}

// HasValue returns true if value of given name exists.
func (ctx *DefaultContext) HasValue(name string) bool {
	_, ok := ctx.Data[name]
	return ok
}

// RenderWithErr used for page has form validation but need to prompt error to users.
func (ctx *DefaultContext) RenderWithErr(msg string, tpl string, form interface{}) {
	if form != nil {
		auth.AssignForm(form, ctx.Data)
	}
	ctx.Flash(ErrorFlash, msg)
	_ = ctx.HTML(200, tpl)
}

// SetSession sets session key value
func (ctx *DefaultContext) SetSession(key string, val interface{}) error {
	return ctx.Session.Set(key, val)
}

// GetSession gets session via key
func (ctx *DefaultContext) GetSession(key string) (interface{}, error) {
	v := ctx.Session.Get(key)
	return v, nil
}

// DestroySession deletes all the data of the session
func (ctx *DefaultContext) DestroySession() error {
	return ctx.Session.Release()
}

// Flash set message to flash
func (ctx *DefaultContext) Flash(tp, v string) {
	if ctx.flash == nil {
		ctx.flash = make(Flash)
	}
	ctx.flash[tp] = v
	ctx.Data[tp] = v
	ctx.Data["Flash"] = ctx.flash
}

var (
	defaultContextKey interface{} = "default_context"
)

// WithDefaultContext set up install context in request
func WithDefaultContext(req *http.Request, ctx *DefaultContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), defaultContextKey, ctx))
}

// GetDefaultContext retrieves install context from request
func GetDefaultContext(req *http.Request) *DefaultContext {
	return req.Context().Value(defaultContextKey).(*DefaultContext)
}
