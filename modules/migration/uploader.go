// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// SPDX-License-Identifier: MIT

package migration

// Uploader uploads all the information of one repository
type Uploader interface {
	MaxBatchInsertSize(tp string) int
	CreateRepo(repo *Repository, opts MigrateOptions) error
	CreateTopics(topics ...string) error
	CreateMilestones(milestones ...*Milestone) error
	CreateReleases(releases ...*Release) error
	SyncTags() error
	CreateLabels(labels ...*Label) error
	CreateIssues(issues ...*Issue) error
	CreateComments(comments ...*Comment) error
	CreatePullRequests(prs ...*PullRequest) error
	CreateReviews(reviews ...*Review) error
	UpdateTopics(topics ...string) error             // update topics of a repository, and delete those that are not in the list
	UpdateMilestones(milestones ...*Milestone) error // update milestones of a repository, and delete those that are not in the list
	UpdateLabels(labels ...*Label) error             // rewrite all issue labels and delete those that are not in the list
	PatchReleases(releases ...*Release) error        // add or update releases (no deletes)
	PatchComments(comments ...*Comment) error        // add or update comments (no deletes)
	PatchIssues(issues ...*Issue) error              // add or update issues (no deletes)
	PatchPullRequests(prs ...*PullRequest) error     // add or update pull requests (no deletes)
	PatchReviews(reviews ...*Review) error           // add or update reviews (no deletes)
	Rollback() error
	Finish() error
	Close()
}
