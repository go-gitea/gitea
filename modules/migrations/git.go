// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"

	"code.gitea.io/gitea/modules/migrations/base"
)

var (
	_ base.Downloader = &PlainGitDownloader{}
)

// PlainGitDownloader implements a Downloader interface to clone git from a http/https URL
type PlainGitDownloader struct {
	ownerName string
	repoName  string
	remoteURL string
}

// NewPlainGitDownloader creates a git Downloader
func NewPlainGitDownloader(ownerName, repoName, remoteURL string) *PlainGitDownloader {
	return &PlainGitDownloader{
		ownerName: ownerName,
		repoName:  repoName,
		remoteURL: remoteURL,
	}
}

// SetContext set context
func (g *PlainGitDownloader) SetContext(ctx context.Context) {
}

// GetRepoInfo returns a repository information
func (g *PlainGitDownloader) GetRepoInfo() (*base.Repository, error) {
	// convert github repo to stand Repo
	return &base.Repository{
		Owner:    g.ownerName,
		Name:     g.repoName,
		CloneURL: g.remoteURL,
	}, nil
}

// GetTopics returns empty list for plain git repo
func (g *PlainGitDownloader) GetTopics() ([]string, error) {
	return []string{}, nil
}

// GetMilestones returns milestones
func (g *PlainGitDownloader) GetMilestones() ([]*base.Milestone, error) {
	return nil, ErrNotSupported
}

// GetLabels returns labels
func (g *PlainGitDownloader) GetLabels() ([]*base.Label, error) {
	return nil, ErrNotSupported
}

// GetReleases returns releases
func (g *PlainGitDownloader) GetReleases() ([]*base.Release, error) {
	return nil, ErrNotSupported
}

// GetIssues returns issues according page and perPage
func (g *PlainGitDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	return nil, false, ErrNotSupported
}

// GetComments returns comments according issueNumber
func (g *PlainGitDownloader) GetComments(issueNumber int64) ([]*base.Comment, error) {
	return nil, ErrNotSupported
}

// GetPullRequests returns pull requests according page and perPage
func (g *PlainGitDownloader) GetPullRequests(start, limit int) ([]*base.PullRequest, error) {
	return nil, ErrNotSupported
}
