// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"regexp"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/setting"
)

// OptionalBool a boolean that can be "null"
type OptionalBool byte

const (
	// OptionalBoolNone a "null" boolean value
	OptionalBoolNone = iota
	// OptionalBoolTrue a "true" boolean value
	OptionalBoolTrue
	// OptionalBoolFalse a "false" boolean value
	OptionalBoolFalse
)

var matchComplexities = map[string]regexp.Regexp{}
var matchComplexityOnce sync.Once

// CheckPasswordComplexity return True if password is Complexity
func CheckPasswordComplexity(pwd string) bool {
	matchComplexityOnce.Do(func() {
		for key, val := range setting.PasswordComplexity {
			var matchComplexity *regexp.Regexp
			matchComplexity = regexp.MustCompile(val)
			matchComplexities[key] = *matchComplexity

		}
	})
	for _, val := range matchComplexities {
		if !val.MatchString(pwd) {
			return false
		}
	}
	return true
}

// IsTrue return true if equal to OptionalBoolTrue
func (o OptionalBool) IsTrue() bool {
	return o == OptionalBoolTrue
}

// IsFalse return true if equal to OptionalBoolFalse
func (o OptionalBool) IsFalse() bool {
	return o == OptionalBoolFalse
}

// IsNone return true if equal to OptionalBoolNone
func (o OptionalBool) IsNone() bool {
	return o == OptionalBoolNone
}

// OptionalBoolOf get the corresponding OptionalBool of a bool
func OptionalBoolOf(b bool) OptionalBool {
	if b {
		return OptionalBoolTrue
	}
	return OptionalBoolFalse
}

// Max max of two ints
func Max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// Min min of two ints
func Min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// IsEmptyString checks if the provided string is empty
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
