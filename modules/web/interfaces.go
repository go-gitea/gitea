// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package web

import "net/http"

type IRouter interface {
	http.Handler
	Group(pattern string, fn func(), middlewares ...any)
	Get(pattern string, h ...any)
	Post(pattern string, h ...any)
	Put(pattern string, h ...any)
	Patch(pattern string, h ...any)
	Delete(pattern string, h ...any)
	Methods(methods, pattern string, h ...any)
	Combo(pattern string, h ...any) ICombo
}
type ICombo interface {
	Get(h ...any) ICombo
	Post(h ...any) ICombo
	Patch(h ...any) ICombo
	Put(h ...any) ICombo
	Delete(h ...any) ICombo
}
