// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"time"
)

// Release represents a repository release
type Release struct {
	ID           int64  `json:"id"`
	TagName      string `json:"tag_name"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	URL          string `json:"url"`
	HTMLURL      string `json:"html_url"`
	TarURL       string `json:"tarball_url"`
	ZipURL       string `json:"zipball_url"`
	UploadURL    string `json:"upload_url"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
	// swagger:strfmt date-time
	CreatedAt time.Time `json:"created_at"`
	// swagger:strfmt date-time
	PublishedAt time.Time     `json:"published_at"`
	Publisher   *User         `json:"author"`
	Attachments []*Attachment `json:"assets"`
}

// CreateReleaseOption options when creating a release
type CreateReleaseOption struct {
	// required: true
	TagName      string `json:"tag_name" binding:"Required"`
	TagMessage   string `json:"tag_message"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
}

// EditReleaseOption options when editing a release
type EditReleaseOption struct {
	TagName      string `json:"tag_name"`
	Target       string `json:"target_commitish"`
	Title        string `json:"name"`
	Note         string `json:"body"`
	IsDraft      *bool  `json:"draft"`
	IsPrerelease *bool  `json:"prerelease"`
}
