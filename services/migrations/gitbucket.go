// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"strings"

	"gitea.dev/modules/log"
	base "gitea.dev/modules/migration"
	"gitea.dev/modules/structs"
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
	info, err := parseServiceCloneURL(opts.CloneAddr)
	if err != nil {
		return nil, err
	}
	if len(info.segments) < 2 {
		return nil, fmt.Errorf("invalid path: %s", info.repoPath)
	}

	// GitBucket exposes its API at "<host>/<sub-path>" where <sub-path> is the URL
	// minus the trailing "/git/<owner>/<repo>.git" used for the git clone endpoint.
	subPath := strings.Join(info.segments[:len(info.segments)-2], "/")
	if subPath != "" {
		subPath = "/" + subPath
	}
	baseURL := info.apiURL.String() + strings.TrimSuffix(subPath, "/git")

	oldOwner := info.segments[len(info.segments)-2]
	oldName := strings.TrimSuffix(info.segments[len(info.segments)-1], ".git")

	log.Trace("Create GitBucket downloader. BaseURL: %s RepoOwner: %s RepoName: %s", baseURL, oldOwner, oldName)
	return NewGitBucketDownloader(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName)
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
func NewGitBucketDownloader(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) (*GitBucketDownloader, error) {
	githubDownloader, err := NewGithubDownloaderV3(ctx, baseURL, userName, password, token, repoOwner, repoName)
	if err != nil {
		return nil, err
	}
	// Gitbucket 4.40 uses different internal hard-coded perPage values.
	// Issues, PRs, and other major parts use 25.  Release page uses 10.
	// Some API doesn't support paging yet.  Sounds difficult, but using
	// minimum number among them worked out very well.
	githubDownloader.maxPerPage = 10
	githubDownloader.SkipReactions = true
	githubDownloader.SkipReviews = true
	return &GitBucketDownloader{
		githubDownloader,
	}, nil
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GitBucketDownloader) SupportGetRepoComments() bool {
	return false
}
