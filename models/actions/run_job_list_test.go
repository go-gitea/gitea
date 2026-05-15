// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionJobList_SortMatrixGroupsByName(t *testing.T) {
	mk := func(jobID, name string) *ActionRunJob {
		return &ActionRunJob{JobID: jobID, Name: name}
	}
	names := func(jobs ActionJobList) []string {
		out := make([]string, len(jobs))
		for i, j := range jobs {
			out[i] = j.Name
		}
		return out
	}

	t.Run("matrix group sorted naturally", func(t *testing.T) {
		jobs := ActionJobList{
			mk("build", "build"),
			mk("test", "test (10)"),
			mk("test", "test (2)"),
			mk("test", "test (1)"),
			mk("deploy", "deploy"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"build", "test (1)", "test (2)", "test (10)", "deploy"}, names(jobs))
	})

	t.Run("non-adjacent same JobID stays in input order", func(t *testing.T) {
		jobs := ActionJobList{
			mk("test", "test (10)"),
			mk("build", "build"),
			mk("test", "test (1)"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"test (10)", "build", "test (1)"}, names(jobs))
	})

	t.Run("groups stay in input order", func(t *testing.T) {
		jobs := ActionJobList{
			mk("z", "z"),
			mk("a", "a"),
		}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"z", "a"}, names(jobs))
	})

	t.Run("empty and singleton", func(t *testing.T) {
		ActionJobList(nil).SortMatrixGroupsByName()
		jobs := ActionJobList{mk("only", "only")}
		jobs.SortMatrixGroupsByName()
		assert.Equal(t, []string{"only"}, names(jobs))
	})
}
