// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"testing"

	"code.gitea.io/gitea/models/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"

	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/git"
	_ "code.gitea.io/gitea/models/repo"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

func Test_FixCommitStatusTargetURLToUseRunAndJobID(t *testing.T) {
	defer test.MockVariableValue(&setting.AppSubURL, "")()

	type Repository struct {
		ID        int64 `xorm:"pk autoincr"`
		OwnerName string
		Name      string
	}

	type ActionRun struct {
		ID           int64 `xorm:"pk autoincr"`
		RepoID       int64 `xorm:"index"`
		Index        int64
		CommitSHA    string `xorm:"commit_sha"`
		Event        string
		TriggerEvent string
		EventPayload string `xorm:"LONGTEXT"`
	}

	type ActionRunJob struct {
		ID    int64 `xorm:"pk autoincr"`
		RunID int64 `xorm:"index"`
	}

	type CommitStatus struct {
		ID        int64 `xorm:"pk autoincr"`
		RepoID    int64 `xorm:"index"`
		SHA       string
		TargetURL string
	}

	type CommitStatusSummary struct {
		ID        int64  `xorm:"pk autoincr"`
		RepoID    int64  `xorm:"index"`
		SHA       string `xorm:"VARCHAR(64) NOT NULL"`
		State     string `xorm:"VARCHAR(7) NOT NULL"`
		TargetURL string
	}

	x, deferable := base.PrepareTestEnv(t, 0,
		new(Repository),
		new(ActionRun),
		new(ActionRunJob),
		new(CommitStatus),
		new(CommitStatusSummary),
	)
	defer deferable()

	require.NoError(t, FixCommitStatusTargetURLToUseRunAndJobID(x))

	cases := []struct {
		table string
		id    int64
		want  string
	}{
		// Legacy URLs for runs whose resolved run IDs are below the threshold should be rewritten.
		{table: "commit_status", id: 10010, want: "/testuser/repo1/actions/runs/990/jobs/997"},
		{table: "commit_status", id: 10011, want: "/testuser/repo1/actions/runs/990/jobs/998"},
		{table: "commit_status", id: 10012, want: "/testuser/repo1/actions/runs/991/jobs/1997"},

		// Runs whose resolved IDs are above the threshold are intentionally left unchanged.
		{table: "commit_status", id: 10013, want: "/testuser/repo1/actions/runs/9/jobs/0"},

		// URLs that do not resolve cleanly as legacy Actions URLs should remain untouched.
		{table: "commit_status", id: 10014, want: "/otheruser/badrepo/actions/runs/7/jobs/0"},
		{table: "commit_status", id: 10015, want: "/testuser/repo1/actions/runs/10/jobs/0"},
		{table: "commit_status", id: 10016, want: "/testuser/repo1/actions/runs/7/jobs/3"},
		{table: "commit_status", id: 10017, want: "https://ci.example.com/build/123"},

		// Already ID-based URLs are valid inputs and should not be rewritten again.
		{table: "commit_status", id: 10018, want: "/testuser/repo1/actions/runs/990/jobs/997"},

		// The same rewrite rules apply to commit_status_summary rows.
		{table: "commit_status_summary", id: 10020, want: "/testuser/repo1/actions/runs/990/jobs/997"},
		{table: "commit_status_summary", id: 10021, want: "/testuser/repo1/actions/runs/9/jobs/0"},
	}

	for _, tc := range cases {
		assertTargetURL(t, x, tc.table, tc.id, tc.want)
	}
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
