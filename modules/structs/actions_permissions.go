// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ActionsPermissions represents Actions token permissions for a repository
// swagger:model
type ActionsPermissions struct {
	PermissionMode    int  `json:"permission_mode"`
	ActionsRead       bool `json:"actions_read"`
	ActionsWrite      bool `json:"actions_write"`
	ContentsRead      bool `json:"contents_read"`
	ContentsWrite     bool `json:"contents_write"`
	IssuesRead        bool `json:"issues_read"`
	IssuesWrite       bool `json:"issues_write"`
	PackagesRead      bool `json:"packages_read"`
	PackagesWrite     bool `json:"packages_write"`
	PullRequestsRead  bool `json:"pull_requests_read"`
	PullRequestsWrite bool `json:"pull_requests_write"`
	MetadataRead      bool `json:"metadata_read"`
}

// OrgActionsPermissions represents organization-level Actions token permissions
// swagger:model
type OrgActionsPermissions struct {
	PermissionMode    int  `json:"permission_mode"`
	AllowRepoOverride bool `json:"allow_repo_override"`
	ActionsRead       bool `json:"actions_read"`
	ActionsWrite      bool `json:"actions_write"`
	ContentsRead      bool `json:"contents_read"`
	ContentsWrite     bool `json:"contents_write"`
	IssuesRead        bool `json:"issues_read"`
	IssuesWrite       bool `json:"issues_write"`
	PackagesRead      bool `json:"packages_read"`
	PackagesWrite     bool `json:"packages_write"`
	PullRequestsRead  bool `json:"pull_requests_read"`
	PullRequestsWrite bool `json:"pull_requests_write"`
	MetadataRead      bool `json:"metadata_read"`
}

// CrossRepoAccessRule represents a cross-repository access rule
// swagger:model
type CrossRepoAccessRule struct {
	ID           int64 `json:"id"`
	OrgID        int64 `json:"org_id"`
	SourceRepoID int64 `json:"source_repo_id"`
	TargetRepoID int64 `json:"target_repo_id"`
	AccessLevel  int   `json:"access_level"`
}
