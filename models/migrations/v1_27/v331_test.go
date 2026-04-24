// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_27

import (
	"context"
	"slices"
	"testing"

	"code.gitea.io/gitea/models/migrations/base"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm/schemas"
)

type actionRunBeforeV331 struct {
	ID                int64 `xorm:"pk autoincr"`
	ConcurrencyGroup  string
	ConcurrencyCancel bool
	LatestAttemptID   int64 `xorm:"-"`
}

func (actionRunBeforeV331) TableName() string {
	return "action_run"
}

type actionRunJobBeforeV331 struct {
	ID     int64 `xorm:"pk autoincr"`
	RunID  int64 `xorm:"index"`
	RepoID int64 `xorm:"index"`
}

func (actionRunJobBeforeV331) TableName() string {
	return "action_run_job"
}

type actionArtifactBeforeV331 struct {
	ID           int64  `xorm:"pk autoincr"`
	RunID        int64  `xorm:"index unique(runid_name_path)"`
	RepoID       int64  `xorm:"index"`
	ArtifactPath string `xorm:"index unique(runid_name_path)"`
	ArtifactName string `xorm:"index unique(runid_name_path)"`
}

func (actionArtifactBeforeV331) TableName() string {
	return "action_artifact"
}

func Test_AddActionRunAttemptModel(t *testing.T) {
	x, deferable := base.PrepareTestEnv(t, 0,
		new(actionRunBeforeV331),
		new(actionRunJobBeforeV331),
		new(actionArtifactBeforeV331),
	)
	defer deferable()
	if x == nil || t.Failed() {
		return
	}

	_, err := x.Insert(&actionArtifactBeforeV331{
		RunID:        1,
		RepoID:       1,
		ArtifactPath: "artifact/path",
		ArtifactName: "artifact-name",
	})
	require.NoError(t, err)

	require.NoError(t, AddActionRunAttemptModel(x))

	tableMap := base.LoadTableSchemasMap(t, x)

	attemptTable := tableMap["action_run_attempt"]
	require.NotNil(t, attemptTable)
	attemptTablCols := []string{"id", "repo_id", "run_id", "attempt", "trigger_user_id", "status", "started", "stopped", "concurrency_group", "concurrency_cancel", "created", "updated"}
	require.ElementsMatch(t, attemptTable.ColumnsSeq(), attemptTablCols)

	runTable := tableMap["action_run"]
	require.NotNil(t, runTable)
	require.Contains(t, runTable.ColumnsSeq(), "latest_attempt_id")
	require.NotContains(t, runTable.ColumnsSeq(), "concurrency_group")
	require.NotContains(t, runTable.ColumnsSeq(), "concurrency_cancel")

	jobTable := tableMap["action_run_job"]
	require.NotNil(t, jobTable)
	require.Contains(t, jobTable.ColumnsSeq(), "run_attempt_id")
	require.Contains(t, jobTable.ColumnsSeq(), "attempt_job_id")
	require.Contains(t, jobTable.ColumnsSeq(), "source_task_id")

	attemptIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_run_attempt")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(attemptIndexes, []string{"run_id", "attempt"}, true))
	assert.True(t, hasIndexWithColumns(attemptIndexes, []string{"repo_id", "concurrency_group", "status"}, false))

	runIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_run")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(runIndexes, []string{"latest_attempt_id"}, false))
	assert.False(t, hasIndexWithColumns(runIndexes, []string{"repo_id", "concurrency_group"}, false))

	jobIndexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_run_job")
	require.NoError(t, err)
	assert.True(t, hasIndexWithColumns(jobIndexes, []string{"run_attempt_id"}, false))
	assert.True(t, hasIndexWithColumns(jobIndexes, []string{"attempt_job_id"}, false))

	indexes, err := x.Dialect().GetIndexes(x.DB(), context.Background(), "action_artifact")
	require.NoError(t, err)
	assert.False(t, hasIndexWithColumns(indexes, []string{"run_id", "artifact_path", "artifact_name"}, true))
	assert.True(t, hasIndexWithColumns(indexes, []string{"run_id", "run_attempt_id", "artifact_path", "artifact_name"}, true))

	_, err = x.Insert(&actionArtifact{
		RunID:        1,
		RunAttemptID: 2,
		RepoID:       1,
		ArtifactPath: "artifact/path",
		ArtifactName: "artifact-name",
	})
	require.NoError(t, err)
	_, err = x.Insert(&actionArtifact{
		RunID:        1,
		RunAttemptID: 2,
		RepoID:       1,
		ArtifactPath: "artifact/path",
		ArtifactName: "artifact-name",
	})
	require.Error(t, err)

	_, err = x.Insert(&actionRunAttempt{
		RepoID:        1,
		RunID:         1,
		Attempt:       2,
		TriggerUserID: 1,
		Status:        1,
	})
	require.NoError(t, err)
	_, err = x.Insert(&actionRunAttempt{
		RepoID:        1,
		RunID:         1,
		Attempt:       2,
		TriggerUserID: 2,
		Status:        1,
	})
	require.Error(t, err)
}

func hasIndexWithColumns(indexes map[string]*schemas.Index, cols []string, isUnique bool) bool {
	for _, index := range indexes {
		if isUnique && index.Type != schemas.UniqueType {
			continue
		}
		if slices.Equal(index.Cols, cols) {
			return true
		}
	}
	return false
}
