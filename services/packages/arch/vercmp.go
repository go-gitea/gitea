// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package arch

import (
	"strings"
	"unicode"
)

// https://gitlab.archlinux.org/pacman/pacman/-/blob/d55b47e5512808b67bc944feb20c2bcc6c1a4c45/lib/libalpm/version.c

import (
	"strconv"
)

func parseEVR(evr string) (epoch, version, release string) {
	if before, after, f := strings.Cut(evr, ":"); f {
		epoch = before
		evr = after
	} else {
		epoch = "0"
	}

	if before, after, f := strings.Cut(evr, "-"); f {
		version = before
		release = after
	} else {
		version = evr
		release = "1"
	}
	return epoch, version, release
}

func compareSegments(a, b []string) int {
	lenA, lenB := len(a), len(b)
	var l int
	if lenA > lenB {
		l = lenB
	} else {
		l = lenA
	}
	for i := 0; i < l; i++ {
		if r := compare(a[i], b[i]); r != 0 {
			return r
		}
	}
	if lenA == lenB {
		return 0
	} else if l == lenA {
		return -1
	}
	return 1
}

func compare(a, b string) int {
	if a == b {
		return 0
	}

	aNumeric := isNumeric(a)
	bNumeric := isNumeric(b)

	if aNumeric && bNumeric {
		aInt, _ := strconv.Atoi(a)
		bInt, _ := strconv.Atoi(b)
		switch {
		case aInt < bInt:
			return -1
		case aInt > bInt:
			return 1
		default:
			return 0
		}
	}

	if aNumeric {
		return 1
	}
	if bNumeric {
		return -1
	}

	return strings.Compare(a, b)
}

func isNumeric(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func compareVersions(a, b string) int {
	if a == b {
		return 0
	}

	epochA, versionA, releaseA := parseEVR(a)
	epochB, versionB, releaseB := parseEVR(b)

	if res := compareSegments([]string{epochA}, []string{epochB}); res != 0 {
		return res
	}

	if res := compareSegments(strings.Split(versionA, "."), strings.Split(versionB, ".")); res != 0 {
		return res
	}

	return compareSegments([]string{releaseA}, []string{releaseB})
}
