// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package middleware

import (
	"fmt"
	"net/url"
)

// Flash represents a one time data transfer between two requests.
type Flash struct {
	DataStore ContextDataStore
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

// Error sets error message
func (f *Flash) Error(msg any, current ...bool) {
	msgStr := fmt.Sprint(msg)
	f.ErrorMsg = msgStr
	f.set("error", msgStr, current...)
}

// Warning sets warning message
func (f *Flash) Warning(msg any, current ...bool) {
	msgStr := fmt.Sprint(msg)
	f.WarningMsg = msgStr
	f.set("warning", msgStr, current...)
}

// Info sets info message
func (f *Flash) Info(msg any, current ...bool) {
	msgStr := fmt.Sprint(msg)
	f.InfoMsg = msgStr
	f.set("info", msgStr, current...)
}

// Success sets success message
func (f *Flash) Success(msg any, current ...bool) {
	msgStr := fmt.Sprint(msg)
	f.SuccessMsg = msgStr
	f.set("success", msgStr, current...)
}
