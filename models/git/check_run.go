// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
)

type CheckRunStatus int64

const (
	// CheckRunStatusQueued queued
	CheckRunStatusQueued CheckRunStatus = iota
	// CheckRunStatusInProgress in_progress
	CheckRunStatusInProgress
	// CheckRunStatusQueued completed
	CheckRunStatusCompleted
)

type CheckRunConclusion int64

const (
	// CheckRunConclusionActionRequired action_required
	CheckRunConclusionActionRequired CheckRunConclusion = iota
	// CheckRunConclusionCancelled cancelled
	CheckRunConclusionCancelled
	// CheckRunConclusionFailure failure
	CheckRunConclusionFailure
	// CheckRunConclusionNeutral neutral
	CheckRunConclusionNeutral
	// CheckRunConclusionNeutral success
	CheckRunConclusionSuccess
	// CheckRunConclusionSkipped skipped
	CheckRunConclusionSkipped
	// CheckRunConclusionStale stale
	CheckRunConclusionStale
	// CheckRunConclusionTimedOut timed_out
	CheckRunConclusionTimedOut
)

// CommitStatus holds a single Status of a single Commit
type CheckRun struct {
	ID          int64                  `xorm:"pk autoincr"`
	RepoID      int64                  `xorm:"INDEX UNIQUE(repo_sha_index)"`
	Repo        *repo_model.Repository `xorm:"-"`
	Status      CheckRunStatus
	Conclusion  CheckRunConclusion
	HeadSHA     string           `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	DetailsURL  string           `xorm:"TEXT"`
	ExternalID  string           `xorm:"TEXT"`
	Description string           `xorm:"TEXT"`
	NameHash    string           `xorm:"char(40) index"`
	Name        string           `xorm:"TEXT"`
	Creator     *user_model.User `xorm:"-"`
	CreatorID   int64

	StartedAt   timeutil.TimeStamp
	CompletedAt timeutil.TimeStamp

	Output *CheckRunOutput `xorm:"-"`

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

type CheckRunOutput struct {
	ID             int64 `xorm:"pk autoincr"`
	ChekRunID      int64 `xorm:"INDEX"`
	Title          string
	Summary        string
	Text           string
	AnnotationsURL string
	Annotations    []api.CheckRunAnnotation
}

func init() {
	db.RegisterModel(new(CheckRun))
	db.RegisterModel(new(CheckRunOutput))
}
