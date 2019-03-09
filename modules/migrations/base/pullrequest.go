// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// PullRequest defines a standard pull request information
type PullRequest struct {
	Number      int64
	Title       string
	PosterName  string
	PosterEmail string
	Content     string
	Milestone   string
	State       string
	Created     time.Time
	Labels      []*Label
	PatchURL    string
	Merged      bool
	Head        PullRequestBranch
	Base        PullRequestBranch
	Assignee    string
	Assignees   []string
}

// PullRequestBranch
type PullRequestBranch struct {
	Ref       string
	SHA       string
	RepoName  string
	OwnerName string
}
