// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

type WikiCommit struct {
	ID        string      `json:"sha"`
	Author    *CommitUser `json:"author"`
	Committer *CommitUser `json:"commiter"`
	Message   string      `json:"message"`
}

type WikiPage struct {
	*PageMeta
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	CommitCount int64       `json:"commit_count"`
	LastCommit  *WikiCommit `json:"last_commit"`
	Sidebar     string      `json:"sidebar"`
	Footer      string      `json:"footer"`
}

// PageMeta wiki page meta information
type PageMeta struct {
	Name    string `json:"name"`
	SubURL  string `json:"suburl"`
	Updated string `json:"updated"`
}

// NewWikiForm form for creating wiki
type NewWikiForm struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Message string `json:"message"`
}

type WikiCommitList struct {
	WikiCommits []*WikiCommit `json:"commits"`
	Count       int64         `json:"count"`
}
