// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package internal

import "code.gitea.io/gitea/modules/timeutil"

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
}

// IndexerData represents data stored in the code indexer
type IndexerData struct {
	RepoID int64
}

// SearchResult result of performing a search in a repo
type SearchResult struct {
	RepoID      int64
	StartIndex  int
	EndIndex    int
	Filename    string
	Content     string
	CommitID    string
	UpdatedUnix timeutil.TimeStamp
	Language    string
	Color       string
}

// SearchResultLanguages result of top languages count in search results
type SearchResultLanguages struct {
	Language string
	Color    string
	Count    int
}
