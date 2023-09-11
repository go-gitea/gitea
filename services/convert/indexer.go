// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	api "code.gitea.io/gitea/modules/structs"
)

// ToIndexerSearchResult converts IndexerSearch to API format
func ToIndexerSearchResult(result *code_indexer.Result) *api.IndexerResult {
	return &api.IndexerResult{
		RepoID:         result.RepoID,
		Filename:       result.Filename,
		CommitID:       result.CommitID,
		Updated:        result.UpdatedUnix.AsTime(),
		Language:       result.Language,
		Color:          result.Color,
		LineNumbers:    result.LineNumbers,
		FormattedLines: result.FormattedLines,
		ContentLines:   result.ContentLines,
	}
}

// ToIndexerSearchResultLanguages converts IndexerSearch to API format
func ToIndexerSearchResultLanguages(result *code_indexer.SearchResultLanguages) *api.IndexerSearchResultLanguages {
	return &api.IndexerSearchResultLanguages{
		Language: result.Language,
		Color:    result.Color,
		Count:    result.Count,
	}
}

// ToIndexerSearchResultList convert list of *code_indexer.Result to list of *api.IndexerResult
func ToIndexerSearchResultList(results []*code_indexer.Result) []*api.IndexerResult {
	convertedResults := make([]*api.IndexerResult, len(results))
	for i := range results {
		convertedResults[i] = ToIndexerSearchResult(results[i])
	}
	return convertedResults
}

// ToIndexerSearchResultLanguagesList convert list of *code_indexer.SearchResultLanguages to list of *api.IndexerSearchResultLanguages
func ToIndexerSearchResultLanguagesList(results []*code_indexer.SearchResultLanguages) []*api.IndexerSearchResultLanguages {
	convertedResults := make([]*api.IndexerSearchResultLanguages, len(results))
	for i := range results {
		convertedResults[i] = ToIndexerSearchResultLanguages(results[i])
	}
	return convertedResults
}
