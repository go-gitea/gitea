// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/lfs"
	repo_module "gitea.dev/modules/repository"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoLFSMigrateLocal(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.ImportLocalPaths, true)()

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)

	req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &api.MigrateRepoOptions{
		CloneAddr:   path.Join(setting.RepoRootPath, "migration/lfs-test.git"),
		RepoOwnerID: user.ID,
		RepoName:    "lfs-test-local",
		LFS:         true,
	}).AddTokenAuth(token)
	resp := MakeRequest(t, req, NoExpectedStatus)
	assert.Equal(t, http.StatusCreated, resp.Code)

	store := lfs.NewContentStore()
	ok, _ := store.Verify(lfs.Pointer{Oid: "fb8f7d8435968c4f82a726a92395be4d16f2f63116caf36c8ad35c60831ab041", Size: 6})
	assert.True(t, ok)
	ok, _ = store.Verify(lfs.Pointer{Oid: "d6f175817f886ec6fbbc1515326465fa96c3bfd54a4ea06cfd6dbbd8340e0152", Size: 6})
	assert.True(t, ok)
}

func splitLFSFixtureLinks(t *testing.T) (linked, unlinked lfs.Pointer) {
	t.Helper()
	linkedMeta := unittest.AssertExistsAndLoadBean(t, &git_model.LFSMetaObject{ID: 1, RepositoryID: 54})
	unlinkedMeta := unittest.AssertExistsAndLoadBean(t, &git_model.LFSMetaObject{ID: 2, RepositoryID: 54})
	_, err := db.GetEngine(t.Context()).ID(unlinkedMeta.ID).Cols("repository_id").Update(&git_model.LFSMetaObject{RepositoryID: 1})
	require.NoError(t, err)
	return linkedMeta.Pointer, unlinkedMeta.Pointer
}

func TestAPIRepoLFSMigrateObjects(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		srcRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 54})
		linked, unlinked := splitLFSFixtureLinks(t)

		t.Run("OnlyLinkedObjects", func(t *testing.T) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &api.MigrateRepoOptions{
				CloneAddr:    u.String() + srcRepo.FullName() + ".git",
				AuthUsername: user.Name,
				AuthPassword: userPassword,
				RepoOwnerID:  user.ID,
				RepoName:     "lfs-migrate-dst",
				LFS:          true,
			}).AddTokenAuth(getUserToken(t, user.Name, auth_model.AccessTokenScopeWriteRepository))
			dstRepo := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Repository{})
			unittest.AssertExistsAndLoadBean(t, &git_model.LFSMetaObject{RepositoryID: dstRepo.ID, Oid: linked.Oid})
			unittest.AssertNotExistsBean(t, &git_model.LFSMetaObject{RepositoryID: dstRepo.ID, Oid: unlinked.Oid})
		})

		t.Run("WrongContentKeepsStoredObject", func(t *testing.T) {
			root := t.TempDir()
			dir := path.Join("lfs", "objects", unlinked.Oid[:2], unlinked.Oid[2:4])
			files := test.NewFileHelper(root)
			files.MkdirAll(t, dir)
			files.WriteFile(t, path.Join(dir, unlinked.Oid), strings.Repeat("x", int(unlinked.Size)))
			client, err := lfs.NewClientFromEndpoint(root, "", nil)
			require.NoError(t, err)
			gitRepo, err := git.OpenRepository(t.Context(), srcRepo)
			require.NoError(t, err)
			defer gitRepo.Close()

			err = repo_module.StoreMissingLfsObjectsInRepository(t.Context(), srcRepo, gitRepo, client)
			require.ErrorIs(t, err, lfs.ErrHashMismatch)
			unittest.AssertNotExistsBean(t, &git_model.LFSMetaObject{RepositoryID: srcRepo.ID, Oid: unlinked.Oid})
			content, err := lfs.ReadMetaObject(unlinked)
			require.NoError(t, err)
			defer content.Close()
			stored, err := lfs.GeneratePointer(content)
			require.NoError(t, err)
			assert.Equal(t, unlinked, stored)
		})
	})
}
