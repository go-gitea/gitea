// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
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
