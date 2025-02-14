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

func TryReadLanguageAttribute(attrs map[string]string) optional.Option[string] {
	language := AttributeToString(attrs, AttributeLinguistLanguage)
	if language.Value() == "" {
		language = AttributeToString(attrs, AttributeGitlabLanguage)
		if language.Has() {
			raw := language.Value()
			// gitlab-language may have additional parameters after the language
			// ignore them and just use the main language
			// https://docs.gitlab.com/ee/user/project/highlighting.html#override-syntax-highlighting-for-a-file-type
			if idx := strings.IndexByte(raw, '?'); idx >= 0 {
				language = optional.Some(raw[:idx])
			}
		}
	}
	return language
}
