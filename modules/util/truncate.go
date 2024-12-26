// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// in UTF8 "…" is 3 bytes so doesn't really gain us anything...
const (
	utf8Ellipsis  = "…"
	asciiEllipsis = "..."
)

func IsLikelyEllipsisLeftPart(s string) bool {
	return strings.HasSuffix(s, utf8Ellipsis) || strings.HasSuffix(s, asciiEllipsis)
}

func ellipsisGuessDisplayWidth(r rune) int {
	// To make the truncated string as long as possible,
	// CJK/emoji chars are considered as 2-ASCII width but not 3-4 bytes width.
	// Here we only make the best guess (better than counting them in bytes),
	// it's impossible to 100% correctly determine the width of a rune without a real font and render.
	//
	// ATTENTION: the guessed width can't be zero, more details in ellipsisDisplayString's comment
	if r <= 255 {
		return 1
	}

	switch {
	case r == '\u3000': /* ideographic (CJK) characters, still use 2 */
		return 2
	case unicode.Is(unicode.M, r), /* (Mark) */
		unicode.Is(unicode.Cf, r), /* (Other, format) */
		unicode.Is(unicode.Cs, r), /* (Other, surrogate) */
		unicode.Is(unicode.Z /* (Space) */, r):
		return 1
	default:
		return 2
	}
}

// EllipsisDisplayString returns a truncated short string for display purpose.
// The length is the approximate number of ASCII-width in the string (CJK/emoji are 2-ASCII width)
// It appends "…" or "..." at the end of truncated string.
// It guarantees the length of the returned runes doesn't exceed the limit.
func EllipsisDisplayString(str string, limit int) string {
	s, _, _, _ := ellipsisDisplayString(str, limit)
	return s
}

// EllipsisDisplayStringX works like EllipsisDisplayString while it also returns the right part
func EllipsisDisplayStringX(str string, limit int) (left, right string) {
	left, offset, truncated, encounterInvalid := ellipsisDisplayString(str, limit)
	if truncated {
		right = str[offset:]
		r, _ := utf8.DecodeRune(UnsafeStringToBytes(right))
		encounterInvalid = encounterInvalid || r == utf8.RuneError
		ellipsis := utf8Ellipsis
		if encounterInvalid {
			ellipsis = asciiEllipsis
		}
		right = ellipsis + right
	}
	return left, right
}

func ellipsisDisplayString(str string, limit int) (res string, offset int, truncated, encounterInvalid bool) {
	if len(str) <= limit {
		return str, len(str), false, false
	}

	// To future maintainers: this logic must guarantee that the length of the returned runes doesn't exceed the limit,
	// because the returned string will also be used as database value. UTF-8 VARCHAR(10) could store 10 rune characters,
	// So each rune must be countered as at least 1 width.
	// Even if there are some special Unicode characters (zero-width, combining, etc.), they should NEVER be counted as zero.
	pos, used := 0, 0
	for i, r := range str {
		encounterInvalid = encounterInvalid || r == utf8.RuneError
		pos = i
		runeWidth := ellipsisGuessDisplayWidth(r)
		if used+runeWidth+3 > limit {
			break
		}
		used += runeWidth
		offset += utf8.RuneLen(r)
	}

	// if the remaining are fewer than 3 runes, then maybe we could add them, no need to ellipse
	if len(str)-pos <= 12 {
		var nextCnt, nextWidth int
		for _, r := range str[pos:] {
			if nextCnt >= 4 {
				break
			}
			nextWidth += ellipsisGuessDisplayWidth(r)
			nextCnt++
		}
		if nextCnt <= 3 && used+nextWidth <= limit {
			return str, len(str), false, false
		}
	}
	if limit < 3 {
		// if the limit is so small, do not add ellipsis
		return str[:offset], offset, true, false
	}
	ellipsis := utf8Ellipsis
	if encounterInvalid {
		ellipsis = asciiEllipsis
	}
	return str[:offset] + ellipsis, offset, true, encounterInvalid
}

// TruncateRunes returns a truncated string with given rune limit,
// it returns input string if its rune length doesn't exceed the limit.
func TruncateRunes(str string, limit int) string {
	if utf8.RuneCountInString(str) < limit {
		return str
	}
	return string([]rune(str)[:limit])
}
