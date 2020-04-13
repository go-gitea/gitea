// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	TarURL       string `json:"tarball_url"`
	ZipURL       string `json:"zipball_url"`
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
