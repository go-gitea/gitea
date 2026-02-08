// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// ActionsTokenPermissions represents the permissions configuration for Actions job tokens
// swagger:model
type ActionsTokenPermissions struct {
	// Default permission level for job tokens (read, write, or none)
	DefaultPermissions string `json:"default_permissions"`
	// Permission for repository contents
	ContentsPermission string `json:"contents_permission,omitempty"`
	// Permission for issues
	IssuesPermission string `json:"issues_permission,omitempty"`
	// Permission for pull requests
	PullRequestsPermission string `json:"pull_requests_permission,omitempty"`
	// Permission for packages
	PackagesPermission string `json:"packages_permission,omitempty"`
	// Permission for metadata (always at least read)
	MetadataPermission string `json:"metadata_permission,omitempty"`
	// Permission for actions
	ActionsPermission string `json:"actions_permission,omitempty"`
	// Permission for organization
	OrganizationPermission string `json:"organization_permission,omitempty"`
	// Permission for notifications
	NotificationPermission string `json:"notification_permission,omitempty"`
}
