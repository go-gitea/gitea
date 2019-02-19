// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package issues

// IndexerData data stored in the issue indexer
type IndexerData struct {
	ID       int64
	RepoID   int64
	Title    string
	Content  string
	Comments []string
	IsDelete bool
	IDs      []int64
}

// Match represents on search result
type Match struct {
	ID     int64   `json:"id"`
	RepoID int64   `json:"repo_id"`
	Score  float64 `json:"score"`
}

// SearchResult represents search results
type SearchResult struct {
	Hits []Match
}

// Indexer defines an inteface to indexer issues contents
type Indexer interface {
	Init() (bool, error)
	Index(issue []*IndexerData) error
	Delete(ids ...int64) error
	Search(kw string, repoID int64, limit, start int) (*SearchResult, error)
}
