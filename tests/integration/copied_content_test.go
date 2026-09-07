// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFinalizeCopiedContentInternalRoute(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		defer test.MockVariableValue(&setting.LFS.StartServer, true)()

		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		sourceRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		destRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})

		payload := "copied-lfs-bytes"
		pointer, err := lfs.GeneratePointer(strings.NewReader(payload))
		require.NoError(t, err)
		_, err = git_model.NewLFSMetaObject(t.Context(), sourceRepo.ID, pointer)
		require.NoError(t, err)
		require.NoError(t, lfs.NewContentStore().Put(pointer, strings.NewReader(payload)))

		sourceCommit := createCopiedContentFileAPI(t, sourceRepo, "finalize-src.bin", pointer.StringContent())
		destCommit, destParent := createCopiedContentFileAPIWithParent(t, destRepo, "finalize-dst.bin", pointer.StringContent())

		body := repo_service.FinalizeCopiedContentRequest{
			RequestID: uuid.NewString(),
			UserID:    user.ID,
			Destination: repo_service.CopiedContentDestination{
				RepositoryID: destRepo.ID,
				Branch:       destRepo.DefaultBranch,
				Commit:       destCommit,
				Parent:       &destParent,
			},
			Files: []repo_service.CopiedContentFileMapping{{
				SourceRepositoryID: sourceRepo.ID,
				SourceCommit:       sourceCommit,
				SourcePath:         "finalize-src.bin",
				DestinationPath:    "finalize-dst.bin",
				Representation:     repo_service.CopiedContentRepresentationLFS,
			}},
		}

		req := NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body)
		MakeRequest(t, req, http.StatusForbidden)

		req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		resp := MakeRequest(t, req, http.StatusOK)
		var result repo_service.FinalizeCopiedContentResponse
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
		assert.Equal(t, "finalized", result.Status)
		assert.Equal(t, destCommit, result.Commit)
		require.Len(t, result.Files, 1)
		assert.Equal(t, "ready", result.Files[0].Status)

		meta, err := git_model.GetLFSMetaObjectByOid(t.Context(), destRepo.ID, pointer.Oid)
		require.NoError(t, err)
		assert.Equal(t, pointer.Size, meta.Size)

		req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		resp = MakeRequest(t, req, http.StatusOK)
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
		assert.Equal(t, "ready", result.Files[0].Status)

		gitContent := "plain-git"
		sourceGit := createCopiedContentFileAPI(t, sourceRepo, "finalize-plain.txt", gitContent)
		destGit, destGitParent := createCopiedContentFileAPIWithParent(t, destRepo, "finalize-plain.txt", gitContent)
		gitBody := body
		gitBody.RequestID = uuid.NewString()
		gitBody.Destination.Commit = destGit
		gitBody.Destination.Parent = &destGitParent
		gitBody.Files = []repo_service.CopiedContentFileMapping{{
			SourceRepositoryID: sourceRepo.ID,
			SourceCommit:       sourceGit,
			SourcePath:         "finalize-plain.txt",
			DestinationPath:    "finalize-plain.txt",
			Representation:     repo_service.CopiedContentRepresentationGit,
		}}
		req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", gitBody).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		MakeRequest(t, req, http.StatusOK)

		bad := body
		bad.RequestID = uuid.NewString()
		bad.Destination.Parent = &destCommit
		req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", bad).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		resp = MakeRequest(t, req, http.StatusConflict)
		assert.Contains(t, resp.Body.String(), `"code":"conflict"`)

		oversized := strings.Repeat("a", repo_service.CopiedContentMaxJSONBytes+2)
		req = NewRequestWithBody(t, "POST", "/api/internal/repositories/finalize-copied-content", strings.NewReader(`{"`+oversized)).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken).
			SetHeader("Content-Type", "application/json")
		MakeRequest(t, req, http.StatusRequestEntityTooLarge)

		unknown := `{"request_id":"` + uuid.NewString() + `","user_id":2,"destination":{"repository_id":1,"branch":"master","commit":"` + destCommit + `","parent":null},"files":[{"source_repository_id":2,"source_commit":"` + sourceCommit + `","source_path":"src.bin","destination_path":"dst.bin","representation":"lfs"}],"extra":true}`
		req = NewRequestWithBody(t, "POST", "/api/internal/repositories/finalize-copied-content", strings.NewReader(unknown)).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken).
			SetHeader("Content-Type", "application/json")
		MakeRequest(t, req, http.StatusBadRequest)
	})
}

func TestFinalizeCopiedContentInternalRouteDeniedSource(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 19})
		destRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
		destCommit, destParent := createCopiedContentFileAPIWithParent(t, destRepo, "finalize-denied.txt", "x")

		body := repo_service.FinalizeCopiedContentRequest{
			RequestID: uuid.NewString(),
			UserID:    user.ID,
			Destination: repo_service.CopiedContentDestination{
				RepositoryID: destRepo.ID,
				Branch:       destRepo.DefaultBranch,
				Commit:       destCommit,
				Parent:       &destParent,
			},
			Files: []repo_service.CopiedContentFileMapping{{
				SourceRepositoryID: privateRepo.ID,
				SourceCommit:       strings.Repeat("a", 40),
				SourcePath:         "README.md",
				DestinationPath:    "finalize-denied.txt",
				Representation:     repo_service.CopiedContentRepresentationGit,
			}},
		}
		req := NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body).
			SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		resp := MakeRequest(t, req, http.StatusNotFound)
		assert.Contains(t, resp.Body.String(), `"code":"not_found"`)
		assert.NotContains(t, resp.Body.String(), `"index"`)
	})
}

