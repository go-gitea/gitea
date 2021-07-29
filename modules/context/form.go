// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// FormString returns fist value of a query based on key as string
func (ctx *Context) FormString(key string) string {
	return ctx.Req.FormValue(key)
}

// FormStrings returns request form as strings
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

// FormTrim returns space trimmed value of a query based on key as string
func (ctx *Context) FormTrim(key string) string {
	return strings.TrimSpace(ctx.Req.FormValue(key))
}

// FormInt returns request form as int
func (ctx *Context) FormInt(key string) int {
	v, _ := strconv.Atoi(ctx.Req.FormValue(key))
	return v
}

// FormInt64 returns request form as int64
func (ctx *Context) FormInt64(key string) int64 {
	v, _ := strconv.ParseInt(ctx.Req.FormValue(key), 10, 64)
	return v
}

// FormBool returns value of a query based on key as bool
func (ctx *Context) FormBool(key string) bool {
	v, _ := strconv.ParseBool(ctx.Req.FormValue(key))
	return v
}

// FormOptionalBool returns value of a query based on key as OptionalBool
func (ctx *Context) FormOptionalBool(key string) util.OptionalBool {
	value := ctx.Req.FormValue(key)
	if len(value) == 0 {
		return util.OptionalBoolNone
	}
	v, _ := strconv.ParseBool(ctx.Req.FormValue(key))
	return util.OptionalBoolOf(v)
}
