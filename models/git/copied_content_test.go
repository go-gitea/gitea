// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"sync"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"

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

func TestAssociateCopiedContentLFSSizeConflictRollsBack(t *testing.T) {
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
