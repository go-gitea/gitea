// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import "time"

// Tag represents a repository tag
type Tag struct {
	Name       string      `json:"name"`
	Message    string      `json:"message"`
	ID         string      `json:"id"`
	Commit     *CommitMeta `json:"commit"`
	ZipballURL string      `json:"zipball_url"`
	TarballURL string      `json:"tarball_url"`
}

// AnnotatedTag represents an annotated tag
type AnnotatedTag struct {
	Tag          string                     `json:"tag"`
	SHA          string                     `json:"sha"`
	URL          string                     `json:"url"`
	Message      string                     `json:"message"`
	Tagger       *CommitUser                `json:"tagger"`
	Object       *AnnotatedTagObject        `json:"object"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// AnnotatedTagObject contains meta information of the tag object
type AnnotatedTagObject struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
}

// CreateTagOption options when creating a tag
type CreateTagOption struct {
	// required: true
	TagName string `json:"tag_name" binding:"Required"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

// TagProtection represents a tag protection
type TagProtection struct {
	ID                 int64    `json:"id"`
	NamePattern        string   `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateTagProtectionOption options for creating a tag protection
type CreateTagProtectionOption struct {
	NamePattern        string   `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}

// EditTagProtectionOption options for editing a tag protection
type EditTagProtectionOption struct {
	NamePattern        *string  `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}
