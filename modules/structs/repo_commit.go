// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Identity for a person's identity like an author or committer
type Identity struct {
	// Name is the person's name
	Name string `json:"name" binding:"MaxSize(100)"`
	// swagger:strfmt email
	// Email is the person's email address
	Email string `json:"email" binding:"MaxSize(254)"`
}

// CommitMeta contains meta information of a commit in terms of API.
type CommitMeta struct {
	// URL is the API URL for the commit
	URL string `json:"url"`
	// SHA is the commit SHA hash
	SHA string `json:"sha"`
	// swagger:strfmt date-time
	// Created is the time when the commit was created
	Created time.Time `json:"created"`
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Identity
	// Date is the commit date in string format
	Date string `json:"date"`
}

// RepoCommit contains information of a commit in the context of a repository.
type RepoCommit struct {
	// URL is the API URL for the commit
	URL string `json:"url"`
	// Author contains the commit author information
	Author *CommitUser `json:"author"`
	// Committer contains the commit committer information
	Committer *CommitUser `json:"committer"`
	// Message is the commit message
	Message string `json:"message"`
	// Tree contains the tree information for the commit
	Tree *CommitMeta `json:"tree"`
	// Verification contains commit signature verification information
	Verification *PayloadCommitVerification `json:"verification"`
}

// CommitStats is statistics for a RepoCommit
type CommitStats struct {
	// Total is the total number of lines changed
	Total int `json:"total"`
	// Additions is the number of lines added
	Additions int `json:"additions"`
	// Deletions is the number of lines deleted
	Deletions int `json:"deletions"`
}

// Commit contains information generated from a Git commit.
type Commit struct {
	*CommitMeta
	// HTMLURL is the web URL for viewing the commit
	HTMLURL string `json:"html_url"`
	// RepoCommit contains the commit information
	RepoCommit *RepoCommit `json:"commit"`
	// Author is the GitHub/Gitea user who authored the commit
	Author *User `json:"author"`
	// Committer is the GitHub/Gitea user who committed the commit
	Committer *User `json:"committer"`
	// Parents contains the parent commit information
	Parents []*CommitMeta `json:"parents"`
	// Files contains information about files affected by the commit
	Files []*CommitAffectedFiles `json:"files"`
	// Stats contains statistics about the commit changes
	Stats *CommitStats `json:"stats"`
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	// swagger:strfmt date-time
	// Author is the author date for the commit
	Author time.Time `json:"author"`
	// swagger:strfmt date-time
	// Committer is the committer date for the commit
	Committer time.Time `json:"committer"`
}

// CommitAffectedFiles store information about files affected by the commit
type CommitAffectedFiles struct {
	// Filename is the path of the affected file
	Filename string `json:"filename"`
	// Status indicates how the file was affected (added, modified, deleted)
	Status string `json:"status"`
}
