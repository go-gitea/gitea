// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	packages_module "code.gitea.io/gitea/modules/packages"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	packages_service "code.gitea.io/gitea/services/packages"

	"github.com/stretchr/testify/assert"
)

func TestMigratePackages(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	creator := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	content := "package main\n\nfunc main() {\nfmt.Println(\"hi\")\n}\n"
	buf, err := packages_module.CreateHashedBufferFromReaderWithSize(strings.NewReader(content), 1024)
	assert.NoError(t, err)
	defer buf.Close()

	v, f, err := packages_service.CreatePackageAndAddFile(db.DefaultContext, &packages_service.PackageCreationInfo{
		PackageInfo: packages_service.PackageInfo{
			Owner:       creator,
			PackageType: packages.TypeGeneric,
			Name:        "test",
			Version:     "1.0.0",
		},
		Creator:           creator,
		SemverCompatible:  true,
		VersionProperties: map[string]string{},
	}, &packages_service.PackageFileCreationInfo{
		PackageFileInfo: packages_service.PackageFileInfo{
			Filename: "a.go",
		},
		Creator: creator,
		Data:    buf,
		IsLead:  true,
	})
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.NotNil(t, f)

	ctx := t.Context()

	p := t.TempDir()

	dstStorage, err := storage.NewLocalStorage(
		ctx,
		&setting.Storage{
			Path: p,
		})
	assert.NoError(t, err)

	err = migratePackages(ctx, dstStorage)
	assert.NoError(t, err)

	entries, err := os.ReadDir(p)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.EqualValues(t, "01", entries[0].Name())
	assert.EqualValues(t, "tmp", entries[1].Name())
}
