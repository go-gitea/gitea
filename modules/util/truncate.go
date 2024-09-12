// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
	"unicode/utf8"
)

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

// SplitTrimSpace splits the string at given separator and trims leading and trailing space
func SplitTrimSpace(input, sep string) []string {
	// replace CRLF with LF
	input = strings.ReplaceAll(input, "\r\n", "\n")

	var stringList []string
	for _, s := range strings.Split(input, sep) {
		// trim leading and trailing space
		stringList = append(stringList, strings.TrimSpace(s))
	}

	return stringList
}
