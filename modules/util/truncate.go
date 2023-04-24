// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import "unicode/utf8"

// in UTF8 "…" is 3 bytes so doesn't really gain us anything...
const (
	utf8Ellipsis  = "…"
	asciiEllipsis = "..."
)

// SplitStringAtByteN splits a string at byte n accounting for rune boundaries. (Combining characters are not accounted for.)
func SplitStringAtByteN(input string, n int) (left, right string) {
	if len(input) <= n {
		return input, ""
	}

	if !utf8.ValidString(input) {
		if n-3 < 0 {
			return input, ""
		}
		return input[:n-3] + asciiEllipsis, asciiEllipsis + input[n-3:]
	}

	end := 0
	for end <= n-3 {
		_, size := utf8.DecodeRuneInString(input[end:])
		if end+size > n-3 {
			break
		}
		end += size
	}

	return input[:end] + utf8Ellipsis, utf8Ellipsis + input[end:]
}
