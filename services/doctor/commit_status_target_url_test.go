// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package doctor

import (
	"testing"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/commitstatus"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_fixCommitStatusTargetURL_DryRun(t *testing.T) {
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	unittest.PrepareTestEnv(t)
	insertCommitStatusTargetURLTestData(t)

	require.NoError(t, fixCommitStatusTargetURL(t.Context(), log.GetLogger(log.DEFAULT), false))

	assertTargetURL(t, "commit_status", 99010, "/testuser/target-url-test/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status", 99011, "/testuser/target-url-test/actions/runs/7/jobs/1")
	assertTargetURL(t, "commit_status", 99012, "/otheruser/badrepo/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status", 99013, "/testuser/target-url-test/actions/runs/10/jobs/0")
	assertTargetURL(t, "commit_status", 99014, "/testuser/target-url-test/actions/runs/7/jobs/3")
	assertTargetURL(t, "commit_status", 99015, "https://ci.example.com/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status_summary", 99020, "/testuser/target-url-test/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status_summary", 99021, "https://ci.example.com/actions/runs/7/jobs/0")
}

func Test_fixCommitStatusTargetURL_AutoFix(t *testing.T) {
	defer test.MockVariableValue(&setting.AppSubURL, "")()
	unittest.PrepareTestEnv(t)
	insertCommitStatusTargetURLTestData(t)

	require.NoError(t, fixCommitStatusTargetURL(t.Context(), log.GetLogger(log.DEFAULT), true))

	assertTargetURL(t, "commit_status", 99010, "/testuser/target-url-test/actions/runs/99106/jobs/99530")
	assertTargetURL(t, "commit_status", 99011, "/testuser/target-url-test/actions/runs/99106/jobs/99531")
	assertTargetURL(t, "commit_status", 99012, "/otheruser/badrepo/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status", 99013, "/testuser/target-url-test/actions/runs/10/jobs/0")
	assertTargetURL(t, "commit_status", 99014, "/testuser/target-url-test/actions/runs/7/jobs/3")
	assertTargetURL(t, "commit_status", 99015, "https://ci.example.com/actions/runs/7/jobs/0")
	assertTargetURL(t, "commit_status_summary", 99020, "/testuser/target-url-test/actions/runs/99106/jobs/99530")
	assertTargetURL(t, "commit_status_summary", 99021, "https://ci.example.com/actions/runs/7/jobs/0")
}

func insertCommitStatusTargetURLTestData(t *testing.T) {
	t.Helper()

	x := db.GetEngine(t.Context())
	repo := &repo_model.Repository{
		ID:            99001,
		OwnerID:       2,
		OwnerName:     "testuser",
		LowerName:     "target-url-test",
		Name:          "target-url-test",
		DefaultBranch: "main",
	}
	_, err := x.Insert(repo)
	require.NoError(t, err)

	_, err = x.Insert(
		&actions_model.ActionRun{ID: 99106, RepoID: repo.ID, Index: 7},
		&actions_model.ActionRunJob{ID: 99530, RunID: 99106},
		&actions_model.ActionRunJob{ID: 99531, RunID: 99106},
	)
	require.NoError(t, err)

	_, err = x.Insert(
		// normal job
		&git_model.CommitStatus{ID: 99010, Index: 1, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "/testuser/target-url-test/actions/runs/7/jobs/0"},
		// normal job
		&git_model.CommitStatus{ID: 99011, Index: 2, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "/testuser/target-url-test/actions/runs/7/jobs/1"},
		// invalid: repo not found
		&git_model.CommitStatus{ID: 99012, Index: 3, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "/otheruser/badrepo/actions/runs/7/jobs/0"},
		// invalid: run_index not found
		&git_model.CommitStatus{ID: 99013, Index: 4, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "/testuser/target-url-test/actions/runs/10/jobs/0"},
		// invalid: job_index not found
		&git_model.CommitStatus{ID: 99014, Index: 5, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "/testuser/target-url-test/actions/runs/7/jobs/3"},
		// external target url
		&git_model.CommitStatus{ID: 99015, Index: 6, RepoID: repo.ID, SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: commitstatus.CommitStatusSuccess, TargetURL: "https://ci.example.com/actions/runs/7/jobs/0"},
	)
	require.NoError(t, err)

	_, err = x.Insert(
		// normal job
		&git_model.CommitStatusSummary{ID: 99020, RepoID: repo.ID, SHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", State: commitstatus.CommitStatusSuccess, TargetURL: "/testuser/target-url-test/actions/runs/7/jobs/0"},
		// external target url
		&git_model.CommitStatusSummary{ID: 99021, RepoID: repo.ID, SHA: "cccccccccccccccccccccccccccccccccccccccc", State: commitstatus.CommitStatusSuccess, TargetURL: "https://ci.example.com/actions/runs/7/jobs/0"},
	)
	require.NoError(t, err)
}

func assertTargetURL(t *testing.T, table string, id int64, want string) {
	t.Helper()

	var row struct {
		TargetURL string
	}
	has, err := db.GetEngine(t.Context()).Table(table).Where("id=?", id).Cols("target_url").Get(&row)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, want, row.TargetURL)
}
