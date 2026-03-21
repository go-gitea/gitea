// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// Tag represents a repository tag
type Tag struct {
	// The name of the tag
	Name string `json:"name"`
	// The message associated with the tag
	Message string `json:"message"`
	// The ID (SHA) of the tag
	ID string `json:"id"`
	// The commit information associated with this tag
	Commit *CommitMeta `json:"commit"`
	// The URL to download the zipball archive
	ZipballURL string `json:"zipball_url,omitempty"`
	// The URL to download the tarball archive
	TarballURL string `json:"tarball_url,omitempty"`
}

// AnnotatedTag represents an annotated tag
type AnnotatedTag struct {
	// The name of the annotated tag
	Tag string `json:"tag"`
	// The SHA hash of the annotated tag
	SHA string `json:"sha"`
	// The URL to access the annotated tag
	URL string `json:"url"`
	// The message associated with the annotated tag
	Message string `json:"message"`
	// The user who created the annotated tag
	Tagger *CommitUser `json:"tagger"`
	// The object that the annotated tag points to
	Object *AnnotatedTagObject `json:"object"`
	// The verification information for the annotated tag
	Verification *PayloadCommitVerification `json:"verification"`
}

// AnnotatedTagObject contains meta information of the tag object
type AnnotatedTagObject struct {
	// The type of the tagged object (e.g., commit, tree)
	Type string `json:"type"`
	// The URL to access the tagged object
	URL string `json:"url"`
	// The SHA hash of the tagged object
	SHA string `json:"sha"`
}

// CreateTagOption options when creating a tag
type CreateTagOption struct {
	// required: true
	// The name of the tag to create
	TagName string `json:"tag_name" binding:"Required"`
	// The message to associate with the tag
	Message string `json:"message"`
	// The target commit SHA or branch name for the tag
	Target string `json:"target"`
}

// TagProtection represents a tag protection
type TagProtection struct {
	// The unique identifier of the tag protection
	ID int64 `json:"id"`
	// The pattern to match tag names for protection
	NamePattern string `json:"name_pattern"`
	// List of usernames allowed to create/delete protected tags
	WhitelistUsernames []string `json:"whitelist_usernames"`
	// List of team names allowed to create/delete protected tags
	WhitelistTeams []string `json:"whitelist_teams"`
	// swagger:strfmt date-time
	// The date and time when the tag protection was created
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	// The date and time when the tag protection was last updated
	Updated time.Time `json:"updated_at"`
}

// CreateTagProtectionOption options for creating a tag protection
type CreateTagProtectionOption struct {
	// The pattern to match tag names for protection
	NamePattern string `json:"name_pattern"`
	// List of usernames allowed to create/delete protected tags
	WhitelistUsernames []string `json:"whitelist_usernames"`
	// List of team names allowed to create/delete protected tags
	WhitelistTeams []string `json:"whitelist_teams"`
}

// EditTagProtectionOption options for editing a tag protection
type EditTagProtectionOption struct {
	// The pattern to match tag names for protection
	NamePattern *string `json:"name_pattern"`
	// List of usernames allowed to create/delete protected tags
	WhitelistUsernames []string `json:"whitelist_usernames"`
	// List of team names allowed to create/delete protected tags
	WhitelistTeams []string `json:"whitelist_teams"`
}
