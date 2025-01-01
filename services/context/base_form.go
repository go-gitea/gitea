// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/util"
)

// FormString returns the first value matching the provided key in the form as a string
func (b *Base) FormString(key string, def ...string) string {
	s := b.Req.FormValue(key)
	if s == "" {
		s = util.OptionalArg(def)
	}
	return s
}

// FormStrings returns a string slice for the provided key from the form
func (b *Base) FormStrings(key string) []string {
	if b.Req.Form == nil {
		if err := b.Req.ParseMultipartForm(32 << 20); err != nil {
			return nil
		}
	}
	if v, ok := b.Req.Form[key]; ok {
		return v
	}
	return nil
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
func (b *Base) FormBool(key string) bool {
	s := b.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return v
}

// FormOptionalBool returns an optional.Some(true) or optional.Some(false) if the value
// for the provided key exists in the form else it returns optional.None[bool]()
func (b *Base) FormOptionalBool(key string) optional.Option[bool] {
	value := b.Req.FormValue(key)
	if len(value) == 0 {
		return optional.None[bool]()
	}
	s := b.Req.FormValue(key)
	v, _ := strconv.ParseBool(s)
	v = v || strings.EqualFold(s, "on")
	return optional.Some(v)
}

func (b *Base) SetFormString(key, value string) {
	_ = b.Req.FormValue(key) // force parse form
	b.Req.Form.Set(key, value)
}
