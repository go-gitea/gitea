// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "unicode/utf8"

// in UTF8 "…" is 3 bytes so doesn't really gain us anything...
const utf8Ellipsis = "…"
const asciiEllipsis = "..."

// SplitStringAtByteN splits a string at byte n accounting for rune boundaries. (Combining characters are not accounted for.)
func SplitStringAtByteN(input string, n int) (left, right string) {
	if len(input) <= n {
		return input, ""
	}

	if !utf8.ValidString(input) {
		left = input[:n-3] + asciiEllipsis
		right = asciiEllipsis + input[n-3:]
		return
	}

	end := 0
	for end <= n-3 {
		_, size := utf8.DecodeRuneInString(input[end:])
		if end+size > n-3 {
			break
		}
		end += size
	}

	left = input[:end] + utf8Ellipsis
	right = utf8Ellipsis + input[end:]
	return
}

// SplitStringAtRuneN splits a string at rune n accounting for rune boundaries. (Combining characters are not accounted for.)
func SplitStringAtRuneN(input string, n int) (left, right string) {
	if !utf8.ValidString(input) {
		if len(input) <= n {
			return input, ""
		}
		left = input[:n-3] + asciiEllipsis
		right = asciiEllipsis + input[n-3:]
		return
	}

	if utf8.RuneCountInString(input) <= n {
		return input, ""
	}

	count := 0
	end := 0
	for count < n-1 {
		_, size := utf8.DecodeRuneInString(input[end:])
		end += size
		count++
	}

	left = input[:end] + utf8Ellipsis
	right = utf8Ellipsis + input[end:]
	return
}
