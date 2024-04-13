// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// NaturalSortLess compares two strings so that they could be sorted in natural order
func NaturalSortLess(s1, s2 string) bool {
	c := collate.New(language.English, collate.Numeric)
	return c.CompareString(s1, s2) < 0
}
