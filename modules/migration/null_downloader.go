// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"net/url"
)

// NullDownloader implements a blank downloader
type NullDownloader struct{}

var _ Downloader = &NullDownloader{}

// GetRepoInfo returns a repository information
func (n NullDownloader) GetRepoInfo(_ context.Context) (*Repository, error) {
	return nil, ErrNotSupported{Entity: "RepoInfo"}
}

// GetTopics return repository topics
func (n NullDownloader) GetTopics(_ context.Context) ([]string, error) {
	return nil, ErrNotSupported{Entity: "Topics"}
}

// GetMilestones returns milestones
func (n NullDownloader) GetMilestones(_ context.Context) ([]*Milestone, error) {
	return nil, ErrNotSupported{Entity: "Milestones"}
}

// GetReleases returns releases
func (n NullDownloader) GetReleases(_ context.Context) ([]*Release, error) {
	return nil, ErrNotSupported{Entity: "Releases"}
}

// GetLabels returns labels
func (n NullDownloader) GetLabels(_ context.Context) ([]*Label, error) {
	return nil, ErrNotSupported{Entity: "Labels"}
}

// GetIssues returns issues according start and limit
func (n NullDownloader) GetIssues(_ context.Context, page, perPage int) ([]*Issue, bool, error) {
	return nil, false, ErrNotSupported{Entity: "Issues"}
}

// GetComments returns comments of an issue or PR
func (n NullDownloader) GetComments(_ context.Context, commentable Commentable) ([]*Comment, bool, error) {
	return nil, false, ErrNotSupported{Entity: "Comments"}
}

// GetAllComments returns paginated comments
func (n NullDownloader) GetAllComments(_ context.Context, page, perPage int) ([]*Comment, bool, error) {
	return nil, false, ErrNotSupported{Entity: "AllComments"}
}

// GetPullRequests returns pull requests according page and perPage
func (n NullDownloader) GetPullRequests(_ context.Context, page, perPage int) ([]*PullRequest, bool, error) {
	return nil, false, ErrNotSupported{Entity: "PullRequests"}
}

// GetReviews returns pull requests review
func (n NullDownloader) GetReviews(_ context.Context, reviewable Reviewable) ([]*Review, error) {
	return nil, ErrNotSupported{Entity: "Reviews"}
}

// FormatCloneURL add authentication into remote URLs
func (n NullDownloader) FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error) {
	if len(opts.AuthToken) > 0 || len(opts.AuthUsername) > 0 {
		u, err := url.Parse(remoteAddr)
		if err != nil {
			return "", err
		}
		u.User = url.UserPassword(opts.AuthUsername, opts.AuthPassword)
		if len(opts.AuthToken) > 0 {
			u.User = url.UserPassword("oauth2", opts.AuthToken)
		}
		return u.String(), nil
	}
	return remoteAddr, nil
}

// SupportGetRepoComments return true if it supports get repo comments
func (n NullDownloader) SupportGetRepoComments() bool {
	return false
}
