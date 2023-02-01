// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"io"
	"time"
)

// Asset represents an asset for issue, comment, release
type Asset struct {
	ID            int64
	Name          string
	ContentType   *string `yaml:"content_type"`
	Size          *int
	DownloadCount *int `yaml:"download_count"`
	Created       time.Time
	Updated       time.Time
	DownloadURL   *string `yaml:"download_url"`
	OriginalURL   string
	// if DownloadURL is nil, the function should be invoked
	DownloadFunc func() (io.ReadCloser, error) `yaml:"-"`
}
