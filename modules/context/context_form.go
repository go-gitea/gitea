// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/util"
)

// FormString returns the first value matching the provided key in the form as a string
func (ctx *Context) FormString(key string) string {
	return ctx.Req.FormValue(key)
}

// FormStrings returns a string slice for the provided key from the form
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

// FormTrim returns the first value for the provided key in the form as a space trimmed string
func (ctx *Context) FormTrim(key string) string {
	return strings.TrimSpace(ctx.Req.FormValue(key))
}

// FormInt returns the first value for the provided key in the form as an int
func (ctx *Context) FormInt(key string) int {
	v, _ := strconv.Atoi(ctx.Req.FormValue(key))
	return v
}

// FormInt64 returns the first value for the provided key in the form as an int64
func (ctx *Context) FormInt64(key string) int64 {
	v, _ := strconv.ParseInt(ctx.Req.FormValue(key), 10, 64)
	return v
}

// FormBool returns true if the value for the provided key in the form is "1", "true" or "on"
func (ctx *Context) FormBool(key string) bool {
	s := ctx.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return v
}

// FormOptionalBool returns an OptionalBoolTrue or OptionalBoolFalse if the value
// for the provided key exists in the form else it returns OptionalBoolNone
func (ctx *Context) FormOptionalBool(key string) util.OptionalBool {
	value := ctx.Req.FormValue(key)
	if len(value) == 0 {
		return util.OptionalBoolNone
	}
	s := ctx.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return util.OptionalBoolOf(v)
}

func (ctx *Context) SetFormString(key, value string) {
	_ = ctx.Req.FormValue(key) // force parse form
	ctx.Req.Form.Set(key, value)
}
