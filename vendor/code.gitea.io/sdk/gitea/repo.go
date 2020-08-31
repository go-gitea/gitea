// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
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
	ID                        int64       `json:"id"`
	Owner                     *User       `json:"owner"`
	Name                      string      `json:"name"`
	FullName                  string      `json:"full_name"`
	Description               string      `json:"description"`
	Empty                     bool        `json:"empty"`
	Private                   bool        `json:"private"`
	Fork                      bool        `json:"fork"`
	Parent                    *Repository `json:"parent"`
	Mirror                    bool        `json:"mirror"`
	Size                      int         `json:"size"`
	HTMLURL                   string      `json:"html_url"`
	SSHURL                    string      `json:"ssh_url"`
	CloneURL                  string      `json:"clone_url"`
	OriginalURL               string      `json:"original_url"`
	Website                   string      `json:"website"`
	Stars                     int         `json:"stars_count"`
	Forks                     int         `json:"forks_count"`
	Watchers                  int         `json:"watchers_count"`
	OpenIssues                int         `json:"open_issues_count"`
	DefaultBranch             string      `json:"default_branch"`
	Archived                  bool        `json:"archived"`
	Created                   time.Time   `json:"created_at"`
	Updated                   time.Time   `json:"updated_at"`
	Permissions               *Permission `json:"permissions,omitempty"`
	HasIssues                 bool        `json:"has_issues"`
	HasWiki                   bool        `json:"has_wiki"`
	HasPullRequests           bool        `json:"has_pull_requests"`
	IgnoreWhitespaceConflicts bool        `json:"ignore_whitespace_conflicts"`
	AllowMerge                bool        `json:"allow_merge_commits"`
	AllowRebase               bool        `json:"allow_rebase"`
	AllowRebaseMerge          bool        `json:"allow_rebase_explicit"`
	AllowSquash               bool        `json:"allow_squash_merge"`
	AvatarURL                 string      `json:"avatar_url"`
}

// ListReposOptions options for listing repositories
type ListReposOptions struct {
	ListOptions
}

// ListMyRepos lists all repositories for the authenticated user that has access to.
func (c *Client) ListMyRepos(opt ListReposOptions) ([]*Repository, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/user/repos?%s", opt.getURLQuery().Encode()), nil, nil, &repos)
}

// ListUserRepos list all repositories of one user by user's name
func (c *Client) ListUserRepos(user string, opt ListReposOptions) ([]*Repository, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/users/%s/repos?%s", user, opt.getURLQuery().Encode()), nil, nil, &repos)
}

// ListOrgReposOptions options for a organization's repositories
type ListOrgReposOptions struct {
	ListOptions
}

// ListOrgRepos list all repositories of one organization by organization's name
func (c *Client) ListOrgRepos(org string, opt ListOrgReposOptions) ([]*Repository, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	return repos, c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/repos?%s", org, opt.getURLQuery().Encode()), nil, nil, &repos)
}

// SearchRepoOptions options for searching repositories
type SearchRepoOptions struct {
	ListOptions
	Keyword         string
	Topic           bool
	IncludeDesc     bool
	UID             int64
	PriorityOwnerID int64
	StarredBy       int64
	Private         bool
	Template        bool
	Mode            string
	Exclusive       bool
	Sort            string
}

// QueryEncode turns options into querystring argument
func (opt *SearchRepoOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Keyword != "" {
		query.Add("q", opt.Keyword)
	}

	query.Add("topic", fmt.Sprintf("%t", opt.Topic))
	query.Add("includeDesc", fmt.Sprintf("%t", opt.IncludeDesc))

	if opt.UID > 0 {
		query.Add("uid", fmt.Sprintf("%d", opt.UID))
	}

	if opt.PriorityOwnerID > 0 {
		query.Add("priority_owner_id", fmt.Sprintf("%d", opt.PriorityOwnerID))
	}

	if opt.StarredBy > 0 {
		query.Add("starredBy", fmt.Sprintf("%d", opt.StarredBy))
	}

	query.Add("private", fmt.Sprintf("%t", opt.Private))
	query.Add("template", fmt.Sprintf("%t", opt.Template))

	if opt.Mode != "" {
		query.Add("mode", opt.Mode)
	}

	query.Add("exclusive", fmt.Sprintf("%t", opt.Exclusive))

	if opt.Sort != "" {
		query.Add("sort", opt.Sort)
	}

	return query.Encode()
}

type searchRepoResponse struct {
	Repos []*Repository `json:"data"`
}

// SearchRepos searches for repositories matching the given filters
func (c *Client) SearchRepos(opt SearchRepoOptions) ([]*Repository, error) {
	opt.setDefaults()
	resp := new(searchRepoResponse)

	link, _ := url.Parse("/repos/search")
	link.RawQuery = opt.QueryEncode()

	err := c.getParsedResponse("GET", link.String(), nil, nil, &resp)
	return resp.Repos, err
}

