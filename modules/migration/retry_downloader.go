// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import (
	"context"
	"time"
)

var _ Downloader = &RetryDownloader{}

// RetryDownloader retry the downloads
type RetryDownloader struct {
	Downloader
	RetryTimes int // the total execute times
	RetryDelay int // time to delay seconds
}

// NewRetryDownloader creates a retry downloader
func NewRetryDownloader(downloader Downloader, retryTimes, retryDelay int) *RetryDownloader {
	return &RetryDownloader{
		Downloader: downloader,
		RetryTimes: retryTimes,
		RetryDelay: retryDelay,
	}
}

func (d *RetryDownloader) retry(ctx context.Context, work func(context.Context) error) error {
	var (
		times = d.RetryTimes
		err   error
	)
	for ; times > 0; times-- {
		if err = work(ctx); err == nil {
			return nil
		}
		if IsErrNotSupported(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * time.Duration(d.RetryDelay)):
		}
	}
	return err
}

// GetRepoInfo returns a repository information with retry
func (d *RetryDownloader) GetRepoInfo(ctx context.Context) (*Repository, error) {
	var (
		repo *Repository
		err  error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		repo, err = d.Downloader.GetRepoInfo(ctx)
		return err
	})

	return repo, err
}

// GetTopics returns a repository's topics with retry
func (d *RetryDownloader) GetTopics(ctx context.Context) ([]string, error) {
	var (
		topics []string
		err    error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		topics, err = d.Downloader.GetTopics(ctx)
		return err
	})

	return topics, err
}

// GetMilestones returns a repository's milestones with retry
func (d *RetryDownloader) GetMilestones(ctx context.Context) ([]*Milestone, error) {
	var (
		milestones []*Milestone
		err        error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		milestones, err = d.Downloader.GetMilestones(ctx)
		return err
	})

	return milestones, err
}

// GetReleases returns a repository's releases with retry
func (d *RetryDownloader) GetReleases(ctx context.Context) ([]*Release, error) {
	var (
		releases []*Release
		err      error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		releases, err = d.Downloader.GetReleases(ctx)
		return err
	})

	return releases, err
}

// GetLabels returns a repository's labels with retry
func (d *RetryDownloader) GetLabels(ctx context.Context) ([]*Label, error) {
	var (
		labels []*Label
		err    error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		labels, err = d.Downloader.GetLabels(ctx)
		return err
	})

	return labels, err
}

// GetIssues returns a repository's issues with retry
func (d *RetryDownloader) GetIssues(ctx context.Context, page, perPage int) ([]*Issue, bool, error) {
	var (
		issues []*Issue
		isEnd  bool
		err    error
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		issues, isEnd, err = d.Downloader.GetIssues(ctx, page, perPage)
		return err
	})

	return issues, isEnd, err
}

// GetComments returns a repository's comments with retry
func (d *RetryDownloader) GetComments(ctx context.Context, commentable Commentable) ([]*Comment, bool, error) {
	var (
		comments []*Comment
		isEnd    bool
		err      error
	)

	err = d.retry(ctx, func(context.Context) error {
		comments, isEnd, err = d.Downloader.GetComments(ctx, commentable)
		return err
	})

	return comments, isEnd, err
}

// GetPullRequests returns a repository's pull requests with retry
func (d *RetryDownloader) GetPullRequests(ctx context.Context, page, perPage int) ([]*PullRequest, bool, error) {
	var (
		prs   []*PullRequest
		err   error
		isEnd bool
	)

	err = d.retry(ctx, func(ctx context.Context) error {
		prs, isEnd, err = d.Downloader.GetPullRequests(ctx, page, perPage)
		return err
	})

	return prs, isEnd, err
}

// GetReviews returns pull requests reviews
func (d *RetryDownloader) GetReviews(ctx context.Context, reviewable Reviewable) ([]*Review, error) {
	var (
		reviews []*Review
		err     error
	)
	err = d.retry(ctx, func(ctx context.Context) error {
		reviews, err = d.Downloader.GetReviews(ctx, reviewable)
		return err
	})

	return reviews, err
}
