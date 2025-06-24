// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository_test

import (
	"bytes"
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestGarbageCollectLFSMetaObjects(t *testing.T) {
	unittest.PrepareTestEnv(t)

	setting.LFS.StartServer = true
	err := storage.Init()
	assert.NoError(t, err)

	repo, err := repo_model.GetRepositoryByOwnerAndName(db.DefaultContext, "user2", "repo1")
	assert.NoError(t, err)

	// add lfs object
	lfsContent := []byte("gitea1")
	lfsOid := storeObjectInRepo(t, repo.ID, &lfsContent)

	// gc
	err = repo_service.GarbageCollectLFSMetaObjects(t.Context(), repo_service.GarbageCollectLFSMetaObjectsOptions{
		AutoFix:                 true,
		OlderThan:               time.Now().Add(7 * 24 * time.Hour).Add(5 * 24 * time.Hour),
		UpdatedLessRecentlyThan: time.Now().Add(7 * 24 * time.Hour).Add(3 * 24 * time.Hour),
	})
	assert.NoError(t, err)

	// lfs meta has been deleted
	_, err = git_model.GetLFSMetaObjectByOid(db.DefaultContext, repo.ID, lfsOid)
	assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
}

func storeObjectInRepo(t *testing.T, repositoryID int64, content *[]byte) string {
	pointer, err := lfs.GeneratePointer(bytes.NewReader(*content))
	assert.NoError(t, err)

	_, err = git_model.NewLFSMetaObject(db.DefaultContext, repositoryID, pointer)
	assert.NoError(t, err)
	contentStore := lfs.NewContentStore()
	exist, err := contentStore.Exists(pointer)
	assert.NoError(t, err)
	if !exist {
		err := contentStore.Put(pointer, bytes.NewReader(*content))
		assert.NoError(t, err)
	}
	return pointer.Oid
}
