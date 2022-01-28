// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package strings

import (
	"strings"
	"unicode/utf8"
)

// ToASCIIUpper returns s with all ASCII letters mapped to their upper case.
func ToASCIIUpper(s string) string {
	isASCII, hasLower := true, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			isASCII = false
			break
		}
		hasLower = hasLower || ('a' <= c && c <= 'z')
	}

	// Optimize for ASCII-only strings.
	if isASCII {
		if !hasLower {
			return s
		}
		var b strings.Builder
		b.Grow(len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if 'a' <= c && c <= 'z' {
				c -= 'a' - 'A'
			}
			b.WriteByte(c)
		}
		return b.String()
	}

	sBytes := []byte(s)
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(sBytes); {
		// Use ut8 because it includes non-ASCII letters.
		r, width := utf8.DecodeRune(sBytes[i:])
		i += width

		if r == utf8.RuneError {
			// Might change to RUNE_ERROR, which can be tracked down
			// via the SQL Logs what's possibly going on.
			return s
		}
		// Only uppercase ASCII.
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}
