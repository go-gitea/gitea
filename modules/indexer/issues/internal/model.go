// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64    `json:"id"`
	RepoID   int64    `json:"repo_id"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Comments []string `json:"comments"`
	IsDelete bool     `json:"is_delete"`
	IDs      []int64  `json:"ids"`
}

// Match represents on search result
type Match struct {
	ID    int64   `json:"id"`
	Score float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Total int64
	Hits  []Match
}
