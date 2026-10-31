// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CodeSearchResults contains matching files.
type CodeSearchResults struct {
	TotalCount int64 `json:"total_count"`
	// Search stopped early
	IncompleteResults bool                    `json:"incomplete_results"`
	Items             []*CodeSearchResultItem `json:"items"`
}

// CodeSearchResultItem is a matching file.
type CodeSearchResultItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Contents API URL at the searched commit
	URL string `json:"url"`
	// File URL at the searched commit
	HTMLURL string `json:"html_url"`
	// Programming language (Gitea extension)
	Language    string                 `json:"language,omitempty"`
	Repository  *Repository            `json:"repository"`
	TextMatches []*CodeSearchTextMatch `json:"text_matches"`
}

// CodeSearchTextMatch is a matching fragment.
type CodeSearchTextMatch struct {
	ObjectURL  string                     `json:"object_url"`
	ObjectType string                     `json:"object_type"`
	Property   string                     `json:"property"`
	Fragment   string                     `json:"fragment"`
	Matches    []*CodeSearchTextMatchTerm `json:"matches"`
}

// CodeSearchTextMatchTerm is a matched term.
type CodeSearchTextMatchTerm struct {
	Text string `json:"text"`
	// Start and end UTF-8 byte offsets in the fragment, end-exclusive
	Indices []int `json:"indices"`
}
