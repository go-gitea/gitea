// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import "time"

// WikiCommit page commit/revision
type WikiCommit struct {
	ID        string      `json:"sha"`
	Author    *CommitUser `json:"author"`
	Committer *CommitUser `json:"commiter"`
	Message   string      `json:"message"`
}

// WikiPage a wiki page
type WikiPage struct {
	*WikiPageMetaData
	Content     string      `json:"content"`
	CommitCount int64       `json:"commit_count"`
	LastCommit  *WikiCommit `json:"last_commit"`
	Sidebar     string      `json:"sidebar"`
	Footer      string      `json:"footer"`
}

// WikiPageMetaData wiki page meta information
type WikiPageMetaData struct {
	Title   string    `json:"title"`
	SubURL  string    `json:"suburl"`
	Updated time.Time `json:"updated"`
}

// CreateWikiPageOptions form for creating wiki
type CreateWikiPageOptions struct {
	// page title. leave empty to keep unchanged
	Title string `json:"title"`
	// content must be UTF-8 encoded
	Content string `json:"content"`
	// commit message summarizing the change
	Message string `json:"message"`
}

// WikiCommitList commit/revision list
type WikiCommitList struct {
	WikiCommits []*WikiCommit `json:"commits"`
	Count       int64         `json:"count"`
}
