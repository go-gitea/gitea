// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
)

func Test_AddTableCommitStatusIndex(t *testing.T) {
	// Create the models used in the migration
	type CommitStatus struct {
		ID     int64  `xorm:"pk autoincr"`
		Index  int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
		RepoID int64  `xorm:"INDEX UNIQUE(repo_sha_index)"`
		SHA    string `xorm:"VARCHAR(64) NOT NULL INDEX UNIQUE(repo_sha_index)"`
	}

	// Prepare and load the testing database
	x, deferable := base.PrepareTestEnv(t, 0, new(CommitStatus))
	if x == nil || t.Failed() {
		defer deferable()
		return
	}
	defer deferable()

	// Run the migration
	if err := AddTableCommitStatusIndex(x); err != nil {
		assert.NoError(t, err)
		return
	}

	type CommitStatusIndex struct {
		ID       int64
		RepoID   int64  `xorm:"unique(repo_sha)"`
		SHA      string `xorm:"unique(repo_sha)"`
		MaxIndex int64  `xorm:"index"`
	}

	start := 0
	const batchSize = 1000
	for {
		indexes := make([]CommitStatusIndex, 0, batchSize)
		err := x.Table("commit_status_index").Limit(batchSize, start).Find(&indexes)
		assert.NoError(t, err)

		for _, idx := range indexes {
			var maxIndex int
			has, err := x.SQL("SELECT max(`index`) FROM commit_status WHERE repo_id = ? AND sha = ?", idx.RepoID, idx.SHA).Get(&maxIndex)
			assert.NoError(t, err)
			assert.True(t, has)
			assert.EqualValues(t, maxIndex, idx.MaxIndex)
		}
		if len(indexes) < batchSize {
			break
		}
		start += len(indexes)
	}
}
