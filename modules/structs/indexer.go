// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// IndexerResult a search result to display
type IndexerResult struct {
	RepoID         int64     `json:"repo_id"`
	Filename       string    `json:"filename"`
	CommitID       string    `json:"commit_id"`
	Updated        time.Time `json:"updated"`
	Language       string    `json:"language"`
	Color          string    `json:"color"`
	LineNumbers    []int     `json:"line_numbers"`
	FormattedLines string    `json:"formated_lines"`
	ContentLines   []string  `json:"content_lines"`
}

// IndexerSearchResultLanguages result of top languages count in search results
type IndexerSearchResultLanguages struct {
	Language string `json:"language"`
	Color    string `json:"color"`
	Count    int    `json:"count"`
}
