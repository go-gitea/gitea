// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "unicode/utf8"

// SplitStringAtByteN splits a string at byte n accounting for rune boundaries. (Combining characters are not accounted for.)
func SplitStringAtByteN(input string, n int) (left, right string) {
	if len(input) <= n {
		left = input
		return
	}

	if !utf8.ValidString(input) {
		left = input[:n-3] + "..."
		right = "..." + input[n-3:]
		return
	}

	// in UTF8 "…" is 3 bytes so doesn't really gain us anything...
	end := 0
	for end <= n-3 {
		_, size := utf8.DecodeRuneInString(input[end:])
		if end+size > n-3 {
			break
		}
		end += size
	}

	left = input[:end] + "…"
	right = "…" + input[end:]
	return
}
