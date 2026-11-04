// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestIterateLFSMetaObjectsForRepoUpdatesDoNotSkip(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	ctx := t.Context()
	repo, err := repo_model.GetRepositoryByOwnerAndName(ctx, "user2", "repo1")
	assert.NoError(t, err)

	defer test.MockVariableValue(&setting.Database.IterateBufferSize, 1)()

	created := make([]*git_model.LFSMetaObject, 0, 3)
	for i := range 3 {
		content := []byte("gitea-lfs-" + strconv.Itoa(i))
		pointer, err := lfs.GeneratePointer(bytes.NewReader(content))
		assert.NoError(t, err)

		meta, err := git_model.NewLFSMetaObject(ctx, repo.ID, pointer)
		assert.NoError(t, err)
		created = append(created, meta)
	}

	iterated := make([]int64, 0, len(created))
	cutoff := time.Now().Add(24 * time.Hour)
	iterErr := git_model.IterateLFSMetaObjectsForRepo(ctx, repo.ID, func(ctx context.Context, meta *git_model.LFSMetaObject, count int64) error {
		iterated = append(iterated, meta.ID)
		_, err := db.GetEngine(ctx).ID(meta.ID).Cols("updated_unix").Update(&git_model.LFSMetaObject{
			UpdatedUnix: timeutil.TimeStamp(time.Now().Unix()),
		})
		return err
	}, &git_model.IterateLFSMetaObjectsForRepoOptions{
		OlderThan:               timeutil.TimeStamp(cutoff.Unix()),
		UpdatedLessRecentlyThan: timeutil.TimeStamp(cutoff.Unix()),
	})
	assert.NoError(t, iterErr)

	expected := []int64{created[0].ID, created[1].ID, created[2].ID}
	assert.Equal(t, expected, iterated)
}

func TestLFSDeletionInvalidatesQuotaUsage(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	p := lfs.Pointer{Oid: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 1024}
	_, err := git_model.NewLFSMetaObject(ctx, 1, p)
	assert.NoError(t, err)
	assert.NoError(t, quota_model.StoreUsage(ctx, &quota_model.Usage{RepoID: 1, OwnerID: 2, LFSBytes: 1024}))
	allowed, err := quota_model.RequestGC(ctx, 1)
	assert.NoError(t, err)
	assert.True(t, allowed)
	previous, _, err := quota_model.GetUsage(ctx, 1)
	assert.NoError(t, err)
	_, err = git_model.RemoveLFSMetaObjectByOidFn(ctx, 1, p.Oid, func(int64) error { return context.Canceled })
	assert.ErrorIs(t, err, context.Canceled)
	rolledBack, found, err := quota_model.GetUsage(ctx, 1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, previous.Updated, rolledBack.Updated)
	assert.Equal(t, previous.GCRequested, rolledBack.GCRequested)
	_, err = git_model.GetLFSMetaObjectByOid(ctx, 1, p.Oid)
	assert.NoError(t, err)
	_, err = git_model.RemoveLFSMetaObjectByOid(ctx, 1, p.Oid)
	assert.NoError(t, err)
	usage, found, err := quota_model.GetUsage(ctx, 1)
	assert.NoError(t, err)
	assert.True(t, found)
	assert.Zero(t, usage.Updated)
	assert.Equal(t, previous.GCRequested, usage.GCRequested)
	assert.NoError(t, quota_model.StoreUsage(ctx, &quota_model.Usage{RepoID: 1, OwnerID: 2}))
	allowed, err = quota_model.RequestGC(ctx, 1)
	assert.NoError(t, err)
	assert.False(t, allowed)
}
