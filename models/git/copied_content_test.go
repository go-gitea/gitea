// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssociateCopiedContentLFSInsertAndReplay(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	p1 := lfs.Pointer{Oid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 11}
	p2 := lfs.Pointer{Oid: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Size: 22}
	pointers := []git_model.CopiedContentLFSPointer{
		{Oid: p1.Oid, Size: p1.Size},
		{Oid: p2.Oid, Size: p2.Size},
		{Oid: p1.Oid, Size: p1.Size}, // duplicate oid in the same batch
	}

	require.NoError(t, git_model.AssociateCopiedContentLFS(ctx, repo.ID, pointers))

	got1, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, p1.Oid)
	require.NoError(t, err)
	assert.Equal(t, p1.Size, got1.Size)
	got2, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, p2.Oid)
	require.NoError(t, err)
	assert.Equal(t, p2.Size, got2.Size)

	count, err := db.GetEngine(ctx).Where("repository_id = ? AND oid = ?", repo.ID, p1.Oid).Count(new(git_model.LFSMetaObject))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	require.NoError(t, git_model.AssociateCopiedContentLFS(ctx, repo.ID, pointers))
	got1Again, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, p1.Oid)
	require.NoError(t, err)
	assert.Equal(t, got1.ID, got1Again.ID)
}

func TestAssociateCopiedContentLFSSizeConflictPrecheck(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	existing := lfs.Pointer{Oid: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", Size: 5}
	_, err := git_model.NewLFSMetaObject(ctx, repo.ID, existing)
	require.NoError(t, err)

	first := lfs.Pointer{Oid: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", Size: 6}
	last := lfs.Pointer{Oid: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Size: 7}
	err = git_model.AssociateCopiedContentLFS(ctx, repo.ID, []git_model.CopiedContentLFSPointer{
		{Oid: first.Oid, Size: first.Size},
		{Oid: existing.Oid, Size: existing.Size + 1},
		{Oid: last.Oid, Size: last.Size},
	})
	require.ErrorIs(t, err, git_model.ErrCopiedContentSizeConflict)

	_, err = git_model.GetLFSMetaObjectByOid(ctx, repo.ID, first.Oid)
	assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
	_, err = git_model.GetLFSMetaObjectByOid(ctx, repo.ID, last.Oid)
	assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
	got, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, existing.Oid)
	require.NoError(t, err)
	assert.Equal(t, existing.Size, got.Size)
}

func TestAssociateCopiedContentLFSRequestSizeConflict(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	err := git_model.AssociateCopiedContentLFS(t.Context(), 1, []git_model.CopiedContentLFSPointer{
		{Oid: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", Size: 1},
		{Oid: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", Size: 2},
	})
	require.ErrorIs(t, err, git_model.ErrCopiedContentSizeConflict)
}

func TestAssociateCopiedContentLFSConcurrentDuplicate(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pointer := git_model.CopiedContentLFSPointer{
		Oid:  "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
		Size: 9,
	}

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = git_model.AssociateCopiedContentLFS(ctx, repo.ID, []git_model.CopiedContentLFSPointer{pointer})
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}
	count, err := db.GetEngine(ctx).Where("repository_id = ? AND oid = ?", repo.ID, pointer.Oid).Count(new(git_model.LFSMetaObject))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

// Abort actual INSERTs, including after earlier rows have been written in the transaction.
func TestAssociateCopiedContentLFSInsertFailureRollback(t *testing.T) {
	if !setting.Database.Type.IsSQLite3() {
		t.Skip("SQLite trigger fault injection")
	}
	for failureIndex := range 3 {
		t.Run(fmt.Sprintf("insert_%d", failureIndex+1), func(t *testing.T) {
			require.NoError(t, unittest.PrepareTestDatabase())
			ctx := t.Context()
			existing := lfs.Pointer{Oid: strings.Repeat("9", 64), Size: 9}
			original, err := git_model.NewLFSMetaObject(ctx, 1, existing)
			require.NoError(t, err)
			pointers := []git_model.CopiedContentLFSPointer{
				{Oid: strings.Repeat("a", 64), Size: 1},
				{Oid: strings.Repeat("b", 64), Size: 2},
				{Oid: strings.Repeat("c", 64), Size: 3},
			}
			_, err = db.GetEngine(ctx).Exec("CREATE TRIGGER copied_content_insert_failure BEFORE INSERT ON lfs_meta_object WHEN NEW.oid = '" + pointers[failureIndex].Oid + "' BEGIN SELECT RAISE(ABORT, 'injected copy insert failure'); END")
			require.NoError(t, err)
			defer func() {
				_, err := db.GetEngine(ctx).Exec("DROP TRIGGER copied_content_insert_failure")
				require.NoError(t, err)
			}()
			err = git_model.AssociateCopiedContentLFS(ctx, 1, append([]git_model.CopiedContentLFSPointer{{Oid: existing.Oid, Size: existing.Size}}, pointers...))
			require.ErrorContains(t, err, "injected copy insert failure")
			for _, pointer := range pointers {
				_, err := git_model.GetLFSMetaObjectByOid(ctx, 1, pointer.Oid)
				assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
			}
			kept, err := git_model.GetLFSMetaObjectByOid(ctx, 1, existing.Oid)
			require.NoError(t, err)
			assert.Equal(t, original.ID, kept.ID)
			assert.Equal(t, existing.Size, kept.Size)
		})
	}
}
