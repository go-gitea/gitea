// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// FormString returns fist value matching a specific key in form as string
func (ctx *Context) FormString(key string) string {
	return ctx.Req.FormValue(key)
}

// FormStrings returns a string slice by key from form
func (ctx *Context) FormStrings(key string) []string {
	if ctx.Req.Form == nil {
		if err := ctx.Req.ParseMultipartForm(32 << 20); err != nil {
			return nil
		}
	}
	if v, ok := ctx.Req.Form[key]; ok {
		return v
	}
	return nil
}

// FormTrim returns space trimmed fist value matching a specific key in form as string
func (ctx *Context) FormTrim(key string) string {
	return strings.TrimSpace(ctx.Req.FormValue(key))
}

// FormInt returns fist value matching a specific key in form as int
func (ctx *Context) FormInt(key string) int {
	v, _ := strconv.Atoi(ctx.Req.FormValue(key))
	return v
}

// FormInt64 returns fist value matching a specific key in form as int64
func (ctx *Context) FormInt64(key string) int64 {
	v, _ := strconv.ParseInt(ctx.Req.FormValue(key), 10, 64)
	return v
}

// FormBool returns true if value matching a specific key in form "1" or "true"
func (ctx *Context) FormBool(key string) bool {
	v, _ := strconv.ParseBool(ctx.Req.FormValue(key))
	return v
}

// FormOptionalBool returns an OptionalBoolTrue or OptionalBoolFalse if value for key exist
// else it return OptionalBoolNone
func (ctx *Context) FormOptionalBool(key string) util.OptionalBool {
	value := ctx.Req.FormValue(key)
	if len(value) == 0 {
		return util.OptionalBoolNone
	}
	v, _ := strconv.ParseBool(ctx.Req.FormValue(key))
	return util.OptionalBoolOf(v)
}
