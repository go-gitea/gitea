// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// Uploader uploads all the informations
type Uploader interface {
	CreateRepo(repo *Repository, includeWiki bool) error
	CreateMilestone(milestone *Milestone) error
	CreateRelease(release *Release) error
	CreateLabel(label *Label) error
	CreateIssue(issue *Issue) error
	CreateComment(issueNumber int64, comment *Comment) error
	CreatePullRequest(pr *PullRequest) error
	Rollback() error
}
