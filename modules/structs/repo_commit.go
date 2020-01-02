// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Identity for a person's identity like an author or committer
type Identity struct {
	Name string `json:"name" binding:"MaxSize(100)"`
	// swagger:strfmt email
	Email string `json:"email" binding:"MaxSize(254)"`
}

// CommitMeta contains meta information of a commit in terms of API.
type CommitMeta struct {
	URL string `json:"url"`
	SHA string `json:"sha"`
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Identity
	Date string `json:"date"`
}

// RepoCommit contains information of a commit in the context of a repository.
type RepoCommit struct {
	URL       string      `json:"url"`
	Author    *CommitUser `json:"author"`
	Committer *CommitUser `json:"committer"`
	Message   string      `json:"message"`
	Tree      *CommitMeta `json:"tree"`
}

// Commit contains information generated from a Git commit.
type Commit struct {
	*CommitMeta
	HTMLURL    string        `json:"html_url"`
	RepoCommit *RepoCommit   `json:"commit"`
	Author     *User         `json:"author"`
	Committer  *User         `json:"committer"`
	Parents    []*CommitMeta `json:"parents"`
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	// swagger:strfmt date-time
	Author time.Time `json:"author"`
	// swagger:strfmt date-time
	Committer time.Time `json:"committer"`
}
