// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"errors"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/container"
	"gitea.dev/modules/indexer"
	code_indexer "gitea.dev/modules/indexer/code"
	"gitea.dev/modules/setting"
	"gitea.dev/services/codesearch"
	"gitea.dev/services/context"
)

func SearchCode(ctx *context.Context, opts *codesearch.Options) {
	keyword := ctx.FormString("q")
	ctx.Data["Keyword"] = keyword
	ctx.Data["IsRepoIndexerEnabled"] = opts.Engine == codesearch.Indexer
	if keyword == "" {
		return
	}
	opts.Doer = ctx.Doer
	opts.Paginator = &db.ListOptions{Page: max(1, ctx.FormInt("page")), PageSize: setting.UI.RepoSearchPagingNum}
	result, err := codesearch.Search(ctx, keyword, opts)
	if err != nil {
		if queryError, ok := errors.AsType[*indexer.SearchQueryError](err); ok {
			ctx.Data["CodeSearchError"] = queryError.Message
		} else if errors.Is(err, codesearch.ErrIndexerUnavailable) {
			ctx.Data["CodeIndexerUnavailable"] = true
		} else {
			ctx.ServerError("SearchCode", err)
		}
		return
	}
	ctx.Data["Language"] = result.Query.Language
	ctx.Data["KeywordWithoutLanguage"] = result.Query.WithoutLanguage()
	results := make([]*codeSearchResult, len(result.Results))
	for i, hit := range result.Results {
		results[i] = &codeSearchResult{Result: hit}
		for _, fragment := range code_indexer.BuildResultFragments(hit.Content, hit.ContentMatches) {
			content := strings.TrimSuffix(fragment.Content, "\n")
			lineNumbers := make([]int, 1+strings.Count(content, "\n"))
			for i := range lineNumbers {
				lineNumbers[i] = fragment.StartLine + i
			}
			results[i].Fragments = append(results[i].Fragments, code_indexer.HighlightSearchResultCode(hit.Filename, hit.Language, lineNumbers, content))
		}
	}
	ctx.Data["SearchResults"] = results
	ctx.Data["SearchResultLanguages"] = result.Languages
	ctx.Data["RepoMaps"] = result.Repos
	ctx.Data["CodeSearchIncomplete"] = result.Incomplete
	pager := context.NewPagerBuilder(ctx).TotalCount(result.Total).PerPageLimit(opts.Paginator.PageSize).CurPage(opts.Paginator.Page).Build()
	pager.RemoveParam(container.SetOf("search_mode", "l"))
	ctx.Data["Page"] = pager
}

type codeSearchResult struct {
	*code_indexer.Result
	Fragments [][]*code_indexer.ResultLine
}
