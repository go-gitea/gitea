// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

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
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	CommitCount int64       `json:"commit_count"`
	LastCommit  *WikiCommit `json:"last_commit"`
	Sidebar     string      `json:"sidebar"`
	Footer      string      `json:"footer"`
}

// WikiPageMetaData wiki page meta information
type WikiPageMetaData struct {
	Name    string `json:"name"`
	SubURL  string `json:"suburl"`
	Updated string `json:"updated"`
}

// CreateWikiPageOptions form for creating wiki
type CreateWikiPageOptions struct {
	Title string `json:"title"`
	// content must be UTF-8 encoded
	Content string `json:"content"`
	Message string `json:"message"`
}

// WikiCommitList commit/revision list
type WikiCommitList struct {
	WikiCommits []*WikiCommit `json:"commits"`
	Count       int64         `json:"count"`
}
