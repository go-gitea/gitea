// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"time"
)

// PullRequest defines a standard pull request information
type PullRequest struct {
	Number         int64
	Title          string
	PosterName     string
	PosterID       int64
	PosterEmail    string
	Content        string
	Milestone      string
	State          string
	Created        time.Time
	Updated        time.Time
	Closed         *time.Time
	Labels         []*Label
	PatchURL       string
	Merged         bool
	MergedTime     *time.Time
	MergeCommitSHA string
	Head           PullRequestBranch
	Base           PullRequestBranch
	Assignee       string
	Assignees      []string
	IsLocked       bool
}

// IsForkPullRequest returns true if the pull request from a forked repository but not the same repository
func (p *PullRequest) IsForkPullRequest() bool {
	return p.Head.RepoPath() != p.Base.RepoPath()
}

// PullRequestBranch represents a pull request branch
type PullRequestBranch struct {
	CloneURL  string
	Ref       string
	SHA       string
	RepoName  string
	OwnerName string
}

// RepoPath returns pull request repo path
func (p PullRequestBranch) RepoPath() string {
	return fmt.Sprintf("%s/%s", p.OwnerName, p.RepoName)
}
