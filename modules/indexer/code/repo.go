// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package code

import (
	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search/query"
)

// RepoSearchResult result of performing a search in a repo
type RepoSearchResult struct {
	RepoID     int64
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
}

// SearchRepoByKeyword searches for files in the specified repo.
// Returns the matching file-paths
func SearchRepoByKeyword(repoIDs []int64, keyword string, page, pageSize int) (int64, []*RepoSearchResult, error) {
	phraseQuery := bleve.NewMatchPhraseQuery(keyword)
	phraseQuery.FieldVal = "Content"
	phraseQuery.Analyzer = repoIndexerAnalyzer

	var indexerQuery query.Query
	if len(repoIDs) > 0 {
		var repoQueries = make([]query.Query, 0, len(repoIDs))
		for _, repoID := range repoIDs {
			repoQueries = append(repoQueries, numericEqualityQuery(repoID, "RepoID"))
		}

		indexerQuery = bleve.NewConjunctionQuery(
			bleve.NewDisjunctionQuery(repoQueries...),
			phraseQuery,
		)
	} else {
		indexerQuery = phraseQuery
	}

	from := (page - 1) * pageSize
	searchRequest := bleve.NewSearchRequestOptions(indexerQuery, pageSize, from, false)
	searchRequest.Fields = []string{"Content", "RepoID"}
	searchRequest.IncludeLocations = true

	result, err := indexerHolder.get().Search(searchRequest)
	if err != nil {
		return 0, nil, err
	}

	searchResults := make([]*RepoSearchResult, len(result.Hits))
	for i, hit := range result.Hits {
		var startIndex, endIndex int = -1, -1
		for _, locations := range hit.Locations["Content"] {
			location := locations[0]
			locationStart := int(location.Start)
			locationEnd := int(location.End)
			if startIndex < 0 || locationStart < startIndex {
				startIndex = locationStart
			}
			if endIndex < 0 || locationEnd > endIndex {
				endIndex = locationEnd
			}
		}
		searchResults[i] = &RepoSearchResult{
			RepoID:     int64(hit.Fields["RepoID"].(float64)),
			StartIndex: startIndex,
			EndIndex:   endIndex,
			Filename:   filenameOfIndexerID(hit.ID),
			Content:    hit.Fields["Content"].(string),
		}
	}
	return int64(result.Total), searchResults, nil
}
