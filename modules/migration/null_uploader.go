// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// NullUploader implements a blank uploader
type NullUploader struct{}

var _ Uploader = &NullUploader{}

func (g *NullUploader) MaxBatchInsertSize(tp string) int {
	return 0
}

func (g *NullUploader) CreateRepo(repo *Repository, opts MigrateOptions) error {
	return nil
}

func (g *NullUploader) CreateTopics(topic ...string) error {
	return nil
}

func (g *NullUploader) CreateMilestones(milestones ...*Milestone) error {
	return nil
}

func (g *NullUploader) CreateReleases(releases ...*Release) error {
	return nil
}

func (g *NullUploader) SyncTags() error {
	return nil
}

func (g *NullUploader) CreateLabels(labels ...*Label) error {
	return nil
}

func (g *NullUploader) CreateIssues(issues ...*Issue) error {
	return nil
}

func (g *NullUploader) CreateComments(comments ...*Comment) error {
	return nil
}

func (g *NullUploader) CreatePullRequests(prs ...*PullRequest) error {
	return nil
}

func (g *NullUploader) CreateReviews(reviews ...*Review) error {
	return nil
}

func (g *NullUploader) UpdateTopics(topic ...string) error {
	return nil
}

func (g *NullUploader) UpdateMilestones(milestones ...*Milestone) error {
	return nil
}

func (g *NullUploader) UpdateLabels(labels ...*Label) error {
	return nil
}

func (g *NullUploader) PatchReleases(releases ...*Release) error {
	return nil
}

func (g *NullUploader) PatchIssues(issues ...*Issue) error {
	return nil
}

func (g *NullUploader) PatchComments(comments ...*Comment) error {
	return nil
}

func (g *NullUploader) PatchPullRequests(prs ...*PullRequest) error {
	return nil
}

func (g *NullUploader) PatchReviews(reviews ...*Review) error {
	return nil
}

func (g *NullUploader) Rollback() error {
	return nil
}

func (g *NullUploader) Finish() error {
	return nil
}

func (g *NullUploader) Close() {}
