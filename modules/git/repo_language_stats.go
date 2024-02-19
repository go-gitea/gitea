// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"unicode"

	"code.gitea.io/gitea/modules/optional"
)

const (
	fileSizeLimit int64 = 16 * 1024   // 16 KiB
	bigFileSize   int64 = 1024 * 1024 // 1 MiB
)

// true if "set"/"true", false if "unset"/"false", none otherwise
func linguistToBool(attr map[string]string, name string) optional.Option[bool] {
	if value, has := attr[name]; has && value != "unspecified" {
		switch value {
		case "set", "true":
			return optional.Some(true)
		case "unset", "false":
			return optional.Some(false)
		}
	}
	return optional.None[bool]()
}

func linguistToString(attr map[string]string, name string) optional.Option[string] {
	if value, has := attr[name]; has && value != "unspecified" {
		return optional.Some(value)
	}
	return optional.None[string]()
}

// mergeLanguageStats mergers language names with different cases. The name with most upper case letters is used.
func mergeLanguageStats(stats map[string]int64) map[string]int64 {
	names := map[string]struct {
		uniqueName string
		upperCount int
	}{}

	countUpper := func(s string) (count int) {
		for _, r := range s {
			if unicode.IsUpper(r) {
				count++
			}
		}
		return count
	}

	for name := range stats {
		cnt := countUpper(name)
		lower := strings.ToLower(name)
		if cnt >= names[lower].upperCount {
			names[lower] = struct {
				uniqueName string
				upperCount int
			}{uniqueName: name, upperCount: cnt}
		}
	}

	res := make(map[string]int64, len(names))
	for name, num := range stats {
		res[names[strings.ToLower(name)].uniqueName] += num
	}
	return res
}
