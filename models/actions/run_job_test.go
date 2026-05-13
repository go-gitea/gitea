// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSortMatrixJobsByName(t *testing.T) {
	mk := func(jobID, name string) *ActionRunJob {
		return &ActionRunJob{JobID: jobID, Name: name}
	}
	names := func(jobs []*ActionRunJob) []string {
		out := make([]string, len(jobs))
		for i, j := range jobs {
			out[i] = j.Name
		}
		return out
	}

	t.Run("matrix group sorted naturally", func(t *testing.T) {
		jobs := []*ActionRunJob{
			mk("build", "build"),
			mk("test", "test (10)"),
			mk("test", "test (2)"),
			mk("test", "test (1)"),
			mk("deploy", "deploy"),
		}
		sortMatrixJobsByName(jobs)
		assert.Equal(t, []string{"build", "test (1)", "test (2)", "test (10)", "deploy"}, names(jobs))
	})

	t.Run("non-adjacent same JobID stays in input order", func(t *testing.T) {
		jobs := []*ActionRunJob{
			mk("test", "test (10)"),
			mk("build", "build"),
			mk("test", "test (1)"),
		}
		sortMatrixJobsByName(jobs)
		assert.Equal(t, []string{"test (10)", "build", "test (1)"}, names(jobs))
	})

	t.Run("groups stay in input order", func(t *testing.T) {
		jobs := []*ActionRunJob{
			mk("z", "z"),
			mk("a", "a"),
		}
		sortMatrixJobsByName(jobs)
		assert.Equal(t, []string{"z", "a"}, names(jobs))
	})

	t.Run("empty and singleton", func(t *testing.T) {
		sortMatrixJobsByName(nil)
		jobs := []*ActionRunJob{mk("only", "only")}
		sortMatrixJobsByName(jobs)
		assert.Equal(t, []string{"only"}, names(jobs))
	})
}

func TestGetRunJobsByRunAndAttemptID_MatrixOrder(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	const runID int64 = 9_999_001
	mk := func(jobID, name string) *ActionRunJob {
		return &ActionRunJob{RunID: runID, Attempt: 1, JobID: jobID, Name: name, Status: StatusWaiting}
	}
	inputs := []*ActionRunJob{
		mk("build", "build"),
		mk("test", "test (10)"),
		mk("test", "test (2)"),
		mk("test", "test (1)"),
		mk("deploy", "deploy"),
	}
	for _, j := range inputs {
		require.NoError(t, db.Insert(t.Context(), j))
	}

	got, err := GetRunJobsByRunAndAttemptID(t.Context(), runID, 0)
	require.NoError(t, err)
	gotNames := make([]string, len(got))
	for i, j := range got {
		gotNames[i] = j.Name
	}
	assert.Equal(t, []string{"build", "test (1)", "test (2)", "test (10)", "deploy"}, gotNames)
}
