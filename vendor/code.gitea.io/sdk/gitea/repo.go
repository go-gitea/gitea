// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// InternalTracker represents settings for internal tracker
type InternalTracker struct {
	// Enable time tracking (Built-in issue tracker)
	EnableTimeTracker bool `json:"enable_time_tracker"`
	// Let only contributors track time (Built-in issue tracker)
	AllowOnlyContributorsToTrackTime bool `json:"allow_only_contributors_to_track_time"`
	// Enable dependencies for issues and pull requests (Built-in issue tracker)
	EnableIssueDependencies bool `json:"enable_issue_dependencies"`
}

// ExternalTracker represents settings for external tracker
type ExternalTracker struct {
	// URL of external issue tracker.
	ExternalTrackerURL string `json:"external_tracker_url"`
	// External Issue Tracker URL Format. Use the placeholders {user}, {repo} and {index} for the username, repository name and issue index.
	ExternalTrackerFormat string `json:"external_tracker_format"`
	// External Issue Tracker Number Format, either `numeric` or `alphanumeric`
	ExternalTrackerStyle string `json:"external_tracker_style"`
}

// ExternalWiki represents setting for external wiki
type ExternalWiki struct {
	// URL of external wiki.
	ExternalWikiURL string `json:"external_wiki_url"`
}

