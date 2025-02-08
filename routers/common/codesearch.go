// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
)

func PrepareCodeSearch(ctx *context.Context) (ret struct {
	Keyword  string
	Language string
	IsFuzzy  bool
},
) {
	ret.Language = ctx.FormTrim("l")
	ret.Keyword = ctx.FormTrim("q")

	fuzzyDefault := setting.Indexer.RepoIndexerEnabled
	fuzzyAllow := true
	if setting.Indexer.RepoType == "bleve" && setting.Indexer.TypeBleveMaxFuzzniess == 0 {
		fuzzyDefault = false
		fuzzyAllow = false
	}
	isFuzzy := ctx.FormOptionalBool("fuzzy").ValueOrDefault(fuzzyDefault)
	if isFuzzy && !fuzzyAllow {
		ctx.Flash.Info("Fuzzy search is disabled by default due to performance reasons")
		isFuzzy = false
	}

	ctx.Data["IsBleveFuzzyDisabled"] = true
	ctx.Data["Keyword"] = ret.Keyword
	ctx.Data["Language"] = ret.Language
	ctx.Data["IsFuzzy"] = isFuzzy

	ctx.Data["IsRepoIndexerEnabled"] = setting.Indexer.RepoIndexerEnabled
	return ret
}
