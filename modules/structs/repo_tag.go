// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// Tag represents a repository tag
type Tag struct {
	Name                 string                   `json:"name"`
	Message              string                   `json:"message"`
	ID                   string                   `json:"id"`
	Commit               *CommitMeta              `json:"commit"`
	ZipballURL           string                   `json:"zipball_url"`
	TarballURL           string                   `json:"tarball_url"`
	ArchiveDownloadCount *TagArchiveDownloadCount `json:"archive_download_count"`
}

// AnnotatedTag represents an annotated tag
type AnnotatedTag struct {
	Tag                  string                     `json:"tag"`
	SHA                  string                     `json:"sha"`
	URL                  string                     `json:"url"`
	Message              string                     `json:"message"`
	Tagger               *CommitUser                `json:"tagger"`
	Object               *AnnotatedTagObject        `json:"object"`
	Verification         *PayloadCommitVerification `json:"verification"`
	ArchiveDownloadCount *TagArchiveDownloadCount   `json:"archive_download_count"`
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

// TagArchiveDownloadCount counts how many times a archive was downloaded
type TagArchiveDownloadCount struct {
	Zip   int64 `json:"zip"`
	TarGz int64 `json:"tar_gz"`
}
