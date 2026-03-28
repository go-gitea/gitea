// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	webhook_module "code.gitea.io/gitea/modules/webhook"

	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/git"
	_ "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

type testMigrationRepository struct {
	ID        int64 `xorm:"pk autoincr"`
	OwnerName string
	Name      string
}

func (testMigrationRepository) TableName() string { return "repository" }

type testMigrationActionRun struct {
	ID           int64 `xorm:"pk autoincr"`
	RepoID       int64 `xorm:"index"`
	Index        int64
	CommitSHA    string `xorm:"commit_sha"`
	Event        webhook_module.HookEventType
	TriggerEvent string
	EventPayload string
}

func (testMigrationActionRun) TableName() string { return "action_run" }

type testMigrationActionRunJob struct {
	ID    int64 `xorm:"pk autoincr"`
	RunID int64 `xorm:"index"`
}

func (testMigrationActionRunJob) TableName() string { return "action_run_job" }

type testMigrationCommitStatus struct {
	ID        int64 `xorm:"pk autoincr"`
	RepoID    int64 `xorm:"index"`
	SHA       string
	TargetURL string
}

func (testMigrationCommitStatus) TableName() string { return "commit_status" }

type testMigrationCommitStatusSummary struct {
	ID        int64  `xorm:"pk autoincr"`
	RepoID    int64  `xorm:"index"`
	SHA       string `xorm:"VARCHAR(64) NOT NULL"`
	State     string `xorm:"VARCHAR(7) NOT NULL"`
	TargetURL string
}

func (testMigrationCommitStatusSummary) TableName() string { return "commit_status_summary" }

func Test_FixCommitStatusTargetURLToUseRunAndJobID(t *testing.T) {
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	x, deferable := base.PrepareTestEnv(t, 0,
		new(testMigrationRepository),
		new(testMigrationActionRun),
		new(testMigrationActionRunJob),
		new(testMigrationCommitStatus),
		new(testMigrationCommitStatusSummary),
	)
	defer deferable()

	_, err := x.Insert(
		&testMigrationRepository{ID: 100, OwnerName: "testuser", Name: "repo1"},

		&testMigrationActionRun{
			ID:           990,
			RepoID:       100,
			Index:        7,
			CommitSHA:    "merge-sha",
			Event:        webhook_module.HookEventPullRequest,
			EventPayload: toJSON(t, &api.PullRequestPayload{PullRequest: &api.PullRequest{Head: &api.PRBranchInfo{Sha: "sha-shared"}}}),
		},
		&testMigrationActionRun{
			ID:           991,
			RepoID:       100,
			Index:        8,
			CommitSHA:    "sha-shared",
			Event:        webhook_module.HookEventPush,
			EventPayload: toJSON(t, &api.PushPayload{HeadCommit: &api.PayloadCommit{ID: "sha-shared"}}),
		},
		&testMigrationActionRun{
			ID:        1991,
			RepoID:    100,
			Index:     9,
			CommitSHA: "sha-other",
			Event:     webhook_module.HookEventRelease,
		},

		&testMigrationActionRunJob{ID: 997, RunID: 990},
		&testMigrationActionRunJob{ID: 998, RunID: 990},
		&testMigrationActionRunJob{ID: 1997, RunID: 991},
		&testMigrationActionRunJob{ID: 1998, RunID: 1991},

		&testMigrationCommitStatus{ID: 10010, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/7/jobs/0"},
		&testMigrationCommitStatus{ID: 10011, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/7/jobs/1"},
		&testMigrationCommitStatus{ID: 10012, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/8/jobs/0"},
		&testMigrationCommitStatus{ID: 10013, RepoID: 100, SHA: "sha-other", TargetURL: "/testuser/repo1/actions/runs/9/jobs/0"}, // run_id > 1000, do not update

		&testMigrationCommitStatus{ID: 10014, RepoID: 100, SHA: "sha-shared", TargetURL: "/otheruser/badrepo/actions/runs/7/jobs/0"},  // invalid repo
		&testMigrationCommitStatus{ID: 10015, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/10/jobs/0"},    // invalid run_id
		&testMigrationCommitStatus{ID: 10016, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/7/jobs/3"},     // invalid job_id
		&testMigrationCommitStatus{ID: 10017, RepoID: 100, SHA: "sha-shared", TargetURL: "https://ci.example.com/build/123"},          // external URL
		&testMigrationCommitStatus{ID: 10018, RepoID: 100, SHA: "sha-shared", TargetURL: "/testuser/repo1/actions/runs/990/jobs/997"}, // already ID-based URL, no updates needed

		&testMigrationCommitStatusSummary{ID: 10020, RepoID: 100, SHA: "sha-shared", State: "pending", TargetURL: "/testuser/repo1/actions/runs/7/jobs/0"},
		&testMigrationCommitStatusSummary{ID: 10021, RepoID: 100, SHA: "sha-other", State: "pending", TargetURL: "/testuser/repo1/actions/runs/9/jobs/0"}, // run_id > 1000, do not update
	)
	require.NoError(t, err)

	require.NoError(t, FixCommitStatusTargetURLToUseRunAndJobID(x))

	cases := []struct {
		table string
		id    int64
		want  string
	}{
		{table: "commit_status", id: 10010, want: "/testuser/repo1/actions/runs/990/jobs/997"},
		{table: "commit_status", id: 10011, want: "/testuser/repo1/actions/runs/990/jobs/998"},
		{table: "commit_status", id: 10012, want: "/testuser/repo1/actions/runs/991/jobs/1997"},
		{table: "commit_status", id: 10013, want: "/testuser/repo1/actions/runs/9/jobs/0"},
		{table: "commit_status", id: 10014, want: "/otheruser/badrepo/actions/runs/7/jobs/0"},
		{table: "commit_status", id: 10015, want: "/testuser/repo1/actions/runs/10/jobs/0"},
		{table: "commit_status", id: 10016, want: "/testuser/repo1/actions/runs/7/jobs/3"},
		{table: "commit_status", id: 10017, want: "https://ci.example.com/build/123"},
		{table: "commit_status", id: 10018, want: "/testuser/repo1/actions/runs/990/jobs/997"},
		{table: "commit_status_summary", id: 10020, want: "/testuser/repo1/actions/runs/990/jobs/997"},
		{table: "commit_status_summary", id: 10021, want: "/testuser/repo1/actions/runs/9/jobs/0"},
	}

	for _, tc := range cases {
		assertTargetURL(t, x, tc.table, tc.id, tc.want)
	}
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func assertTargetURL(t *testing.T, x *xorm.Engine, table string, id int64, want string) {
	t.Helper()

	var row struct {
		TargetURL string
	}
	has, err := x.Table(table).Where("id=?", id).Cols("target_url").Get(&row)
	require.NoError(t, err)
	require.Truef(t, has, "row not found: table=%s id=%d", table, id)
	require.Equal(t, want, row.TargetURL)
}
