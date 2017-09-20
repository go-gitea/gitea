// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"math/big"
	"unicode/utf8"
)

// NaturalSortLess compares two strings so that they could be sorted in natural order
func NaturalSortLess(s1, s2 string) bool {
	var i1, i2 int
	for {
		rune1, j1, end1 := getNextRune(s1, i1)
		rune2, j2, end2 := getNextRune(s2, i2)
		if end1 || end2 {
			return end1 != end2 && end1
		}
		dec1 := isDecimal(rune1)
		dec2 := isDecimal(rune2)
		var less, equal bool
		if dec1 && dec2 {
			i1, i2, less, equal = compareByNumbers(s1, i1, s2, i2)
		} else if !dec1 && !dec2 {
			equal = rune1 == rune2
			less = rune1 < rune2
			i1 = j1
			i2 = j2
		} else {
			return rune1 < rune2
		}
		if !equal {
			return less
		}
	}
}

func getNextRune(str string, pos int) (rune, int, bool) {
	if pos < len(str) {
		r, w := utf8.DecodeRuneInString(str[pos:])
		// Fallback to ascii
		if r == utf8.RuneError {
			r = rune(str[pos])
			w = 1
		}
		return r, pos + w, false
	}
	return 0, pos, true
}

func isDecimal(r rune) bool {
	return '0' <= r && r <= '9'
}

func compareByNumbers(str1 string, pos1 int, str2 string, pos2 int) (i1, i2 int, less, equal bool) {
	var d1, d2 bool = true, true
	var dec1, dec2 string
	for d1 || d2 {
		if d1 {
			r, j, end := getNextRune(str1, pos1)
			if !end && isDecimal(r) {
				dec1 += string(r)
				pos1 = j
			} else {
				d1 = false
			}
		}
		if d2 {
			r, j, end := getNextRune(str2, pos2)
			if !end && isDecimal(r) {
				dec2 += string(r)
				pos2 = j
			} else {
				d2 = false
			}
		}
	}
	less, equal = compareBigNumbers(dec1, dec2)
	return pos1, pos2, less, equal
}

func compareBigNumbers(dec1, dec2 string) (less, equal bool) {
	d1, _ := big.NewInt(0).SetString(dec1, 10)
	d2, _ := big.NewInt(0).SetString(dec2, 10)
	cmp := d1.Cmp(d2)
	return cmp < 0, cmp == 0
}
