// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base

import (
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func naturalSortGetRune(str string, pos int) (r rune, size int, has bool) {
	if pos >= len(str) {
		return 0, 0, false
	}
	r, size = utf8.DecodeRuneInString(str[pos:])
	if r == utf8.RuneError {
		r, size = rune(str[pos]), 1 // if invalid input, treat it as a single byte ascii
	}
	return r, size, true
}

func naturalSortAdvance(str string, pos int) (end int, isNumber bool) {
	end = pos
	for {
		r, size, has := naturalSortGetRune(str, end)
		if !has {
			break
		}
		isCurRuneNum := '0' <= r && r <= '9'
		if end == pos {
			isNumber = isCurRuneNum
			end += size
		} else if isCurRuneNum == isNumber {
			end += size
		} else {
			break
		}
	}
	return end, isNumber
}

// NaturalSortLess compares two strings so that they could be sorted in natural order
func NaturalSortLess(s1, s2 string) bool {
	// There is a bug in Golang's collate package: https://github.com/golang/go/issues/67997
	// text/collate: CompareString(collate.Numeric) returns wrong result for "0.0" vs "1.0" #67997
	// So we need to handle the number parts by ourselves
	c := collate.New(language.English, collate.Numeric)
	pos1, pos2 := 0, 0
	for pos1 < len(s1) && pos2 < len(s2) {
		end1, isNum1 := naturalSortAdvance(s1, pos1)
		end2, isNum2 := naturalSortAdvance(s2, pos2)
		part1, part2 := s1[pos1:end1], s2[pos2:end2]
		if isNum1 && isNum2 {
			if part1 != part2 {
				if len(part1) != len(part2) {
					return len(part1) < len(part2)
				}
				return part1 < part2
			}
		} else {
			if cmp := c.CompareString(part1, part2); cmp != 0 {
				return cmp < 0
			}
		}
		pos1, pos2 = end1, end2
	}
	return len(s1) < len(s2)
}
