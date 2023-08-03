// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"

	"code.gitea.io/gitea/modules/structs"
)

// Downloader downloads the site repo information
type Downloader interface {
	SetContext(context.Context)
	GetRepoInfo() (*Repository, error)
	GetTopics() ([]string, error)
	GetMilestones() ([]*Milestone, error)
	GetReleases() ([]*Release, error)
	GetLabels() ([]*Label, error)
	GetIssues(page, perPage int) ([]*Issue, bool, error)
	GetComments(commentable Commentable) ([]*Comment, bool, error)
	GetAllComments(page, perPage int) ([]*Comment, bool, error)
	SupportGetRepoComments() bool
	GetPullRequests(page, perPage int) ([]*PullRequest, bool, error)
	GetReviews(reviewable Reviewable) ([]*Review, error)
	FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error)
}

// DownloaderFactory defines an interface to match a downloader implementation and create a downloader
type DownloaderFactory interface {
	New(ctx context.Context, opts MigrateOptions) (Downloader, error)
	GitServiceType() structs.GitServiceType
}

// DownloaderContext has opaque information only relevant to a given downloader
type DownloaderContext any
