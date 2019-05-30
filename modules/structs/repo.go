// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Permission represents a set of permissions
type Permission struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// Repository represents a repository
type Repository struct {
	ID            int64       `json:"id"`
	Owner         *User       `json:"owner"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	Description   string      `json:"description"`
	Empty         bool        `json:"empty"`
	Private       bool        `json:"private"`
	Fork          bool        `json:"fork"`
	Parent        *Repository `json:"parent"`
	Mirror        bool        `json:"mirror"`
	Size          int         `json:"size"`
	HTMLURL       string      `json:"html_url"`
	SSHURL        string      `json:"ssh_url"`
	CloneURL      string      `json:"clone_url"`
	Website       string      `json:"website"`
	Stars         int         `json:"stars_count"`
	Forks         int         `json:"forks_count"`
	Watchers      int         `json:"watchers_count"`
	OpenIssues    int         `json:"open_issues_count"`
	DefaultBranch string      `json:"default_branch"`
	Archived      bool        `json:"archived"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated     time.Time   `json:"updated_at"`
	Permissions *Permission `json:"permissions,omitempty"`
	AvatarURL   string      `json:"avatar_url"`
}

// CreateRepoOption options when creating repository
// swagger:model
type CreateRepoOption struct {
	// Name of the repository to create
	//
	// required: true
	// unique: true
	Name string `json:"name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	// Description of the repository to create
	Description string `json:"description" binding:"MaxSize(255)"`
	// Whether the repository is private
	Private bool `json:"private"`
	// Whether the repository should be auto-intialized?
	AutoInit bool `json:"auto_init"`
	// Gitignores to use
	Gitignores string `json:"gitignores"`
	// License to use
	License string `json:"license"`
	// Readme of the repository to create
	Readme string `json:"readme"`
}

// EditRepoOption options when editing a repository's properties
// swagger:model
type EditRepoOption struct {
	// Name of the repository
	//
	// required: true
	// unique: true
	Name *string `json:"name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	// A short description of the repository.
	Description *string `json:"description,omitempty" binding:"MaxSize(255)"`
	// A URL with more information about the repository.
	Website *string `json:"website,omitempty" binding:"MaxSize(255)"`
	// Either `true` to make the repository private or `false` to make it public.
	// Note: You will get a 422 error if the organization restricts changing repository visibility to organization
	// owners and a non-owner tries to change the value of private.
	Private *bool `json:"private,omitempty"`
	// Either `true` to enable issues for this repository or `false` to disable them.
	EnableIssues *bool `json:"enable_issues,omitempty"`
	// Either `true` to enable the wiki for this repository or `false` to disable it.
	EnableWiki *bool `json:"enable_wiki,omitempty"`
	// Updates the default branch for this repository.
	DefaultBranch *string `json:"default_branch,omitempty"`
	// Either `true` to allow pull requests, or `false` to prevent pull request.
	EnablePullRequests *bool `json:"enable_pull_requests,omitempty"`
	// Either `true` to ignore whitepace for conflicts, or `false` to not ignore whitespace. `enabled_pull_requests` must be `true`.
	IgnoreWhitespaceConflicts *bool `json:"ignore_whitespace,omitempty"`
	// Either `true` to allow merging pull requests with a merge commit, or `false` to prevent merging pull requests with merge commits. `enabled_pull_requests` must be `true`.
	AllowMerge *bool `json:"allow_merge_commits,omitempty"`
	// Either `true` to allow rebase-merging pull requests, or `false` to prevent rebase-merging. `enabled_pull_requests` must be `true`.
	AllowRebase *bool `json:"allow_rebase,omitempty"`
	// Either `true` to allow rebase with explicit merge commits (--no-ff), or `false` to prevent rebase with explicit merge commits. `enabled_pull_requests` must be `true`.
	AllowRebaseMerge *bool `json:"allow_rebase_explicit,omitempty"`
	// Either `true` to allow squash-merging pull requests, or `false` to prevent squash-merging. `enabled_pull_requests` must be `true`.
	AllowSquashMerge *bool `json:"allow_squash_merge,omitempty"`
	// `true` to archive this repository. Note: You cannot unarchive repositories through the API.
	Archived *bool `json:"archived,omitempty"`
}

// MigrateRepoOption options for migrating a repository from an external service
type MigrateRepoOption struct {
	// required: true
	CloneAddr    string `json:"clone_addr" binding:"Required"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
	// required: true
	UID int `json:"uid" binding:"Required"`
	// required: true
	RepoName    string `json:"repo_name" binding:"Required"`
	Mirror      bool   `json:"mirror"`
	Private     bool   `json:"private"`
	Description string `json:"description"`
}
