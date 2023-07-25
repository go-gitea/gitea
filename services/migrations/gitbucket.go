// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/log"
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

	fields := strings.Split(u.Path, "/")
	if len(fields) < 2 {
		return nil, fmt.Errorf("invalid path: %s", u.Path)
	}
	baseURL := u.Scheme + "://" + u.Host + strings.TrimSuffix(strings.Join(fields[:len(fields)-2], "/"), "/git")

	oldOwner := fields[len(fields)-2]
	oldName := strings.TrimSuffix(fields[len(fields)-1], ".git")

	log.Trace("Create GitBucket downloader. BaseURL: %s RepoOwner: %s RepoName: %s", baseURL, oldOwner, oldName)
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

// String implements Stringer
func (g *GitBucketDownloader) String() string {
	return fmt.Sprintf("migration from gitbucket server %s %s/%s", g.baseURL, g.repoOwner, g.repoName)
}

func (g *GitBucketDownloader) LogString() string {
	if g == nil {
		return "<GitBucketDownloader nil>"
	}
	return fmt.Sprintf("<GitBucketDownloader %s %s/%s>", g.baseURL, g.repoOwner, g.repoName)
}

// NewGitBucketDownloader creates a GitBucket downloader
func NewGitBucketDownloader(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GitBucketDownloader {
	githubDownloader := NewGithubDownloaderV3(ctx, baseURL, userName, password, token, repoOwner, repoName)
	githubDownloader.SkipReactions = true
	githubDownloader.SkipReviews = true
	return &GitBucketDownloader{
		githubDownloader,
	}
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GitBucketDownloader) SupportGetRepoComments() bool {
	return false
}
