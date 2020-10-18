// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"io"

	"code.gitea.io/gitea/modules/migrations/base"
)

// RepositoryRestorer implements an Downloader from the local directory
type RepositoryRestorer struct {
	ctx       context.Context
	baseDir   string
	repoOwner string
	repoName  string
}

func NewRepositoryRestorer(ctx context.Context, baseDir string, owner, repoName string) *RepositoryRestorer {
	return &RepositoryRestorer{
		ctx:       ctx,
		baseDir:   baseDir,
		repoOwner: owner,
		repoName:  repoName,
	}
}

func (r *RepositoryRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

func (r *RepositoryRestorer) GetRepoInfo() (*base.Repository, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetTopics() ([]string, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetMilestones() ([]*base.Milestone, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetReleases() ([]*base.Release, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetAsset(tagName string, relID, assetID int64) (io.ReadCloser, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetLabels() ([]*base.Label, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	return nil, false, nil
}

func (r *RepositoryRestorer) GetComments(issueNumber int64) ([]*base.Comment, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	return nil, false, nil
}

func (r *RepositoryRestorer) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {
	return nil, nil
}
