// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/reqctx"
)

// Flash represents a one time data transfer between two requests.
type Flash struct {
	DataStore reqctx.RequestDataStore
	url.Values
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string, current ...bool) {
	if f.Values == nil {
		f.Values = make(map[string][]string)
	}
	showInCurrentPage := len(current) > 0 && current[0]
	if showInCurrentPage {
		// assign it to the context data, then the template can use ".Flash.XxxMsg" to render the message
		f.DataStore.GetData()["Flash"] = f
	} else {
		// the message map will be saved into the cookie and be shown in next response (a new page response which decodes the cookie)
		f.Set(name, msg)
	}
}

func flashMsgStringOrHTML(msg any) string {
	switch v := msg.(type) {
	case string:
		return v
	case template.HTML:
		return string(v)
	}
	panic(fmt.Sprintf("unknown type: %T", msg))
}

// Error sets error message
func (f *Flash) Error(msg any, current ...bool) {
	f.ErrorMsg = flashMsgStringOrHTML(msg)
	f.set("error", f.ErrorMsg, current...)
}

// Warning sets warning message
func (f *Flash) Warning(msg any, current ...bool) {
	f.WarningMsg = flashMsgStringOrHTML(msg)
	f.set("warning", f.WarningMsg, current...)
}

// Info sets info message
func (f *Flash) Info(msg any, current ...bool) {
	f.InfoMsg = flashMsgStringOrHTML(msg)
	f.set("info", f.InfoMsg, current...)
}

// Success sets success message
func (f *Flash) Success(msg any, current ...bool) {
	f.SuccessMsg = flashMsgStringOrHTML(msg)
	f.set("success", f.SuccessMsg, current...)
}

func ParseCookieFlashMessage(val string) *Flash {
	if vals, _ := url.ParseQuery(val); len(vals) > 0 {
		return &Flash{
			Values:     vals,
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
