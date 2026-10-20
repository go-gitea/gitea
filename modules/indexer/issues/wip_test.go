// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/indexer/issues/bleve"
	"gitea.dev/modules/indexer/issues/internal"
	"gitea.dev/modules/optional"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryWIPIndexerSync(t *testing.T) {
	t.Run("recovery", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			calls := 0
			start := time.Now()
			err := retryWIPIndexerSync(t.Context(), func() error {
				calls++
				if calls == 1 {
					return errors.New("indexer unavailable")
				}
				return nil
			})
			require.NoError(t, err)
			assert.Equal(t, 2, calls)
			assert.Equal(t, 30*time.Second, time.Since(start))
		})
	})
	t.Run("shutdown during retry", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			var err error
			go func() {
				err = retryWIPIndexerSync(ctx, func() error {
					calls++
					return errors.New("indexer unavailable")
				})
			}()
			synctest.Wait()
			cancel()
			synctest.Wait()
			assert.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, 1, calls)
		})
	})
}

type wipTestIndexer struct {
	internal.Indexer
	err   error
	calls int
}

func (i *wipTestIndexer) Index(ctx context.Context, data ...*internal.IndexerData) error {
	i.calls++
	if i.err != nil {
		return i.err
	}
	return i.Indexer.Index(ctx, data...)
}

func TestSyncWIPIndexer(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	defer test.MockVariableValue(&setting.Indexer.IssueType, "bleve")()
	defer test.MockVariableValue(&setting.Indexer.IssuePath, t.TempDir())()
	defer test.MockVariableValue(&setting.Repository.PullRequest.WorkInProgressPrefixes, []string{"[WIP]"})()
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	issue.Title = "[wip] changes"
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "name"))
	indexer := &wipTestIndexer{Indexer: bleve.NewIndexer(setting.Indexer.IssuePath)}
	defer indexer.Close()
	_, err := indexer.Init(t.Context())
	require.NoError(t, err)
	prefixes, err := issues_model.SyncIssueWIP(t.Context())
	require.NoError(t, err)
	require.NoError(t, syncWIPIndexer(t.Context(), indexer, prefixes, true))
	result, err := indexer.Search(t.Context(), &SearchOptions{IsWIP: optional.Some(true)})
	require.NoError(t, err)
	assert.EqualValues(t, 1, result.Total)
	calls := indexer.calls
	require.NoError(t, syncWIPIndexer(t.Context(), indexer, prefixes, true))
	assert.Equal(t, calls, indexer.calls, "unchanged prefixes must not reindex")

	setting.Repository.PullRequest.WorkInProgressPrefixes = nil
	prefixes, err = issues_model.SyncIssueWIP(t.Context())
	require.NoError(t, err)
	indexer.err = errors.New("index unavailable")
	require.ErrorIs(t, syncWIPIndexer(t.Context(), indexer, prefixes, true), indexer.err)
	indexer.err = nil
	require.NoError(t, syncWIPIndexer(t.Context(), indexer, prefixes, true))
	result, err = indexer.Search(t.Context(), &SearchOptions{IsWIP: optional.Some(true)})
	require.NoError(t, err)
	assert.Zero(t, result.Total, "retry after an interrupted refresh must remove stale WIP flags")

	issue.Title = "ready"
	require.NoError(t, issues_model.UpdateIssueCols(t.Context(), issue, "name"))
	data, exists, err := getIssueIndexerData(t.Context(), issue.ID)
	require.NoError(t, err)
	require.True(t, exists)
	assert.False(t, data.IsWIP)
}
