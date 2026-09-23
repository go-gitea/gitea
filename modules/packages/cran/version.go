// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cran

import (
	"cmp"
	"strings"
)

// CompareVersions compares validated R package versions as sequences of integers.
func CompareVersions(a, b string) int {
	split := func(v string) []string {
		return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })
	}
	aa, bb := split(a), split(b)
	for i := range max(len(aa), len(bb)) {
		var x, y string
		if i < len(aa) {
			x = strings.TrimLeft(aa[i], "0")
		}
		if i < len(bb) {
			y = strings.TrimLeft(bb[i], "0")
		}
		if c := cmp.Compare(len(x), len(y)); c != 0 {
			return c
		}
		if c := strings.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}
