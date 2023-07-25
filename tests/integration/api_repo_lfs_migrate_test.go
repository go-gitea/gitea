// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"path"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/services/migrations"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoLFSMigrateLocal(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	oldImportLocalPaths := setting.ImportLocalPaths
	oldAllowLocalNetworks := setting.Migrations.AllowLocalNetworks
	setting.ImportLocalPaths = true
	setting.Migrations.AllowLocalNetworks = true
	assert.NoError(t, migrations.Init())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate?token="+token, &api.MigrateRepoOptions{
		CloneAddr:   path.Join(setting.RepoRootPath, "migration/lfs-test.git"),
		RepoOwnerID: user.ID,
		RepoName:    "lfs-test-local",
		LFS:         true,
	})
	resp := MakeRequest(t, req, NoExpectedStatus)
	assert.EqualValues(t, http.StatusCreated, resp.Code)

	store := lfs.NewContentStore()
	ok, _ := store.Verify(lfs.Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6})
	assert.True(t, ok)
	ok, _ = store.Verify(lfs.Pointer{Oid: "d6f175817f886ec6fbbc1515326465fa96c3bfd54a4ea06cfd6dbbd8340e0152", Size: 6})
	assert.True(t, ok)

	setting.ImportLocalPaths = oldImportLocalPaths
	setting.Migrations.AllowLocalNetworks = oldAllowLocalNetworks
	assert.NoError(t, migrations.Init()) // reset old migration settings
}
