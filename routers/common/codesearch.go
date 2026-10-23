// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"codenest.dev/modules/indexer"
	code_indexer "codenest.dev/modules/indexer/code"
	"codenest.dev/modules/setting"
	"codenest.dev/services/context"
)

func PrepareCodeSearch(ctx *context.Context) (ret struct {
	Keyword    string
	Language   string
	SearchMode indexer.SearchModeType
},
) {
	ret.Language = ctx.FormTrim("l")
	ret.Keyword = ctx.FormTrim("q")
	ret.SearchMode = indexer.SearchModeType(ctx.FormTrim("search_mode"))

	ctx.Data["Keyword"] = ret.Keyword
	ctx.Data["Language"] = ret.Language
	ctx.Data["SelectedSearchMode"] = string(ret.SearchMode)
	if setting.Indexer.RepoIndexerEnabled {
		ctx.Data["SearchModes"] = code_indexer.SupportedSearchModes()
	} else {
		ctx.Data["SearchModes"] = indexer.GitGrepSupportedSearchModes()
	}
	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	return ret
}
