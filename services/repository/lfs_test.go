// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository_test

import (
	"bytes"
	"testing"
	"time"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"

	"github.com/stretchr/testify/assert"
)

func TestGarbageCollectLFSMetaObjects(t *testing.T) {
	unittest.PrepareTestEnv(t)

	defer test.MockVariableValue(&setting.LFS.StartServer, true)()

	err := storage.Init()
	assert.NoError(t, err)

	repo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
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
	_, err = git_model.GetLFSMetaObjectByOid(t.Context(), repo.ID, lfsOid)
	assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
}

func TestGarbageCollectLFSMetaObjectsForRepoAutoFix(t *testing.T) {
	unittest.PrepareTestEnv(t)

	defer test.MockVariableValue(&setting.LFS.StartServer, true)()

	err := storage.Init()
	assert.NoError(t, err)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	// add lfs object
	lfsContent := []byte("gitea2")
	lfsOid := storeObjectInRepo(t, repo.ID, &lfsContent)

	err = repo_service.GarbageCollectLFSMetaObjectsForRepo(t.Context(), repo, repo_service.GarbageCollectLFSMetaObjectsOptions{
		LogDetail:               func(string, ...any) {},
		AutoFix:                 true,
		OlderThan:               time.Now().Add(24 * time.Hour * 7),
		UpdatedLessRecentlyThan: time.Now().Add(24 * time.Hour * 3),
	})
	assert.NoError(t, err)

	_, err = git_model.GetLFSMetaObjectByOid(t.Context(), repo.ID, lfsOid)
	assert.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
}

func storeObjectInRepo(t *testing.T, repositoryID int64, content *[]byte) string {
	pointer, err := lfs.GeneratePointer(bytes.NewReader(*content))
	assert.NoError(t, err)

	_, err = git_model.NewLFSMetaObject(t.Context(), repositoryID, pointer)
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
