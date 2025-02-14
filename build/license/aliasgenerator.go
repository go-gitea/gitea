// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package license

import "strings"

func GetLicenseNameFromAliases(fnl []string) string {
	if len(fnl) == 0 {
		return ""
	}

	shortestItem := func(list []string) string {
		s := list[0]
		for _, l := range list[1:] {
			if len(l) < len(s) {
				s = l
			}
		}
		return s
	}
	allHasPrefix := func(list []string, s string) bool {
		for _, l := range list {
			if !strings.HasPrefix(l, s) {
				return false
			}
		}
		return true
	}

	sl := shortestItem(fnl)
	slv := strings.Split(sl, "-")
	var result string
	for i := len(slv); i >= 0; i-- {
		result = strings.Join(slv[:i], "-")
		if allHasPrefix(fnl, result) {
			return result
		}
	}
	return ""
}
