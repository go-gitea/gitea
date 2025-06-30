// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	base "code.gitea.io/gitea/modules/migration"

	"gopkg.in/yaml.v3"
)

// RepositoryRestorer implements an Downloader from the local directory
type RepositoryRestorer struct {
	base.NullDownloader
	baseDir    string
	repoOwner  string
	repoName   string
	validation bool
}

// NewRepositoryRestorer creates a repository restorer which could restore repository from a dumped folder
func NewRepositoryRestorer(_ context.Context, baseDir, owner, repoName string, validation bool) (*RepositoryRestorer, error) {
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	return &RepositoryRestorer{
		baseDir:    baseDir,
		repoOwner:  owner,
		repoName:   repoName,
		validation: validation,
	}, nil
}

func (r *RepositoryRestorer) commentDir() string {
	return filepath.Join(r.baseDir, "comments")
}

func (r *RepositoryRestorer) reviewDir() string {
	return filepath.Join(r.baseDir, "reviews")
}

func (r *RepositoryRestorer) getRepoOptions() (map[string]string, error) {
	p := filepath.Join(r.baseDir, "repo.yml")
	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	opts := make(map[string]string)
	err = yaml.Unmarshal(bs, &opts)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// GetRepoInfo returns a repository information
func (r *RepositoryRestorer) GetRepoInfo(_ context.Context) (*base.Repository, error) {
	opts, err := r.getRepoOptions()
	if err != nil {
		return nil, err
	}

	isPrivate, _ := strconv.ParseBool(opts["is_private"])

	return &base.Repository{
		Owner:         r.repoOwner,
		Name:          r.repoName,
		IsPrivate:     isPrivate,
		Description:   opts["description"],
		OriginalURL:   opts["original_url"],
		CloneURL:      filepath.Join(r.baseDir, "git"),
		DefaultBranch: opts["default_branch"],
	}, nil
}

// GetTopics return github topics
func (r *RepositoryRestorer) GetTopics(_ context.Context) ([]string, error) {
	p := filepath.Join(r.baseDir, "topic.yml")

	topics := struct {
		Topics []string `yaml:"topics"`
	}{}

	bs, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	err = yaml.Unmarshal(bs, &topics)
	if err != nil {
		return nil, err
	}
	return topics.Topics, nil
}

// GetMilestones returns milestones
func (r *RepositoryRestorer) GetMilestones(_ context.Context) ([]*base.Milestone, error) {
	milestones := make([]*base.Milestone, 0, 10)
	p := filepath.Join(r.baseDir, "milestone.yml")
	err := base.Load(p, &milestones, r.validation)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return milestones, nil
}

// GetReleases returns releases
func (r *RepositoryRestorer) GetReleases(_ context.Context) ([]*base.Release, error) {
	releases := make([]*base.Release, 0, 10)
	p := filepath.Join(r.baseDir, "release.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &releases)
	if err != nil {
		return nil, err
	}
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if asset.DownloadURL != nil {
				*asset.DownloadURL = "file://" + filepath.Join(r.baseDir, *asset.DownloadURL)
			}
		}
	}
	return releases, nil
}

// GetLabels returns labels
func (r *RepositoryRestorer) GetLabels(_ context.Context) ([]*base.Label, error) {
	labels := make([]*base.Label, 0, 10)
	p := filepath.Join(r.baseDir, "label.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := os.ReadFile(p)
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
func (r *RepositoryRestorer) GetIssues(_ context.Context, _, _ int) ([]*base.Issue, bool, error) {
	issues := make([]*base.Issue, 0, 10)
	p := filepath.Join(r.baseDir, "issue.yml")
	err := base.Load(p, &issues, r.validation)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}
	return issues, true, nil
}

// GetComments returns comments according issueNumber
func (r *RepositoryRestorer) GetComments(_ context.Context, commentable base.Commentable) ([]*base.Comment, bool, error) {
	comments := make([]*base.Comment, 0, 10)
	p := filepath.Join(r.commentDir(), fmt.Sprintf("%d.yml", commentable.GetForeignIndex()))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &comments)
	if err != nil {
		return nil, false, err
	}
	return comments, false, nil
}

// GetPullRequests returns pull requests according page and perPage
func (r *RepositoryRestorer) GetPullRequests(_ context.Context, page, perPage int) ([]*base.PullRequest, bool, error) {
	pulls := make([]*base.PullRequest, 0, 10)
	p := filepath.Join(r.baseDir, "pull_request.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &pulls)
	if err != nil {
		return nil, false, err
	}
	for _, pr := range pulls {
		pr.PatchURL = "file://" + filepath.Join(r.baseDir, pr.PatchURL)
		CheckAndEnsureSafePR(pr, "", r)
	}
	return pulls, true, nil
}

// GetReviews returns pull requests review
func (r *RepositoryRestorer) GetReviews(ctx context.Context, reviewable base.Reviewable) ([]*base.Review, error) {
	reviews := make([]*base.Review, 0, 10)
	p := filepath.Join(r.reviewDir(), fmt.Sprintf("%d.yml", reviewable.GetForeignIndex()))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &reviews)
	if err != nil {
		return nil, err
	}
	return reviews, nil
}
