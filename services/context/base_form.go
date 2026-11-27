// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"

	"gitea.dev/modules/base"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/util"
	"gitea.dev/modules/web/binding"
)

// FormString returns the first value matching the provided key in the form as a string
// It works the same as http.Request.FormValue:
// try urlencoded request body first, then query string, then multipart form body
func (b *Base) FormString(key string, def ...string) string {
	if v := b.FormValues().Get(key); v != "" {
		return v
	}
	return util.OptionalArg(def)
}

// FormStrings returns a values for the key in the form (including query parameters), similar to FormString
func (b *Base) FormStrings(key string) []string {
	return b.FormValues()[key]
}

func (b *Base) FormValues() url.Values {
	_ = b.Req.ParseMultipartForm(binding.ParseMultipartFormMaxMemory) // the same logic as http.Request.FormValue
	return b.Req.Form
}

func (b *Base) FormStringInt64s(key string) []int64 {
	vals, _ := base.StringsToInt64s(strings.Split(b.FormString(key), ","))
	return vals
}

func (b *Base) FormFile(key string) (multipart.File, *multipart.FileHeader, error) {
	if err := b.Req.ParseMultipartForm(binding.ParseMultipartFormMaxMemory); err != nil {
		return nil, nil, err
	}
	return b.Req.FormFile(key)
}

// FormTrim returns the first value for the provided key in the form as a space trimmed string
func (b *Base) FormTrim(key string) string {
	return strings.TrimSpace(b.Req.FormValue(key))
}

// FormInt returns the first value for the provided key in the form as an int
func (b *Base) FormInt(key string) int {
	v, _ := strconv.Atoi(b.Req.FormValue(key))
	return v
}

// FormInt64 returns the first value for the provided key in the form as an int64
func (b *Base) FormInt64(key string) int64 {
	v, _ := strconv.ParseInt(b.Req.FormValue(key), 10, 64)
	return v
}

// FormBool returns true if the value for the provided key in the form is "1", "true" or "on"
func (b *Base) FormBool(key string, def ...bool) bool {
	s := b.Req.FormValue(key)
	if s == "" {
		return util.OptionalArg(def)
	}
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return v
}

// FormOptionalBool returns an optional.Some(true) or optional.Some(false) if the value
// for the provided key exists in the form else it returns optional.None[bool]()
func (b *Base) FormOptionalBool(key string) optional.Option[bool] {
	s := b.Req.FormValue(key)
	if s == "" {
		return optional.None[bool]()
	}
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return optional.Some(v)
}

func (b *Base) FormOptionalInt64(key string) optional.Option[int64] {
	s := b.Req.FormValue(key)
	v, err := strconv.ParseInt(s, 10, 64)
	if s == "" || err != nil {
		return optional.None[int64]()
	}
	return optional.Some(v)
}
