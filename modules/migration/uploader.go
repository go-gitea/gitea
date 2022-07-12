// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migration

// Uploader uploads all the information of one repository
type Uploader interface {
	MaxBatchInsertSize(tp string) int
	CreateRepo(repo *Repository, opts MigrateOptions) error
	CreateTopics(topic ...string) error
	CreateMilestones(milestones ...*Milestone) error
	CreateReleases(releases ...*Release) error
	SyncTags() error
	CreateLabels(labels ...*Label) error
	CreateIssues(issues ...*Issue) error
	CreateComments(comments ...*Comment) error
	CreatePullRequests(prs ...*PullRequest) error
	CreateReviews(reviews ...*Review) error
	UpdateTopics(topic ...string) error              // update topics of a repository, and delete unused ones
	UpdateMilestones(milestones ...*Milestone) error // update milestones of a repository, and delete unused ones
	UpdateLabels(labels ...*Label) error             // rewrite all issue labels and delete unused ones
	PatchReleases(releases ...*Release) error        // add or update releases (no deletes)
	PatchComments(comments ...*Comment) error        // add or update comments (no deletes)
	PatchIssues(issues ...*Issue) error              // add or update issues (no deletes)
	PatchPullRequests(prs ...*PullRequest) error     // add or update pull requests (no deletes)
	PatchReviews(reviews ...*Review) error           // add or update reviews (no deletes)
	Rollback() error
	Finish() error
	Close()
}
