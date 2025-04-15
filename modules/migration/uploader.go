// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

import "context"

// Uploader uploads all the information of one repository
type Uploader interface {
	MaxBatchInsertSize(tp string) int
	CreateRepo(ctx context.Context, repo *Repository, opts MigrateOptions) error
	CreateTopics(ctx context.Context, topic ...string) error
	CreateMilestones(ctx context.Context, milestones ...*Milestone) error
	CreateReleases(ctx context.Context, releases ...*Release) error
	SyncTags(ctx context.Context) error
	CreateLabels(ctx context.Context, labels ...*Label) error
	CreateIssues(ctx context.Context, issues ...*Issue) error
	CreateComments(ctx context.Context, comments ...*Comment) error
	CreatePullRequests(ctx context.Context, prs ...*PullRequest) error
	CreateReviews(ctx context.Context, reviews ...*Review) error
	Rollback() error
	Finish(ctx context.Context) error
	Close()
}
