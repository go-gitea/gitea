// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"time"

	"gitea.dev/modules/structs"
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
	// SupportSyncing reports whether the downloader can fetch what changed since a time, to keep a migrated repository in sync
	SupportSyncing() bool
	// GetNewIssues returns issues updated after the given time, paginated
	GetNewIssues(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*Issue, bool, error)
	// GetNewPullRequests returns pull requests updated after the given time, paginated
	GetNewPullRequests(ctx context.Context, page, perPage int, updatedAfter time.Time) ([]*PullRequest, bool, error)
	// GetNewComments returns an issue's or pull request's comments updated after the given time
	GetNewComments(ctx context.Context, commentable Commentable, updatedAfter time.Time) ([]*Comment, bool, error)
	// GetNewReviews returns a pull request's reviews updated after the given time
	GetNewReviews(ctx context.Context, reviewable Reviewable, updatedAfter time.Time) ([]*Review, error)
}

// DownloaderFactory defines an interface to match a downloader implementation and create a downloader
type DownloaderFactory interface {
	New(ctx context.Context, opts MigrateOptions) (Downloader, error)
	GitServiceType() structs.GitServiceType
}

// DownloaderContext has opaque information only relevant to a given downloader
type DownloaderContext any
