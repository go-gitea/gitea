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
	GetRepoInfo(ctx context.Context) (*Repository, error)
	GetTopics(ctx context.Context) ([]string, error)
	GetMilestones(ctx context.Context) ([]*Milestone, error)
	GetReleases(ctx context.Context) ([]*Release, error)
	GetLabels(ctx context.Context) ([]*Label, error)
	GetIssues(ctx context.Context, page, perPage int) ([]*Issue, bool, error)
	GetComments(ctx context.Context, commentable Commentable) ([]*Comment, bool, error)
	GetAllComments(ctx context.Context, page, perPage int) ([]*Comment, bool, error)
	SupportGetRepoComments() bool
	GetPullRequests(ctx context.Context, page, perPage int) ([]*PullRequest, bool, error)
	GetReviews(ctx context.Context, reviewable Reviewable) ([]*Review, error)
	FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error)
}

// DownloaderFactory defines an interface to match a downloader implementation and create a downloader
type DownloaderFactory interface {
	New(ctx context.Context, opts MigrateOptions) (Downloader, error)
	GitServiceType() structs.GitServiceType
}

// DownloaderContext has opaque information only relevant to a given downloader
type DownloaderContext any
