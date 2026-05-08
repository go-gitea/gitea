// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package tests

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models/db"
	packages_model "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/testlogger"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers"

	"github.com/stretchr/testify/assert"
)

func InitIntegrationTest() error {
	testlogger.Init()

	err := setting.PrepareIntegrationTestConfig()
	if err != nil {
		return err
	}

	setting.SetupGiteaTestEnv()
	setting.Repository.DefaultBranch = "master" // many test code still assume that default branch is called "master"

	if err := git.InitFull(); err != nil {
		return err
	}

	setting.Database.SlowQueryThreshold = 0
	setting.LoadDBSetting()
	cleanupDb, err := unittest.ResetTestDatabase()
	if err != nil {
		return err
	}
	_ = cleanupDb // no clean up yet (not really needed at the moment)

	if err := storage.Init(); err != nil {
		return err
	}

	routers.InitWebInstalled(graceful.GetManager().HammerContext())
	return nil
}

func PrepareAttachmentsStorage(t testing.TB) {
	// prepare attachments directory and files
	assert.NoError(t, storage.Clean(storage.Attachments))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "attachments"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.Attachments, p, s, p)
		return err
	}))
}

func PrepareGitRepoDirectory(t testing.TB) {
	if !assert.NotEmpty(t, setting.RepoRootPath) {
		return
	}
	assert.NoError(t, unittest.SyncDirs(filepath.Join(setting.GetGiteaTestSourceRoot(), "tests/gitea-repositories-meta"), setting.RepoRootPath))
}

func PrepareArtifactsStorage(t testing.TB) {
	// prepare actions artifacts directory and files
	assert.NoError(t, storage.Clean(storage.ActionsArtifacts))

	s, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests", "testdata", "data", "artifacts"),
	})
	assert.NoError(t, err)
	assert.NoError(t, s.IterateObjects("", func(p string, obj storage.Object) error {
		_, err = storage.Copy(storage.ActionsArtifacts, p, s, p)
		return err
	}))
}

func PrepareLFSStorage(t testing.TB) {
	// load LFS object fixtures
	// (LFS storage can be on any of several backends, including remote servers, so init it with the storage API)
	lfsFixtures, err := storage.NewStorage(setting.LocalStorageType, &setting.Storage{
		Path: filepath.Join(filepath.Dir(setting.AppPath), "tests/gitea-lfs-meta"),
	})
	assert.NoError(t, err)
	assert.NoError(t, storage.Clean(storage.LFS))
	assert.NoError(t, lfsFixtures.IterateObjects("", func(path string, _ storage.Object) error {
		_, err := storage.Copy(storage.LFS, path, lfsFixtures, path)
		return err
	}))
}

func PrepareCleanPackageData(t testing.TB) {
	// clear all package data
	assert.NoError(t, db.TruncateBeans(t.Context(),
		&packages_model.Package{},
		&packages_model.PackageVersion{},
		&packages_model.PackageFile{},
		&packages_model.PackageBlob{},
		&packages_model.PackageProperty{},
		&packages_model.PackageBlobUpload{},
		&packages_model.PackageCleanupRule{},
	))
	assert.NoError(t, storage.Clean(storage.Packages))
}

func PrepareTestEnv(t testing.TB, skip ...int) func() {
	t.Helper()
	deferFn := PrintCurrentTest(t, util.OptionalArg(skip)+1)

	// load database fixtures
	assert.NoError(t, unittest.LoadFixtures())

	// do not add more Prepare* functions here, only call necessary ones in the related test functions
	PrepareGitRepoDirectory(t)
	PrepareLFSStorage(t)
	PrepareCleanPackageData(t)
	return deferFn
}

func PrintCurrentTest(t testing.TB, skip ...int) func() {
	t.Helper()
	return testlogger.PrintCurrentTest(t, util.OptionalArg(skip)+1)
}
