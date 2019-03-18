// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

type ReleaseAsset struct {
	URL           string
	Name          string
	ContentType   *string
	Size          *int
	DownloadCount *int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Release struct {
	TagName         string
	TargetCommitish string
	Name            string
	Body            string
	Draft           bool
	Prerelease      bool
	Assets          []ReleaseAsset

	CreatedAt   time.Time
	PublishedAt time.Time
}
