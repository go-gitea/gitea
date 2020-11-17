// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"

	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/translation"

	"github.com/alexedwards/scs/v2"
	"github.com/unrolled/render"
)

// flashes enumerates all the flash types
const (
	SuccessFlash = "success"
	ErrorFlash   = "error"
	WarnFlash    = "warning"
	InfoFlash    = "info"
)

// Flash represents flashs
type Flash struct {
	MessageType string
	Message     string
}

// DefaultContext represents a context for basic routes, all other context should
// be derived from the context but not add more fields on this context
type DefaultContext struct {
	Resp     http.ResponseWriter
	Req      *http.Request
	Data     map[string]interface{}
	Render   *render.Render
	Sessions *scs.SessionManager
	translation.Locale
	flash *Flash
}

// HTML wraps render HTML
func (ctx *DefaultContext) HTML(statusCode int, tmpl string) error {
	return ctx.Render.HTML(ctx.Resp, statusCode, tmpl, ctx.Data)
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
	ctx.Sessions.Put(ctx.Req.Context(), key, val)
	return nil
}

// GetSession gets session via key
func (ctx *DefaultContext) GetSession(key string) (interface{}, error) {
	v := ctx.Sessions.Get(ctx.Req.Context(), key)
	return v, nil
}

// DestroySession deletes all the data of the session
func (ctx *DefaultContext) DestroySession() error {
	return ctx.Sessions.Destroy(ctx.Req.Context())
}

// Flash set message to flash
func (ctx *DefaultContext) Flash(tp, v string) {
	if ctx.flash == nil {
		ctx.flash = &Flash{}
	}
	ctx.flash.MessageType = tp
	ctx.flash.Message = v
	ctx.Data["Flash"] = ctx.flash
}

// NewCookie creates a cookie
func NewCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Path:     setting.SessionConfig.CookiePath,
		Domain:   setting.SessionConfig.Domain,
		MaxAge:   maxAge,
		Secure:   setting.SessionConfig.Secure,
	}
}
