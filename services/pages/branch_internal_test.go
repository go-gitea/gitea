// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package pages

import (
	"strings"
	"testing"
	"time"

	"gitea.dev/models/db"
	pages_model "gitea.dev/models/pages"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/require"
)

func TestSetBranchCheckResultOnlyUpdatesCurrentSource(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	site := &pages_model.Site{
		RepoID: 1, Enabled: true, SourceType: pages_model.PublishSourceBranch,
		SourceBranch: "master", SourcePath: "docs", PagesUpdatedUnixNano: 10,
	}
	require.NoError(t, pages_model.InsertSite(t.Context(), site))
	initial := *site

	_, err := db.GetEngine(t.Context()).ID(site.RepoID).
		Cols("enabled", "pages_updated_unix_nano").
		Update(&pages_model.Site{Enabled: false, PagesUpdatedUnixNano: 20})
	require.NoError(t, err)
	require.NoError(t, setBranchCheckResult(t.Context(), &initial, strings.Repeat("a", 40), pages_model.SourceErrorArchiveFailed, time.Minute))

	stored, has, err := pages_model.GetSiteByRepoID(t.Context(), site.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	require.False(t, stored.Enabled)
	require.Equal(t, int64(20), stored.PagesUpdatedUnixNano)
	require.Equal(t, strings.Repeat("a", 40), stored.SourceCommitID)
	require.Equal(t, pages_model.SourceErrorArchiveFailed, stored.SourceErrorCode)

	_, err = db.GetEngine(t.Context()).ID(site.RepoID).
		Cols("deleted", "source_commit_id", "source_error_code").
		Update(&pages_model.Site{Deleted: true})
	require.NoError(t, err)
	require.NoError(t, setBranchCheckResult(t.Context(), &initial, strings.Repeat("b", 40), pages_model.SourceErrorCommitFailed, time.Minute))
	stored, has, err = pages_model.GetSiteByRepoID(t.Context(), site.RepoID)
	require.NoError(t, err)
	require.True(t, has)
	require.True(t, stored.Deleted)
	require.Empty(t, stored.SourceCommitID)
	require.Empty(t, stored.SourceErrorCode)
}
