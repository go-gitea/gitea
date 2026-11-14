// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// FileBlame is the blame of a file at a commit
type FileBlame struct {
	// CommitID is the commit the file was blamed at
	CommitID string `json:"commit_id"`
	// Path is the path of the file
	Path string `json:"path"`
	// UsesIgnoreRevs is true if commits listed in .git-blame-ignore-revs were skipped
	UsesIgnoreRevs bool `json:"uses_ignore_revs"`
	// Ranges are the file's lines in order, grouped by the commit that last changed them
	Ranges []*BlameRange `json:"ranges"`
}

// BlameRange is a run of consecutive lines last changed by the same commit
type BlameRange struct {
	// StartLine is the 1-based number of the range's first line
	StartLine int `json:"start_line"`
	// EndLine is the 1-based number of the range's last line
	EndLine int          `json:"end_line"`
	Commit  *BlameCommit `json:"commit"`
	Lines   []string     `json:"lines"`
}

// BlameCommit is the commit that last changed a range of lines
type BlameCommit struct {
	SHA       string      `json:"sha"`
	URL       string      `json:"url"`
	Author    *CommitUser `json:"author"`
	Committer *CommitUser `json:"committer"`
	// Summary is the first line of the commit message
	Summary string `json:"summary"`
}
