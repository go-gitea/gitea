// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/system"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueWIPLifecycle(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Repository.PullRequest.WorkInProgressPrefixes, []string{"[WIP]", "草稿:"})()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	issue := &issues_model.Issue{RepoID: repo.ID, PosterID: doer.ID, Poster: doer, Title: "  [wip] new PR  "}
	require.NoError(t, issues_model.NewPullRequest(t.Context(), repo, issue, nil, nil, &issues_model.PullRequest{BaseRepoID: repo.ID, HeadRepoID: repo.ID}))
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID}).IsWIP)

	for _, title := range []string{"ready", "草稿: changes", "ready again"} {
		oldTitle := issue.Title
		issue.Title = title
		require.NoError(t, issues_model.ChangeIssueTitle(t.Context(), issue, doer, oldTitle))
		stored := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue.ID})
		assert.Equal(t, issues_model.HasWorkInProgressPrefix(title), stored.IsWIP)
	}

	imported := &issues_model.Issue{RepoID: repo.ID, Index: 100, PosterID: doer.ID, IsPull: true, Title: "草稿: imported"}
	ordinary := &issues_model.Issue{RepoID: repo.ID, Index: 101, PosterID: doer.ID, Title: "[WIP] ordinary issue", IsWIP: true}
	require.NoError(t, issues_model.InsertIssues(t.Context(), imported, ordinary))
	assert.True(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: imported.ID}).IsWIP)
	assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: ordinary.ID}).IsWIP)
}

func TestSyncIssueWIP(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Repository.PullRequest.WorkInProgressPrefixes)()
	defer test.MockVariableValue(&setting.Database.IterateBufferSize, 1)()

	// Raw writes model rows created before the WIP column existed.
	for id, title := range map[int64]string{1: "[WIP] ordinary issue", 2: "[wip] PR", 3: "草稿: PR"} {
		_, err := db.GetEngine(t.Context()).ID(id).Cols("name").NoAutoTime().Update(&issues_model.Issue{Title: title})
		require.NoError(t, err)
	}
	before := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	for _, tc := range []struct {
		prefixes   []string
		wip2, wip3 bool
	}{
		{[]string{"[WIP]"}, true, false},
		{[]string{"草稿:"}, false, true},
		{nil, false, false},
		{[]string{""}, true, true},
	} {
		setting.Repository.PullRequest.WorkInProgressPrefixes = tc.prefixes
		_, err := issues_model.SyncIssueWIP(t.Context())
		require.NoError(t, err)
		assert.Equal(t, tc.wip2, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}).IsWIP)
		assert.Equal(t, tc.wip3, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 3}).IsWIP)
		assert.False(t, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1}).IsWIP)
		assert.Equal(t, before.UpdatedUnix, unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2}).UpdatedUnix)
	}
	previous := unittest.AssertExistsAndLoadBean(t, &system.Setting{SettingKey: "issue_wip_prefixes"})
	_, err := issues_model.SyncIssueWIP(t.Context())
	require.NoError(t, err)
	assert.Equal(t, previous.Version, unittest.AssertExistsAndLoadBean(t, &system.Setting{ID: previous.ID}).Version)
}