func createCopiedContentFileAPI(t *testing.T, repo *repo_model.Repository, treePath, content string) string {
	t.Helper()
	commit, _ := createCopiedContentFileAPIWithParent(t, repo, treePath, content)
	return commit
}

func createCopiedContentFileAPIWithParent(t *testing.T, repo *repo_model.Repository, treePath, content string) (string, string) {
	t.Helper()
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	opts := api.CreateFileOptions{
		FileOptions: api.FileOptions{
			Message:    "add " + treePath,
			BranchName: repo.DefaultBranch,
		},
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(content)),
	}
	req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", repo.OwnerName, repo.Name, treePath), opts).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusCreated)
	var fileResp api.FileResponse
	DecodeJSON(t, resp, &fileResp)
	require.NotNil(t, fileResp.Commit)
	require.NotEmpty(t, fileResp.Commit.Parents)
	return fileResp.Commit.SHA, fileResp.Commit.Parents[0].SHA
}

func TestFinalizeCopiedContentInternalRouteMaterializeBatch(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		defer test.MockVariableValue(&setting.LFS.StartServer, true)()
		source := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
		dest := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 16})
		payloads := []string{"materialize first payload", "materialize second payload"}
		createCopiedContentFileAPI(t, source, "batch-a", payloads[0])
		sourceCommit := createCopiedContentFileAPI(t, source, "batch-b", payloads[1])
		token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
		opts := api.ChangeFilesOptions{FileOptions: api.FileOptions{BranchName: dest.DefaultBranch, Message: "publish copied batch"}}
		pointers := make([]lfs.Pointer, 2)
		for i, payload := range payloads {
			var err error
			pointers[i], err = lfs.GeneratePointer(strings.NewReader(payload))
			require.NoError(t, err)
			opts.Files = append(opts.Files, &api.ChangeFileOperation{Operation: "create", Path: fmt.Sprintf("batch-%c", 'a'+i), ContentBase64: base64.StdEncoding.EncodeToString([]byte(pointers[i].StringContent()))})
		}
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/api/v1/repos/%s/%s/contents", dest.OwnerName, dest.Name), opts).AddTokenAuth(token)
		published := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.FilesResponse{})
		require.NotNil(t, published.Commit)
		require.Len(t, published.Commit.Parents, 1)
		parent := published.Commit.Parents[0].SHA
		body := repo_service.FinalizeCopiedContentRequest{RequestID: uuid.NewString(), UserID: 2, Destination: repo_service.CopiedContentDestination{RepositoryID: dest.ID, Branch: dest.DefaultBranch, Commit: published.Commit.SHA, Parent: &parent}}
		for i, pointer := range pointers {
			exists, err := lfs.NewContentStore().Exists(pointer)
			require.NoError(t, err)
			require.False(t, exists, "regular payload must first be materialized by finalizer")
			_, err = git_model.GetLFSMetaObjectByOid(t.Context(), dest.ID, pointer.Oid)
			require.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
			treePath := fmt.Sprintf("batch-%c", 'a'+i)
			body.Files = append(body.Files, repo_service.CopiedContentFileMapping{SourceRepositoryID: source.ID, SourceCommit: sourceCommit, SourcePath: treePath, DestinationPath: treePath, Representation: repo_service.CopiedContentRepresentationLFS})
		}
		// An unavailable second source must not finalize even the valid first mapping.
		bad := body
		bad.Files = append([]repo_service.CopiedContentFileMapping(nil), body.Files...)
		bad.Files[1].SourcePath = "missing-source"
		req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", bad).SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
		MakeRequest(t, req, http.StatusNotFound)
		for _, pointer := range pointers {
			_, err := git_model.GetLFSMetaObjectByOid(t.Context(), dest.ID, pointer.Oid)
			require.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
		}
		// A real local storage failure after publication must return 503 without links.
		func() {
			storageRoot := t.TempDir()
			blockedTemp := filepath.Join(storageRoot, "not-a-directory")
			require.NoError(t, os.WriteFile(blockedTemp, []byte("block temp creation"), 0o600))
			failedStorage, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: storageRoot, TemporaryPath: blockedTemp})
			require.NoError(t, err)
			defer test.MockVariableValue(&storage.LFS, failedStorage)()
			req := NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body).SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
			MakeRequest(t, req, http.StatusServiceUnavailable)
			for _, pointer := range pointers {
				_, err := git_model.GetLFSMetaObjectByOid(t.Context(), dest.ID, pointer.Oid)
				require.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
			}
		}()
		var firstResponse string
		for attempt := range 2 {
			req = NewRequestWithJSON(t, "POST", "/api/internal/repositories/finalize-copied-content", body).SetHeader("X-Gitea-Internal-Auth", "Bearer "+setting.InternalToken)
			response := MakeRequest(t, req, http.StatusOK)
			if attempt == 0 {
				firstResponse = response.Body.String()
			} else {
				assert.Equal(t, firstResponse, response.Body.String())
			}
			var result repo_service.FinalizeCopiedContentResponse
			DecodeJSON(t, response, &result)
			require.Len(t, result.Files, 2)
			for i, file := range result.Files {
				assert.Equal(t, "ready", file.Status)
				assert.Equal(t, i, file.Index)
			}
		}
		for i, pointer := range pointers {
			req = NewRequest(t, "GET", fmt.Sprintf("/%s/%s.git/info/lfs/objects/%s", dest.OwnerName, dest.Name, pointer.Oid))
			req.SetBasicAuth("user2", token)
			response := MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, payloads[i], response.Body.String())
		}
	})
}
