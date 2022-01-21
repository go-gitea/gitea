// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"net/url"
	"strings"

	base "code.gitea.io/gitea/modules/migration"
	"code.gitea.io/gitea/modules/structs"
)

var (
	_ base.Downloader        = &GitBucketDownloader{}
	_ base.DownloaderFactory = &GitBucketDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GitBucketDownloaderFactory{})
}

// GitBucketDownloaderFactory defines a GitBucket downloader factory
type GitBucketDownloaderFactory struct{}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GitBucketDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	fields := strings.Split(u.Path, "/")
	oldOwner := fields[1]
	oldName := strings.TrimSuffix(fields[2], ".git")

	return NewGitBucketDownloader(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName), nil
}

// GitServiceType returns the type of git service
func (f *GitBucketDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GitBucketService
}

// GitBucketDownloader implements a Downloader interface to get repository information
// from GitBucket via GithubDownloader
type GitBucketDownloader struct {
	*GithubDownloaderV3
}

// NewGitBucketDownloader creates a GitBucket downloader
func NewGitBucketDownloader(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GitBucketDownloader {
	githubDownloader := NewGithubDownloaderV3(ctx, baseURL, userName, password, token, repoOwner, repoName)
	githubDownloader.SkipReactions = true
	return &GitBucketDownloader{
		githubDownloader,
	}
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GitBucketDownloader) SupportGetRepoComments() bool {
	return false
}

// GetReviews is not supported
func (g *GitBucketDownloader) GetReviews(context base.IssueContext) ([]*base.Review, error) {
	return nil, &base.ErrNotSupported{Entity: "Reviews"}
}
