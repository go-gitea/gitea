// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middlewares

import "net/url"

// flashes enumerates all the flash types
const (
	SuccessFlash = "SuccessMsg"
	ErrorFlash   = "ErrorMsg"
	WarnFlash    = "WarningMsg"
	InfoFlash    = "InfoMsg"
)

var (
	FlashNow bool
)

type Flash struct {
	DataStore
	url.Values
	ErrorMsg, WarningMsg, InfoMsg, SuccessMsg string
}

func (f *Flash) set(name, msg string, current ...bool) {
	isShow := false
	if (len(current) == 0 && FlashNow) ||
		(len(current) > 0 && current[0]) {
		isShow = true
	}

	if isShow {
		f.GetData()["Flash"] = f
	} else {
		f.Set(name, msg)
	}
}

func (f *Flash) Error(msg string, current ...bool) {
	f.ErrorMsg = msg
	f.set("error", msg, current...)
}

func (f *Flash) Warning(msg string, current ...bool) {
	f.WarningMsg = msg
	f.set("warning", msg, current...)
}

func (f *Flash) Info(msg string, current ...bool) {
	f.InfoMsg = msg
	f.set("info", msg, current...)
}

func (f *Flash) Success(msg string, current ...bool) {
	f.SuccessMsg = msg
	f.set("success", msg, current...)
}
