// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// This file is heavily inspired by https://github.com/nicksnyder/go-i18n/tree/main/v2/internal/plural

package plurals

// Form represents a language pluralization form as defined here:
// http://cldr.unicode.org/index/cldr-spec/plural-rules
type Form string

const (
	Invalid Form = ""
	Zero    Form = "zero"
	One     Form = "one"
	Two     Form = "two"
	Few     Form = "few"
	Many    Form = "many"
	Other   Form = "other"
)
