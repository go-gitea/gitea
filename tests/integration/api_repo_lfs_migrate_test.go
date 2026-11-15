// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
	"testing"

	auth_model "gitea.dev/models/auth"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"
	files_service "gitea.dev/services/repository/files"
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

// createRepoWithLFSPointers creates a repository with two LFS pointer files:
// the object of "own.bin" belongs to the new repository, the object of "other.bin" only belongs to repo1.
func createRepoWithLFSPointers(t *testing.T, owner *user_model.User, name string) (repo *repo_model.Repository, ownPointer, otherPointer lfs.Pointer) {
	t.Helper()
	repo, err := repo_service.CreateRepository(t.Context(), owner, owner, repo_service.CreateRepoOptions{
		Name:          name,
		AutoInit:      true,
		Readme:        "Default",
		DefaultBranch: "master",
	})
	require.NoError(t, err)

	ownContent, otherContent := "object of "+name, "object of repo1, referenced by "+name
	ownPointer = lfs.Pointer{Oid: storeObjectInRepo(t, repo.ID, ownContent), Size: int64(len(ownContent))}
	otherPointer = lfs.Pointer{Oid: storeObjectInRepo(t, 1, otherContent), Size: int64(len(otherContent))}

	_, err = files_service.ChangeRepoFiles(t.Context(), repo, owner, &files_service.ChangeRepoFilesOptions{
		Files: []*files_service.ChangeRepoFile{
			{Operation: "create", TreePath: "own.bin", ContentReader: strings.NewReader(ownPointer.StringContent())},
			{Operation: "create", TreePath: "other.bin", ContentReader: strings.NewReader(otherPointer.StringContent())},
		},
		OldBranch: repo.DefaultBranch,
		NewBranch: repo.DefaultBranch,
		Message:   "add LFS pointers",
	})
	require.NoError(t, err)
	return repo, ownPointer, otherPointer
}

func TestAPIRepoLFSMigrateObjects(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		token := getUserToken(t, user.Name, auth_model.AccessTokenScopeWriteRepository)
		srcRepo, ownPointer, otherPointer := createRepoWithLFSPointers(t, user, "lfs-migrate-src")

		migrate := func(t *testing.T, repoName, lfsEndpoint string) {
			req := NewRequestWithJSON(t, "POST", "/api/v1/repos/migrate", &api.MigrateRepoOptions{
				CloneAddr:   u.String() + srcRepo.FullName() + ".git",
				RepoOwnerID: user.ID,
				RepoName:    repoName,
				LFS:         true,
				LFSEndpoint: lfsEndpoint,
			}).AddTokenAuth(token)
			MakeRequest(t, req, NoExpectedStatus)
		}

		t.Run("OnlyObjectsServedByRemote", func(t *testing.T) {
			migrate(t, "lfs-migrate-dst", "")
			dstRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerID: user.ID, Name: "lfs-migrate-dst"})
			unittest.AssertExistsAndLoadBean(t, &git_model.LFSMetaObject{RepositoryID: dstRepo.ID, Oid: ownPointer.Oid})
			unittest.AssertNotExistsBean(t, &git_model.LFSMetaObject{RepositoryID: dstRepo.ID, Oid: otherPointer.Oid})
		})

		t.Run("RemoteServesOtherContent", func(t *testing.T) {
			var downloads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					downloads.Add(1)
					_, _ = w.Write([]byte("other content"))
					return
				}
				var batchReq lfs.BatchRequest
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&batchReq))
				batchResp := &lfs.BatchResponse{}
				for _, p := range batchReq.Objects {
					batchResp.Objects = append(batchResp.Objects, &lfs.ObjectResponse{
						Pointer: p,
						Actions: map[string]*lfs.Link{"download": lfs.NewLink("http://" + r.Host + "/object")},
					})
				}
				w.Header().Set("Content-Type", lfs.MediaType)
				assert.NoError(t, json.NewEncoder(w).Encode(batchResp))
			}))
			defer server.Close()

			migrate(t, "lfs-migrate-dst-other-content", server.URL)
			assert.NotZero(t, downloads.Load())
			unittest.AssertCount(t, &git_model.LFSMetaObject{Oid: ownPointer.Oid}, 2) // srcRepo and the first migration
			unittest.AssertCount(t, &git_model.LFSMetaObject{Oid: otherPointer.Oid}, 1)
			for _, p := range []lfs.Pointer{ownPointer, otherPointer} {
				ok, err := lfs.NewContentStore().Verify(p)
				assert.NoError(t, err)
				assert.True(t, ok, "existing object must be kept")
			}
		})
	})
}
