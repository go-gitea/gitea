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

func (g *RepositoryRestorer) gitPath() string {
	return filepath.Join(g.baseDir, "git")
}

func (g *RepositoryRestorer) wikiPath() string {
	return filepath.Join(g.baseDir, "wiki")
}

func (g *RepositoryRestorer) topicDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) milestoneDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) labelDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) releaseDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) issueDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) commentDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) pullrequestDir() string {
	return filepath.Join(g.baseDir)
}

func (g *RepositoryRestorer) reviewDir() string {
	return filepath.Join(g.baseDir)
}

func (r *RepositoryRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

func (r *RepositoryRestorer) GetRepoInfo() (*base.Repository, error) {
	return nil, nil
}

func (r *RepositoryRestorer) GetTopics() ([]string, error) {
	p := filepath.Join(g.topicDir(), "topic.yml"))
	
	var topics = struct {
		Topics []string `yaml:"topics"`
	}

	bs, err := ioutil.ReadAll(p)
	if err != nil {
		return err
	}

	err := yaml.Unmarshal(bs, &topics)
	if err != nil {
		return err
	}
	return topics.Topics, nil
}

func (r *RepositoryRestorer) GetMilestones() ([]*base.Milestone, error) {
	var milestones = make([]*base.Milestone)
	p := filepath.Join(g.milestoneDir(), "milestone.yml")
	bs, err := ioutil.ReadAll(p)
	if err != nil {
		return err
	}

	err := yaml.Unmarshal(bs, &milestones)
	if err != nil {
		return err
	}
	return milestones, nil
}

func (r *RepositoryRestorer) GetReleases() ([]*base.Release, error) {
	var releases = make([]*base.Release)
	p := filepath.Join(g.releaseDir(), "release.yml")
	bs, err := ioutil.ReadAll(p)
	if err != nil {
		return err
	}

	err := yaml.Unmarshal(bs, &releases)
	if err != nil {
		return err
	}
	return releases, nil
}

func (r *RepositoryRestorer) GetAsset(tagName string, relID, assetID int64) (io.ReadCloser, error) {
	attachDir := filepath.Join(g.releaseDir(), "release_assets", tagName)
	attachLocalPath := filepath.Join(attachDir, asset.ID)
	return os.Open(attachLocalPath)
}

func (r *RepositoryRestorer) GetLabels() ([]*base.Label, error) {
	var labels = make([]*base.Label)
	p := filepath.Join(g.labelDir(), "label.yml")
	bs, err := ioutil.ReadAll(p)
	if err != nil {
		return err
	}

	err := yaml.Unmarshal(bs, &labels)
	if err != nil {
		return err
	}
	return labels, nil
}

func (r *RepositoryRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	var issues = make([]*base.Issue)
	p := filepath.Join(g.issuesDir(), "issue.yml")
	bs, err := ioutil.ReadAll(p)
	if err != nil {
		return nil, false, err
	}

	err := yaml.Unmarshal(bs, &issues)
	if err != nil {
		return err
	}
	return issues, false, nil
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
