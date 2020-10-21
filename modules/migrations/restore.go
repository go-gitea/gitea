// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/migrations/base"

	"gopkg.in/yaml.v2"
)

// RepositoryRestorer implements an Downloader from the local directory
type RepositoryRestorer struct {
	ctx       context.Context
	baseDir   string
	repoOwner string
	repoName  string
}

// NewRepositoryRestorer creates a repository restorer which could restore repository from a dumped folder
func NewRepositoryRestorer(ctx context.Context, baseDir string, owner, repoName string) *RepositoryRestorer {
	return &RepositoryRestorer{
		ctx:       ctx,
		baseDir:   baseDir,
		repoOwner: owner,
		repoName:  repoName,
	}
}

func (r *RepositoryRestorer) gitPath() string {
	return filepath.Join(r.baseDir, "git")
}

func (r *RepositoryRestorer) wikiPath() string {
	return filepath.Join(r.baseDir, "wiki")
}

func (r *RepositoryRestorer) topicDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) milestoneDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) labelDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) releaseDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) issueDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) commentDir() string {
	return filepath.Join(r.baseDir, "comments")
}

func (r *RepositoryRestorer) pullrequestDir() string {
	return filepath.Join(r.baseDir)
}

func (r *RepositoryRestorer) reviewDir() string {
	return filepath.Join(r.baseDir, "reviews")
}

// SetContext set context
func (r *RepositoryRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

// GetRepoInfo returns a repository information
func (r *RepositoryRestorer) GetRepoInfo() (*base.Repository, error) {
	return &base.Repository{
		Owner:         r.repoOwner,
		Name:          r.repoName,
		IsPrivate:     true,
		//Description:   gr.GetDescription(), // FIXME
		//OriginalURL:   gr.GetHTMLURL(), // FIXME
		CloneURL:      r.gitPath(), // FIXME
		DefaultBranch: "master", // FIXME
	}, nil
}

// GetTopics return github topics
func (r *RepositoryRestorer) GetTopics() ([]string, error) {
	p := filepath.Join(r.topicDir(), "topic.yml")
	
	var topics = struct {
		Topics []string `yaml:"topics"`
	}{}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &topics)
	if err != nil {
		return nil, err
	}
	return topics.Topics, nil
}

// GetMilestones returns milestones
func (r *RepositoryRestorer) GetMilestones() ([]*base.Milestone, error) {
	var milestones = make([]*base.Milestone, 0, 10)
	p := filepath.Join(r.milestoneDir(), "milestone.yml")
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &milestones)
	if err != nil {
		return nil, err
	}
	return milestones, nil
}

// GetReleases returns releases
func (r *RepositoryRestorer) GetReleases() ([]*base.Release, error) {
	var releases = make([]*base.Release, 0, 10)
	p := filepath.Join(r.releaseDir(), "release.yml")
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &releases)
	if err != nil {
		return nil, err
	}
	return releases, nil
}

// GetAsset returns an asset
func (r *RepositoryRestorer) GetAsset(tagName string, relID, assetID int64) (io.ReadCloser, error) {
	attachDir := filepath.Join(r.releaseDir(), "release_assets", tagName)
	attachLocalPath := filepath.Join(attachDir, fmt.Sprintf("%d", assetID))
	return os.Open(attachLocalPath)
}

// GetLabels returns labels
func (r *RepositoryRestorer) GetLabels() ([]*base.Label, error) {
	var labels = make([]*base.Label, 0, 10)
	p := filepath.Join(r.labelDir(), "label.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &labels)
	if err != nil {
		return nil, err
	}
	return labels, nil
}

// GetIssues returns issues according start and limit
func (r *RepositoryRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	var issues = make([]*base.Issue, 0, 10)
	p := filepath.Join(r.issueDir(), "issue.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &issues)
	if err != nil {
		return nil, false, err
	}
	return issues, true, nil
}

// GetComments returns comments according issueNumber
func (r *RepositoryRestorer) GetComments(issueNumber int64) ([]*base.Comment, error) {
	var comments = make([]*base.Comment, 0, 10)
	p := filepath.Join(r.commentDir(), fmt.Sprintf("%d.yml", issueNumber))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &comments)
	if err != nil {
		return nil, err
	}
	return comments, nil
}

// GetPullRequests returns pull requests according page and perPage
func (r *RepositoryRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	var pulls = make([]*base.PullRequest, 0, 10)
	p := filepath.Join(r.pullrequestDir(), "pull_request.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &pulls)
	if err != nil {
		return nil, false, err
	}
	return pulls, true, nil
}

// GetReviews returns pull requests review
func (r *RepositoryRestorer) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {
	var reviews = make([]*base.Review, 0, 10)
	p := filepath.Join(r.reviewDir(), fmt.Sprintf("%d.yml", pullRequestNumber))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &reviews)
	if err != nil {
		return nil, err
	}
	return reviews, nil
}
