// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"io"
	"time"
)

// Attachment is a file uploaded to a pull request, an issue or a comment of the source forge
type Attachment struct {
	Name    string
	Size    *int // nil if unknown
	Created time.Time

	// Link is how the text of the pull request or comment refers to the file. It is replaced by the link of the
	// stored attachment, so that images and links keep working.
	Link string `yaml:"link,omitempty"`

	DownloadURL *string `yaml:"download_url"` // SECURITY: It is the responsibility of downloader to make sure this is safe
	// if DownloadURL is nil, the function should be invoked
	DownloadFunc func() (io.ReadCloser, error) `yaml:"-"` // SECURITY: It is the responsibility of downloader to make sure this is safe
}
