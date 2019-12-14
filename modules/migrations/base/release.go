// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// ReleaseAsset represents a release asset
type ReleaseAsset struct {
	URL           string
	Name          string
	ContentType   *string
	Size          *int
	DownloadCount *int
	Created       time.Time
	Updated       time.Time
}

// Release represents a release
type Release struct {
	TagName         string
	TargetCommitish string
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
	PublisherID     int64
	PublisherName   string
	PublisherEmail  string
	Assets          []ReleaseAsset
	Created         time.Time
	Published       time.Time
}