// CreateRepoOption options when creating repository
type CreateRepoOption struct {
	// Name of the repository to create
	//
	Name string `json:"name"`
	// Description of the repository to create
	Description string `json:"description"`
	// Whether the repository is private
	Private bool `json:"private"`
	// Issue Label set to use
	IssueLabels string `json:"issue_labels"`
	// Whether the repository should be auto-intialized?
	AutoInit bool `json:"auto_init"`
	// Gitignores to use
	Gitignores string `json:"gitignores"`
	// License to use
	License string `json:"license"`
	// Readme of the repository to create
	Readme string `json:"readme"`
	// DefaultBranch of the repository (used when initializes and in template)
	DefaultBranch string `json:"default_branch"`
}

// CreateRepo creates a repository for authenticated user.
func (c *Client) CreateRepo(opt CreateRepoOption) (*Repository, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	repo := new(Repository)
	return repo, c.getParsedResponse("POST", "/user/repos", jsonHeader, bytes.NewReader(body), repo)
}

// CreateOrgRepo creates an organization repository for authenticated user.
func (c *Client) CreateOrgRepo(org string, opt CreateRepoOption) (*Repository, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	repo := new(Repository)
	return repo, c.getParsedResponse("POST", fmt.Sprintf("/org/%s/repos", org), jsonHeader, bytes.NewReader(body), repo)
}

// GetRepo returns information of a repository of given owner.
func (c *Client) GetRepo(owner, reponame string) (*Repository, error) {
	repo := new(Repository)
	return repo, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s", owner, reponame), nil, nil, repo)
}

// EditRepoOption options when editing a repository's properties
type EditRepoOption struct {
	// name of the repository
	Name *string `json:"name,omitempty"`
	// a short description of the repository.
	Description *string `json:"description,omitempty"`
	// a URL with more information about the repository.
	Website *string `json:"website,omitempty"`
	// either `true` to make the repository private or `false` to make it public.
	// Note: you will get a 422 error if the organization restricts changing repository visibility to organization
	// owners and a non-owner tries to change the value of private.
	Private *bool `json:"private,omitempty"`
	// either `true` to enable issues for this repository or `false` to disable them.
	HasIssues *bool `json:"has_issues,omitempty"`
	// either `true` to enable the wiki for this repository or `false` to disable it.
	HasWiki *bool `json:"has_wiki,omitempty"`
	// sets the default branch for this repository.
	DefaultBranch *string `json:"default_branch,omitempty"`
	// either `true` to allow pull requests, or `false` to prevent pull request.
	HasPullRequests *bool `json:"has_pull_requests,omitempty"`
	// either `true` to ignore whitespace for conflicts, or `false` to not ignore whitespace. `has_pull_requests` must be `true`.
	IgnoreWhitespaceConflicts *bool `json:"ignore_whitespace_conflicts,omitempty"`
	// either `true` to allow merging pull requests with a merge commit, or `false` to prevent merging pull requests with merge commits. `has_pull_requests` must be `true`.
	AllowMerge *bool `json:"allow_merge_commits,omitempty"`
	// either `true` to allow rebase-merging pull requests, or `false` to prevent rebase-merging. `has_pull_requests` must be `true`.
	AllowRebase *bool `json:"allow_rebase,omitempty"`
	// either `true` to allow rebase with explicit merge commits (--no-ff), or `false` to prevent rebase with explicit merge commits. `has_pull_requests` must be `true`.
	AllowRebaseMerge *bool `json:"allow_rebase_explicit,omitempty"`
	// either `true` to allow squash-merging pull requests, or `false` to prevent squash-merging. `has_pull_requests` must be `true`.
	AllowSquash *bool `json:"allow_squash_merge,omitempty"`
	// set to `true` to archive this repository.
	Archived *bool `json:"archived,omitempty"`
}

// EditRepo edit the properties of a repository
func (c *Client) EditRepo(owner, reponame string, opt EditRepoOption) (*Repository, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	repo := new(Repository)
	return repo, c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s", owner, reponame), jsonHeader, bytes.NewReader(body), repo)
}

// DeleteRepo deletes a repository of user or organization.
func (c *Client) DeleteRepo(owner, repo string) error {
	_, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
	return err
}

// MigrateRepoOption options for migrating a repository from an external service
type MigrateRepoOption struct {
	CloneAddr    string `json:"clone_addr"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
	UID          int    `json:"uid"`
	RepoName     string `json:"repo_name"`
	Mirror       bool   `json:"mirror"`
	Private      bool   `json:"private"`
	Description  string `json:"description"`
}

// MigrateRepo migrates a repository from other Git hosting sources for the
// authenticated user.
//
// To migrate a repository for a organization, the authenticated user must be a
// owner of the specified organization.
func (c *Client) MigrateRepo(opt MigrateRepoOption) (*Repository, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	repo := new(Repository)
	return repo, c.getParsedResponse("POST", "/repos/migrate", jsonHeader, bytes.NewReader(body), repo)
}

// MirrorSync adds a mirrored repository to the mirror sync queue.
func (c *Client) MirrorSync(owner, repo string) error {
	_, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/mirror-sync", owner, repo), nil, nil)
	return err
}
