// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"html/template"
	"net/http"
	"net/url"

	"gitea.dev/modules/reqctx"
)

// Flash represents a one time data transfer between two requests.
type Flash struct {
	DataStore reqctx.RequestDataStore
	msgValues url.Values

	// message is in HTML format, and is sanitized when rendering (ref: RenderFlashMessage)
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string, current ...bool) {
	if f.msgValues == nil {
		f.msgValues = url.Values{}
	}
	showInCurrentPage := len(current) > 0 && current[0]
	if showInCurrentPage {
		// assign it to the context data, then the template can use ".Flash.XxxMsg" to render the message
		f.DataStore.GetData()["Flash"] = f
	} else {
		// the message map will be saved into the cookie and be shown in next response (a new page response which decodes the cookie)
		f.msgValues.Set(name, msg)
	}
}

func (f *Flash) EncodeToString() string {
	// TODO: modern browsers support 4096 bytes for a cookie, however, it's not easy to truncate the cookie string
	// because SetSiteCookie does another round of "QueryEscape"
	// So in edge cases, if the message is too long after double-escaped, the error message won't be shown
	return f.msgValues.Encode()
}

// Error sets error message
func (f *Flash) Error[T string | template.HTML](msg T, current ...bool) {
	f.ErrorMsg = string(msg)
	f.set("error", f.ErrorMsg, current...)
}

// Warning sets warning message
func (f *Flash) Warning[T string | template.HTML](msg T, current ...bool) {
	f.WarningMsg = string(msg)
	f.set("warning", f.WarningMsg, current...)
}

// Info sets info message
func (f *Flash) Info[T string | template.HTML](msg T, current ...bool) {
	f.InfoMsg = string(msg)
	f.set("info", f.InfoMsg, current...)
}

// Success sets success message
func (f *Flash) Success[T string | template.HTML](msg T, current ...bool) {
	f.SuccessMsg = string(msg)
	f.set("success", f.SuccessMsg, current...)
}

func ParseCookieFlashMessage(val string) *Flash {
	if vals, _ := url.ParseQuery(val); len(vals) > 0 {
		return &Flash{
			ErrorMsg:   vals.Get("error"),
			SuccessMsg: vals.Get("success"),
			InfoMsg:    vals.Get("info"),
			WarningMsg: vals.Get("warning"),
		}
	}
	return nil
}

func GetSiteCookieFlashMessage(dataStore reqctx.RequestDataStore, req *http.Request, cookieName string) (string, *Flash) {
	// Get the last flash message from cookie
	lastFlashCookie := GetSiteCookie(req, cookieName)
	lastFlashMsg := ParseCookieFlashMessage(lastFlashCookie)
	if lastFlashMsg != nil {
		lastFlashMsg.DataStore = dataStore
		return lastFlashCookie, lastFlashMsg
	}
	return lastFlashCookie, nil
}
