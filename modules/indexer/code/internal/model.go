// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import "gitea.dev/modules/timeutil"

type FileUpdate struct {
	Filename string
	BlobSha  string
	Size     int64
	Sized    bool
}

// RepoChanges changes (file additions/updates/removals) to a repo
type RepoChanges struct {
	Updates          []FileUpdate
	RemovedFilenames []string
	// Genesis is true when Updates covers the complete tree instead of a diff,
	// so an indexer can rebuild its index from scratch without listing it again.
	Genesis bool
}

// IndexerData represents data stored in the code indexer
type IndexerData struct {
	RepoID int64
}

// SearchResult result of performing a search in a repo
type SearchResult struct {
	RepoID     int64
	StartIndex int
	EndIndex   int
	Filename   string
	Content    string
	// ContentStartLineNum is the 1-based line number of the first line of
	// Content within the file. Indexers that return the whole file content may
	// leave it as 0, which is treated as line 1.
	ContentStartLineNum int
	CommitID            string
	UpdatedUnix         timeutil.TimeStamp
	Language            string
	Color               string
}

// SearchResultLanguages result of top languages count in search results
type SearchResultLanguages struct {
	Language string
	Color    string
	Count    int
}
