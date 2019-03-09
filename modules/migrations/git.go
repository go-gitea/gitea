// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
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

func (g *PlainGitDownloader) GetRepoInfo() (*base.Repository, error) {
	// convert github repo to stand Repo
	return &base.Repository{
		Owner:    g.ownerName,
		Name:     g.repoName,
		CloneURL: g.remoteURL,
	}, nil
}

func (g *PlainGitDownloader) GetMilestones() ([]*base.Milestone, error) {
	return nil, ErrNotSupported
}

func (g *PlainGitDownloader) GetLabels() ([]*base.Label, error) {
	return nil, ErrNotSupported
}

func (g *PlainGitDownloader) GetIssues(start, limit int) ([]*base.Issue, error) {
	return nil, ErrNotSupported
}

func (g *PlainGitDownloader) GetComments(issueNumber int64) ([]*base.Comment, error) {
	return nil, ErrNotSupported
}

func (g *PlainGitDownloader) GetPullRequests(start, limit int) ([]*base.PullRequest, error) {
	return nil, ErrNotSupported
}
