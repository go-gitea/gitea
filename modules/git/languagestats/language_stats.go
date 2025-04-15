// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package languagestats

import (
	"context"
	"strings"
	"unicode"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/attribute"
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

// GetFileLanguage tries to get the (linguist) language of the file content
func GetFileLanguage(ctx context.Context, gitRepo *git.Repository, treeish, treePath string) (string, error) {
	attributesMap, err := attribute.CheckAttributes(ctx, gitRepo, treeish, attribute.CheckAttributeOpts{
		Attributes: []string{attribute.LinguistLanguage, attribute.GitlabLanguage},
		Filenames:  []string{treePath},
	})
	if err != nil {
		return "", err
	}

	return attributesMap[treePath].GetLanguage().Value(), nil
}
