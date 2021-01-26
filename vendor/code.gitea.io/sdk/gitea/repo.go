// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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

// RepoType represent repo type
type RepoType string

const (
	// RepoTypeNone dont specify a type
	RepoTypeNone RepoType = ""
	// RepoTypeSource is the default repo type
	RepoTypeSource RepoType = "source"
	// RepoTypeFork is a repo witch was forked from an other one
	RepoTypeFork RepoType = "fork"
	// RepoTypeMirror represents an mirror repo
	RepoTypeMirror RepoType = "mirror"
)

// ListReposOptions options for listing repositories
type ListReposOptions struct {
	ListOptions
}

// ListMyRepos lists all repositories for the authenticated user that has access to.
func (c *Client) ListMyRepos(opt ListReposOptions) ([]*Repository, *Response, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/repos?%s", opt.getURLQuery().Encode()), nil, nil, &repos)
	return repos, resp, err
}

// ListUserRepos list all repositories of one user by user's name
func (c *Client) ListUserRepos(user string, opt ListReposOptions) ([]*Repository, *Response, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/users/%s/repos?%s", user, opt.getURLQuery().Encode()), nil, nil, &repos)
	return repos, resp, err
}

// ListOrgReposOptions options for a organization's repositories
type ListOrgReposOptions struct {
	ListOptions
}

// ListOrgRepos list all repositories of one organization by organization's name
func (c *Client) ListOrgRepos(org string, opt ListOrgReposOptions) ([]*Repository, *Response, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/repos?%s", org, opt.getURLQuery().Encode()), nil, nil, &repos)
	return repos, resp, err
}

// SearchRepoOptions options for searching repositories
type SearchRepoOptions struct {
	ListOptions

	// The keyword to query
	Keyword string
	// Limit search to repositories with keyword as topic
	KeywordIsTopic bool
	// Include search of keyword within repository description
	KeywordInDescription bool

	/*
		User Filter
	*/

	// Repo Owner
	OwnerID int64
	// Stared By UserID
	StarredByUserID int64

	/*
		Repo Attributes
	*/

	// pubic, private or all repositories (defaults to all)
	IsPrivate *bool
	// archived, non-archived or all repositories (defaults to all)
	IsArchived *bool
	// Exclude template repos from search
	ExcludeTemplate bool
	// Filter by "fork", "source", "mirror"
	Type RepoType

	/*
		Sort Filters
	*/

	// sort repos by attribute. Supported values are "alpha", "created", "updated", "size", and "id". Default is "alpha"
	Sort string
	// sort order, either "asc" (ascending) or "desc" (descending). Default is "asc", ignored if "sort" is not specified.
	Order string
	// Repo owner to prioritize in the results
	PrioritizedByOwnerID int64

	/*
		Cover EdgeCases
	*/
	// if set all other options are ignored and this string is used as query
	RawQuery string
}

// QueryEncode turns options into querystring argument
func (opt *SearchRepoOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Keyword != "" {
		query.Add("q", opt.Keyword)
	}
	if opt.KeywordIsTopic {
		query.Add("topic", "true")
	}
	if opt.KeywordInDescription {
		query.Add("includeDesc", "true")
	}

	// User Filter
	if opt.OwnerID > 0 {
		query.Add("uid", fmt.Sprintf("%d", opt.OwnerID))
		query.Add("exclusive", "true")
	}
	if opt.StarredByUserID > 0 {
		query.Add("starredBy", fmt.Sprintf("%d", opt.StarredByUserID))
	}

	// Repo Attributes
	if opt.IsPrivate != nil {
		query.Add("is_private", fmt.Sprintf("%v", opt.IsPrivate))
	}
	if opt.IsArchived != nil {
		query.Add("archived", fmt.Sprintf("%v", opt.IsArchived))
	}
	if opt.ExcludeTemplate {
		query.Add("template", "false")
	}
	if len(opt.Type) != 0 {
		query.Add("mode", string(opt.Type))
	}

	// Sort Filters
	if opt.Sort != "" {
		query.Add("sort", opt.Sort)
	}
	if opt.PrioritizedByOwnerID > 0 {
		query.Add("priority_owner_id", fmt.Sprintf("%d", opt.PrioritizedByOwnerID))
	}
	if opt.Order != "" {
		query.Add("order", opt.Order)
	}

	return query.Encode()
}

type searchRepoResponse struct {
	Repos []*Repository `json:"data"`
}

// SearchRepos searches for repositories matching the given filters
func (c *Client) SearchRepos(opt SearchRepoOptions) ([]*Repository, *Response, error) {
	opt.setDefaults()
	repos := new(searchRepoResponse)

	link, _ := url.Parse("/repos/search")

	if len(opt.RawQuery) != 0 {
		link.RawQuery = opt.RawQuery
	} else {
		link.RawQuery = opt.QueryEncode()
		// IsPrivate only works on gitea >= 1.12.0
		if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil && opt.IsPrivate != nil {
			if *opt.IsPrivate {
				// private repos only not supported on gitea <= 1.11.x
				return nil, nil, err
			}
			link.Query().Add("private", "false")
		}
	}

	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &repos)
	return repos.Repos, resp, err
}

// CreateRepoOption options when creating repository
type CreateRepoOption struct {
	// Name of the repository to create
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

// Validate the CreateRepoOption struct
func (opt CreateRepoOption) Validate() error {
	if len(strings.TrimSpace(opt.Name)) == 0 {
		return fmt.Errorf("name is empty")
	}
	return nil
}

// CreateRepo creates a repository for authenticated user.
func (c *Client) CreateRepo(opt CreateRepoOption) (*Repository, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse("POST", "/user/repos", jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// CreateOrgRepo creates an organization repository for authenticated user.
func (c *Client) CreateOrgRepo(org string, opt CreateRepoOption) (*Repository, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/org/%s/repos", org), jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// GetRepo returns information of a repository of given owner.
func (c *Client) GetRepo(owner, reponame string) (*Repository, *Response, error) {
	repo := new(Repository)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s", owner, reponame), nil, nil, repo)
	return repo, resp, err
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
func (c *Client) EditRepo(owner, reponame string, opt EditRepoOption) (*Repository, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s", owner, reponame), jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// DeleteRepo deletes a repository of user or organization.
func (c *Client) DeleteRepo(owner, repo string) (*Response, error) {
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
	return resp, err
}

// MirrorSync adds a mirrored repository to the mirror sync queue.
func (c *Client) MirrorSync(owner, repo string) (*Response, error) {
	_, resp, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/mirror-sync", owner, repo), nil, nil)
	return resp, err
}

// GetRepoLanguages return language stats of a repo
func (c *Client) GetRepoLanguages(owner, repo string) (map[string]int64, *Response, error) {
	langMap := make(map[string]int64)

	data, resp, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/languages", owner, repo), jsonHeader, nil)
	if err != nil {
		return nil, resp, err
	}
	if err = json.Unmarshal(data, &langMap); err != nil {
		return nil, resp, err
	}
	return langMap, resp, nil
}

// ArchiveType represent supported archive formats by gitea
type ArchiveType string

const (
	// ZipArchive represent zip format
	ZipArchive ArchiveType = ".zip"
	// TarGZArchive represent tar.gz format
	TarGZArchive ArchiveType = ".tar.gz"
)

// GetArchive get an archive of a repository by git reference
// e.g.: ref -> master, 70b7c74b33, v1.2.1, ...
func (c *Client) GetArchive(owner, repo, ref string, ext ArchiveType) ([]byte, *Response, error) {
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/archive/%s%s", owner, repo, url.PathEscape(ref), ext), nil, nil)
}
