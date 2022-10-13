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
	// Page content, base64 encoded
	ContentBase64 string `json:"content_base64"`
	CommitCount   int64  `json:"commit_count"`
	Sidebar       string `json:"sidebar"`
	Footer        string `json:"footer"`
}

// WikiPageMetaData wiki page meta information
type WikiPageMetaData struct {
	Title      string      `json:"title"`
	HTMLURL    string      `json:"html_url"`
	SubURL     string      `json:"sub_url"`
	LastCommit *WikiCommit `json:"last_commit"`
}

// CreateWikiPageOptions form for creating wiki
type CreateWikiPageOptions struct {
	// page title. leave empty to keep unchanged
	Title string `json:"title"`
	// content must be base64 encoded
	ContentBase64 string `json:"content_base64"`
	// optional commit message summarizing the change
	Message string `json:"message"`
}

// WikiCommitList commit/revision list
type WikiCommitList struct {
	WikiCommits []*WikiCommit `json:"commits"`
	Count       int64         `json:"count"`
}
