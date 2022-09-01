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

// Zero returns if the form is Zero
func (f Form) Zero() bool {
	return f == Zero
}

// One returns if the form is One
func (f Form) One() bool {
	return f == One
}

// Two returns if the form is Two
func (f Form) Two() bool {
	return f == Two
}

// Few returns if the form is Few
func (f Form) Few() bool {
	return f == Few
}

// Many returns if the form is Many
func (f Form) Many() bool {
	return f == Many
}

// Other returns if the form is Other
func (f Form) Other() bool {
	return f == Other
}
