// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

import (
	"sort"
	"strings"
	"unicode"

	"code.gitea.io/gitea/modules/translation"
)

// AmbiguousTablesForLocale provides the table of ambiguous characters for this locale.
func AmbiguousTablesForLocale(locale translation.Locale) []*AmbiguousTable {
	ambiguousTableMap := globalVars().ambiguousTableMap
	key := locale.Language()
	var table *AmbiguousTable
	var ok bool
	for len(key) > 0 {
		if table, ok = ambiguousTableMap[key]; ok {
			break
		}
		idx := strings.LastIndexAny(key, "-_")
		if idx < 0 {
			key = ""
		} else {
			key = key[:idx]
		}
	}
	if table == nil && (locale.Language() == "zh-CN" || locale.Language() == "zh_CN") {
		table = ambiguousTableMap["zh-hans"]
	}
	if table == nil && strings.HasPrefix(locale.Language(), "zh") {
		table = ambiguousTableMap["zh-hant"]
	}
	if table == nil {
		table = ambiguousTableMap["_default"]
	}

	return []*AmbiguousTable{
		table,
		ambiguousTableMap["_common"],
	}
}

func isAmbiguous(r rune, confusableTo *rune, tables ...*AmbiguousTable) bool {
	for _, table := range tables {
		if !unicode.Is(table.RangeTable, r) {
			continue
		}
		i := sort.Search(len(table.Confusable), func(i int) bool {
			return table.Confusable[i] >= r
		})
		*confusableTo = table.With[i]
		return true
	}
	return false
}
