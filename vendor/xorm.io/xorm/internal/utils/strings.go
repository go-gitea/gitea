// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"strings"
)

func IndexNoCase(s, sep string) int {
	return strings.Index(strings.ToLower(s), strings.ToLower(sep))
}

func SplitNoCase(s, sep string) []string {
	idx := IndexNoCase(s, sep)
	if idx < 0 {
		return []string{s}
	}
	return strings.Split(s, s[idx:idx+len(sep)])
}

func SplitNNoCase(s, sep string, n int) []string {
	idx := IndexNoCase(s, sep)
	if idx < 0 {
		return []string{s}
	}
	return strings.SplitN(s, s[idx:idx+len(sep)], n)
}

