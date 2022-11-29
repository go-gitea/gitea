// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func addBotTables(x *xorm.Engine) error {
	type BotRunner struct {
		ID          int64
		UUID        string `xorm:"CHAR(36) UNIQUE"`
		Name        string `xorm:"VARCHAR(32)"`
		OwnerID     int64  `xorm:"index"` // org level runner, 0 means system
		RepoID      int64  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
		Description string `xorm:"TEXT"`
		Base        int    // 0 native 1 docker 2 virtual machine
		RepoRange   string // glob match which repositories could use this runner

		Token     string `xorm:"-"`
		TokenHash string `xorm:"UNIQUE"` // sha256 of token
		TokenSalt string
		// TokenLastEight string `xorm:"token_last_eight"` // it's unnecessary because we don't find runners by token

		LastOnline timeutil.TimeStamp `xorm:"index"`
		LastActive timeutil.TimeStamp `xorm:"index"`

		// Store OS and Artch.
		AgentLabels []string
		// Store custom labes use defined.
		CustomLabels []string

		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated"`
		Deleted timeutil.TimeStamp `xorm:"deleted"`
	}

	type BotRunnerToken struct {
		ID       int64
		Token    string `xorm:"UNIQUE"`
		OwnerID  int64  `xorm:"index"` // org level runner, 0 means system
		RepoID   int64  `xorm:"index"` // repo level runner, if orgid also is zero, then it's a global
		IsActive bool

		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated"`
		Deleted timeutil.TimeStamp `xorm:"deleted"`
	}

	type BotRun struct {
		ID                int64
		Title             string
		RepoID            int64  `xorm:"index unique(repo_index)"`
		OwnerID           int64  `xorm:"index"`
		WorkflowID        string `xorm:"index"`                    // the name of workflow file
		Index             int64  `xorm:"index unique(repo_index)"` // a unique number for each run of a repository
		TriggerUserID     int64
		Ref               string
		CommitSHA         string
		Event             string
		IsForkPullRequest bool
		EventPayload      string `xorm:"LONGTEXT"`
		Status            int    `xorm:"index"`
		Started           timeutil.TimeStamp
		Stopped           timeutil.TimeStamp
		Created           timeutil.TimeStamp `xorm:"created"`
		Updated           timeutil.TimeStamp `xorm:"updated"`
	}

	type BotRunJob struct {
		ID                int64
		RunID             int64  `xorm:"index"`
		RepoID            int64  `xorm:"index"`
		OwnerID           int64  `xorm:"index"`
		CommitSHA         string `xorm:"index"`
		IsForkPullRequest bool
		Name              string
		Attempt           int64
		WorkflowPayload   []byte
		JobID             string   // job id in workflow, not job's id
		Needs             []string `xorm:"JSON TEXT"`
		RunsOn            []string `xorm:"JSON TEXT"`
		TaskID            int64    // the latest task of the job
		Status            int      `xorm:"index"`
		Started           timeutil.TimeStamp
		Stopped           timeutil.TimeStamp
		Created           timeutil.TimeStamp `xorm:"created"`
		Updated           timeutil.TimeStamp `xorm:"updated index"`
	}

	type Repository struct {
		NumRuns       int `xorm:"NOT NULL DEFAULT 0"`
		NumClosedRuns int `xorm:"NOT NULL DEFAULT 0"`
	}

	type BotRunIndex db.ResourceIndex

	type BotTask struct {
		ID       int64
		JobID    int64
		Attempt  int64
		RunnerID int64              `xorm:"index"`
		Status   int                `xorm:"index"`
		Started  timeutil.TimeStamp `xorm:"index"`
		Stopped  timeutil.TimeStamp

		RepoID            int64  `xorm:"index"`
		OwnerID           int64  `xorm:"index"`
		CommitSHA         string `xorm:"index"`
		IsForkPullRequest bool

		TokenHash      string `xorm:"UNIQUE"` // sha256 of token
		TokenSalt      string
		TokenLastEight string `xorm:"index token_last_eight"`

		LogFilename  string   // file name of log
		LogInStorage bool     // read log from database or from storage
		LogLength    int64    // lines count
		LogSize      int64    // blob size
		LogIndexes   *[]int64 `xorm:"BLOB"` // line number to offset
		LogExpired   bool     // files that are too old will be deleted

		Created timeutil.TimeStamp `xorm:"created"`
		Updated timeutil.TimeStamp `xorm:"updated index"`
	}

	type BotTaskStep struct {
		ID        int64
		Name      string
		TaskID    int64 `xorm:"index unique(task_number)"`
		Number    int64 `xorm:"index unique(task_number)"`
		Status    int   `xorm:"index"`
		LogIndex  int64
		LogLength int64
		Started   timeutil.TimeStamp
		Stopped   timeutil.TimeStamp
		Created   timeutil.TimeStamp `xorm:"created"`
		Updated   timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(
		new(BotRunner),
		new(BotRunnerToken),
		new(BotRun),
		new(BotRunJob),
		new(Repository),
		new(BotRunIndex),
		new(BotTask),
		new(BotTaskStep),
	)
}