// Repository represents a repository
type Repository struct {
	ID                        int64            `json:"id"`
	Owner                     *User            `json:"owner"`
	Name                      string           `json:"name"`
	FullName                  string           `json:"full_name"`
	Description               string           `json:"description"`
	Empty                     bool             `json:"empty"`
	Private                   bool             `json:"private"`
	Fork                      bool             `json:"fork"`
	Template                  bool             `json:"template"`
	Parent                    *Repository      `json:"parent"`
	Mirror                    bool             `json:"mirror"`
	Size                      int              `json:"size"`
	HTMLURL                   string           `json:"html_url"`
	SSHURL                    string           `json:"ssh_url"`
	CloneURL                  string           `json:"clone_url"`
	OriginalURL               string           `json:"original_url"`
	Website                   string           `json:"website"`
	Stars                     int              `json:"stars_count"`
	Forks                     int              `json:"forks_count"`
	Watchers                  int              `json:"watchers_count"`
	OpenIssues                int              `json:"open_issues_count"`
	OpenPulls                 int              `json:"open_pr_counter"`
	Releases                  int              `json:"release_counter"`
	DefaultBranch             string           `json:"default_branch"`
	Archived                  bool             `json:"archived"`
	Created                   time.Time        `json:"created_at"`
	Updated                   time.Time        `json:"updated_at"`
	Permissions               *Permission      `json:"permissions,omitempty"`
	HasIssues                 bool             `json:"has_issues"`
	InternalTracker           *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker           *ExternalTracker `json:"external_tracker,omitempty"`
	HasWiki                   bool             `json:"has_wiki"`
	ExternalWiki              *ExternalWiki    `json:"external_wiki,omitempty"`
	HasPullRequests           bool             `json:"has_pull_requests"`
	HasProjects               bool             `json:"has_projects"`
	IgnoreWhitespaceConflicts bool             `json:"ignore_whitespace_conflicts"`
	AllowMerge                bool             `json:"allow_merge_commits"`
	AllowRebase               bool             `json:"allow_rebase"`
	AllowRebaseMerge          bool             `json:"allow_rebase_explicit"`
	AllowSquash               bool             `json:"allow_squash_merge"`
	AvatarURL                 string           `json:"avatar_url"`
	Internal                  bool             `json:"internal"`
	MirrorInterval            string           `json:"mirror_interval"`
	DefaultMergeStyle         MergeStyle       `json:"default_merge_style"`
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

// TrustModel represent how git signatures are handled in a repository
type TrustModel string

const (
	// TrustModelDefault use TM set by global config
	TrustModelDefault TrustModel = "default"
	// TrustModelCollaborator gpg signature has to be owned by a repo collaborator
	TrustModelCollaborator TrustModel = "collaborator"
	// TrustModelCommitter gpg signature has to match committer
	TrustModelCommitter TrustModel = "committer"
	// TrustModelCollaboratorCommitter gpg signature has to match committer and owned by a repo collaborator
	TrustModelCollaboratorCommitter TrustModel = "collaboratorcommitter"
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
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
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
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
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
		if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil && opt.IsPrivate != nil {
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
	// Whether the repository is template
	Template bool `json:"template"`
	// Gitignores to use
	Gitignores string `json:"gitignores"`
	// License to use
	License string `json:"license"`
	// Readme of the repository to create
	Readme string `json:"readme"`
	// DefaultBranch of the repository (used when initializes and in template)
	DefaultBranch string `json:"default_branch"`
	// TrustModel of the repository
	TrustModel TrustModel `json:"trust_model"`
}

// Validate the CreateRepoOption struct
func (opt CreateRepoOption) Validate(c *Client) error {
	if len(strings.TrimSpace(opt.Name)) == 0 {
		return fmt.Errorf("name is empty")
	}
	if len(opt.Name) > 100 {
		return fmt.Errorf("name has more than 100 chars")
	}
	if len(opt.Description) > 255 {
		return fmt.Errorf("name has more than 255 chars")
	}
	if len(opt.DefaultBranch) > 100 {
		return fmt.Errorf("name has more than 100 chars")
	}
	if len(opt.TrustModel) != 0 {
		if err := c.checkServerVersionGreaterThanOrEqual(version1_13_0); err != nil {
			return err
		}
	}
	return nil
}

// CreateRepo creates a repository for authenticated user.
func (c *Client) CreateRepo(opt CreateRepoOption) (*Repository, *Response, error) {
	if err := opt.Validate(c); err != nil {
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
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(c); err != nil {
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
	if err := escapeValidatePathSegments(&owner, &reponame); err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s", owner, reponame), nil, nil, repo)
	return repo, resp, err
}

// GetRepoByID returns information of a repository by a giver repository ID.
func (c *Client) GetRepoByID(id int64) (*Repository, *Response, error) {
	repo := new(Repository)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repositories/%d", id), nil, nil, repo)
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
	// either `true` to make this repository a template or `false` to make it a normal repository
	Template *bool `json:"template,omitempty"`
	// either `true` to enable issues for this repository or `false` to disable them.
	HasIssues *bool `json:"has_issues,omitempty"`
	// set this structure to configure internal issue tracker (requires has_issues)
	InternalTracker *InternalTracker `json:"internal_tracker,omitempty"`
	// set this structure to use external issue tracker (requires has_issues)
	ExternalTracker *ExternalTracker `json:"external_tracker,omitempty"`
	// either `true` to enable the wiki for this repository or `false` to disable it.
	HasWiki *bool `json:"has_wiki,omitempty"`
	// set this structure to use external wiki instead of internal (requires has_wiki)
	ExternalWiki *ExternalWiki `json:"external_wiki,omitempty"`
	// sets the default branch for this repository.
	DefaultBranch *string `json:"default_branch,omitempty"`
	// either `true` to allow pull requests, or `false` to prevent pull request.
	HasPullRequests *bool `json:"has_pull_requests,omitempty"`
	// either `true` to enable project unit, or `false` to disable them.
	HasProjects *bool `json:"has_projects,omitempty"`
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
	// set to a string like `8h30m0s` to set the mirror interval time
	MirrorInterval *string `json:"mirror_interval,omitempty"`
	// either `true` to allow mark pr as merged manually, or `false` to prevent it. `has_pull_requests` must be `true`.
	AllowManualMerge *bool `json:"allow_manual_merge,omitempty"`
	// either `true` to enable AutodetectManualMerge, or `false` to prevent it. `has_pull_requests` must be `true`, Note: In some special cases, misjudgments can occur.
	AutodetectManualMerge *bool `json:"autodetect_manual_merge,omitempty"`
	// set to a merge style to be used by this repository: "merge", "rebase", "rebase-merge", or "squash". `has_pull_requests` must be `true`.
	DefaultMergeStyle *MergeStyle `json:"default_merge_style,omitempty"`
	// set to `true` to archive this repository.
}

// EditRepo edit the properties of a repository
func (c *Client) EditRepo(owner, reponame string, opt EditRepoOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &reponame); err != nil {
		return nil, nil, err
	}
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
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s", owner, repo), nil, nil)
	return resp, err
}

// MirrorSync adds a mirrored repository to the mirror sync queue.
func (c *Client) MirrorSync(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/mirror-sync", owner, repo), nil, nil)
	return resp, err
}

// GetRepoLanguages return language stats of a repo
func (c *Client) GetRepoLanguages(owner, repo string) (map[string]int64, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
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
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	ref = pathEscapeSegments(ref)
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/archive/%s%s", owner, repo, ref, ext), nil, nil)
}

// GetArchiveReader gets a `git archive` for a particular tree-ish git reference
// such as a branch name (`master`), a commit hash (`70b7c74b33`), a tag
// (`v1.2.1`). The archive is returned as a byte stream in a ReadCloser. It is
// the responsibility of the client to close the reader.
func (c *Client) GetArchiveReader(owner, repo, ref string, ext ArchiveType) (io.ReadCloser, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	ref = pathEscapeSegments(ref)
	resp, err := c.doRequest("GET", fmt.Sprintf("/repos/%s/%s/archive/%s%s", owner, repo, ref, ext), nil, nil)
	if err != nil {
		return nil, resp, err
	}

	if _, err := statusCodeToErr(resp); err != nil {
		return nil, resp, err
	}

	return resp.Body, resp, nil
}
