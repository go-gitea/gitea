// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// CodeSearchResults represents the results of a code search
type CodeSearchResults struct {
	// The total number of matching files
	TotalCount int64 `json:"total_count"`
	// Whether the search stopped before finding all matching files
	IncompleteResults bool `json:"incomplete_results"`
	// The matching files
	Items []*CodeSearchResultItem `json:"items"`
}

// CodeSearchResultItem represents a file matching a code search
type CodeSearchResultItem struct {
	// The name of the file
	Name string `json:"name"`
	// The path of the file in the repository
	Path string `json:"path"`
	// The API URL of the file contents at the searched commit
	URL string `json:"url"`
	// The web URL of the file at the searched commit
	HTMLURL string `json:"html_url"`
	// The repository containing the file
	Repository *Repository `json:"repository"`
	// The line numbers of the lines in the content fragment
	LineNumbers []string `json:"line_numbers"`
	// The fragments of the file content that matched
	TextMatches []*CodeSearchTextMatch `json:"text_matches"`
}

// CodeSearchTextMatch represents a fragment of a property that matched a code search
type CodeSearchTextMatch struct {
	// The API URL of the resource containing the property
	ObjectURL string `json:"object_url"`
	// The type of the resource containing the property
	ObjectType string `json:"object_type"`
	// The name of the matched property
	Property string `json:"property"`
	// The part of the property value that matched
	Fragment string `json:"fragment"`
	// The matched terms in the fragment
	Matches []*CodeSearchTextMatchTerm `json:"matches"`
}

// CodeSearchTextMatchTerm represents a matched term in a text match fragment
type CodeSearchTextMatchTerm struct {
	// The matched text
	Text string `json:"text"`
	// The start and end character offsets of the text in the fragment
	Indices []int `json:"indices"`
}
