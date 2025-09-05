// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Release represents a repository release
type Release struct {
	// The unique identifier of the release
	ID int64 `json:"id"`
	// The name of the git tag associated with the release
	TagName string `json:"tag_name"`
	// The target commitish for the release
	Target string `json:"target_commitish"`
	// The display title of the release
	Title string `json:"name"`
	// The release notes or description
	Note string `json:"body"`
	// The API URL of the release
	URL string `json:"url"`
	// The HTML URL to view the release
	HTMLURL string `json:"html_url"`
	// The URL to download the tarball archive
	TarURL string `json:"tarball_url"`
	// The URL to download the zip archive
	ZipURL string `json:"zipball_url"`
	// The URL template for uploading release assets
	UploadURL string `json:"upload_url"`
	// Whether the release is a draft
	IsDraft bool `json:"draft"`
	// Whether the release is a prerelease
	IsPrerelease bool `json:"prerelease"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	PublishedAt time.Time `json:"published_at"`
	// The user who published the release
	Publisher *User `json:"author"`
	// The files attached to the release
	Attachments []*Attachment `json:"assets"`
}

// CreateReleaseOption options when creating a release
type CreateReleaseOption struct {
	// required: true
	TagName string `json:"tag_name" binding:"Required"`
	// The message for the git tag
	TagMessage string `json:"tag_message"`
	// The target commitish for the release
	Target string `json:"target_commitish"`
	// The display title of the release
	Title string `json:"name"`
	// The release notes or description
	Note string `json:"body"`
	// Whether to create the release as a draft
	IsDraft bool `json:"draft"`
	// Whether to mark the release as a prerelease
	IsPrerelease bool `json:"prerelease"`
}

// EditReleaseOption options when editing a release
type EditReleaseOption struct {
	// The new name of the git tag
	TagName string `json:"tag_name"`
	// The new target commitish for the release
	Target string `json:"target_commitish"`
	// The new display title of the release
	Title string `json:"name"`
	// The new release notes or description
	Note string `json:"body"`
	// Whether to change the draft status
	IsDraft *bool `json:"draft"`
	// Whether to change the prerelease status
	IsPrerelease *bool `json:"prerelease"`
}
