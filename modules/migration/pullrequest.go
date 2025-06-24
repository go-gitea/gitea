// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/git"
)

// PullRequest defines a standard pull request information
type PullRequest struct {
	Number         int64
	Title          string
	PosterName     string `yaml:"poster_name"`
	PosterID       int64  `yaml:"poster_id"`
	PosterEmail    string `yaml:"poster_email"`
	Content        string
	Milestone      string
	State          string
	Created        time.Time
	Updated        time.Time
	Closed         *time.Time
	Labels         []*Label
	PatchURL       string `yaml:"patch_url"` // SECURITY: This must be safe to download directly from
	Merged         bool
	MergedTime     *time.Time `yaml:"merged_time"`
	MergeCommitSHA string     `yaml:"merge_commit_sha"`
	Head           PullRequestBranch
	Base           PullRequestBranch
	Assignees      []string
	IsLocked       bool `yaml:"is_locked"`
	Reactions      []*Reaction
	ForeignIndex   int64
	Context        DownloaderContext `yaml:"-"`
	EnsuredSafe    bool              `yaml:"ensured_safe"`
	IsDraft        bool              `yaml:"is_draft"`
}

func (p *PullRequest) GetLocalIndex() int64          { return p.Number }
func (p *PullRequest) GetForeignIndex() int64        { return p.ForeignIndex }
func (p *PullRequest) GetContext() DownloaderContext { return p.Context }

// IsForkPullRequest returns true if the pull request from a forked repository but not the same repository
func (p *PullRequest) IsForkPullRequest() bool {
	return p.Head.RepoFullName() != p.Base.RepoFullName()
}

// GetGitRefName returns pull request relative path to head
func (p PullRequest) GetGitRefName() string {
	return fmt.Sprintf("%s%d/head", git.PullPrefix, p.Number)
}

// PullRequestBranch represents a pull request branch
type PullRequestBranch struct {
	CloneURL  string `yaml:"clone_url"` // SECURITY: This must be safe to download from
	Ref       string // SECURITY: this must be a git.IsValidRefPattern
	SHA       string // SECURITY: this must be a git.IsValidSHAPattern
	RepoName  string `yaml:"repo_name"`
	OwnerName string `yaml:"owner_name"`
}

// RepoFullName returns pull request repo full name
func (p PullRequestBranch) RepoFullName() string {
	return fmt.Sprintf("%s/%s", p.OwnerName, p.RepoName)
}

// GetExternalName ExternalUserMigrated interface
func (p *PullRequest) GetExternalName() string { return p.PosterName }

// ExternalID ExternalUserMigrated interface
func (p *PullRequest) GetExternalID() int64 { return p.PosterID }
