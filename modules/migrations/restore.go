// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"os"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/migrations/base"
)

// RepositoryRestorer implements an Downloader from the local directory
type RepositoryRestorer struct {
	ctx                  context.Context
	baseDir              string
	repoOwner            string
	repoName             string
	milestoneFile        *os.File
	labelFile            *os.File
	releaseFile          *os.File
	issueFile            *os.File
	commentFile          *os.File
	pullrequestFile      *os.File
	reviewFile           *os.File
	migrateReleaseAssets bool

	gitRepo     *git.Repository
	prHeadCache map[string]struct{}
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

}

func (r *RepositoryRestorer) GetTopics() ([]string, error) {

}

func (r *RepositoryRestorer) GetMilestones() ([]*base.Milestone, error) {

}

func (r *RepositoryRestorer) GetReleases() ([]*base.Release, error) {

}

func (r *RepositoryRestorer) GetLabels() ([]*base.Label, error) {

}

func (r *RepositoryRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {

}

func (r *RepositoryRestorer) GetComments(issueNumber int64) ([]*base.Comment, error) {

}

func (r *RepositoryRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, error) {

}

func (r *RepositoryRestorer) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {

}
