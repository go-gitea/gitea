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
		ID     int64 `xorm:"pk autoincr"`
		RepoID int64 `xorm:"index"`
		Index  int64
	}

	type ActionRunJob struct {
		ID    int64 `xorm:"pk autoincr"`
		RunID int64 `xorm:"index"`
	}

	type CommitStatus struct {
		ID        int64 `xorm:"pk autoincr"`
		RepoID    int64 `xorm:"index"`
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

	newURL1 := "/testuser/repo1/actions/runs/106/jobs/530"
	newURL2 := "/testuser/repo1/actions/runs/106/jobs/531"

	invalidWrongRepo := "/otheruser/badrepo/actions/runs/7/jobs/0"
	invalidNonexistentRun := "/testuser/repo1/actions/runs/10/jobs/0"
	invalidNonexistentJob := "/testuser/repo1/actions/runs/7/jobs/3"
	externalTargetURL := "https://ci.example.com/build/123"

	require.NoError(t, FixCommitStatusTargetURLToUseRunAndJobID(x))

	cases := []struct {
		table string
		id    int64
		want  string
	}{
		{table: "commit_status", id: 10, want: newURL1},
		{table: "commit_status", id: 11, want: newURL2},
		{table: "commit_status", id: 12, want: invalidWrongRepo},
		{table: "commit_status", id: 13, want: invalidNonexistentRun},
		{table: "commit_status", id: 14, want: invalidNonexistentJob},
		{table: "commit_status", id: 15, want: externalTargetURL},
		{table: "commit_status_summary", id: 20, want: newURL1},
		{table: "commit_status_summary", id: 21, want: externalTargetURL},
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
