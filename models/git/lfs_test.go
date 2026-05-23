// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/timeutil"

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
