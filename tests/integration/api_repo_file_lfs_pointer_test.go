// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/lfs"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A client that holds a file's bytes cannot stream them through the contents
// API: it takes base64 in a JSON body. So it puts the object in LFS itself and
// commits the pointer. Without is_lfs_pointer, a path .gitattributes routes
// through LFS has that pointer hashed into a *second* object and a pointer to
// it committed -- a file whose content is a pointer, with the real bytes
// orphaned.

// trackBinThroughLFS puts a .gitattributes in the fixture repository that
// routes *.bin through LFS, which is the condition under which a committed
// pointer would otherwise be re-encoded.
func trackBinThroughLFS(t *testing.T, token, repoFullName, branch string) {
	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/api/v1/repos/%s/contents/.gitattributes", repoFullName),
		&api.CreateFileOptions{
			FileOptions:   api.FileOptions{BranchName: branch, Message: "track bin through lfs"},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("*.bin filter=lfs diff=lfs merge=lfs -text\n")),
		}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func seedLFSObject(t *testing.T, repoID int64, content string) lfs.Pointer {
	pointer, err := lfs.GeneratePointer(strings.NewReader(content))
	require.NoError(t, err)
	_, err = git_model.NewLFSMetaObject(t.Context(), repoID, pointer)
	require.NoError(t, err)
	store := lfs.NewContentStore()
	exists, err := store.Exists(pointer)
	require.NoError(t, err)
	if !exists {
		require.NoError(t, store.Put(pointer, strings.NewReader(content)))
	}
	return pointer
}

func committedBlob(t *testing.T, token, repoFullName, branch, treePath string) string {
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/contents/%s?ref=%s", repoFullName, treePath, branch)).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)
	var body api.ContentsResponse
	DecodeJSON(t, resp, &body)
	require.NotNil(t, body.Content)
	raw, err := base64.StdEncoding.DecodeString(*body.Content)
	require.NoError(t, err)
	return string(raw)
}

func createWithPointer(t *testing.T, token, repoFullName, branch, treePath string, pointer lfs.Pointer, declare bool) *httptest.ResponseRecorder {
	req := NewRequestWithJSON(t, "POST",
		fmt.Sprintf("/api/v1/repos/%s/contents/%s", repoFullName, treePath),
		&api.CreateFileOptions{
			FileOptions:   api.FileOptions{BranchName: branch, Message: "commit a pointer"},
			ContentBase64: base64.StdEncoding.EncodeToString([]byte(pointer.StringContent())),
			IsLFSPointer:  declare,
		}).AddTokenAuth(token)
	return MakeRequest(t, req, NoExpectedStatus)
}

func TestAPICreateFileAsLFSPointer(t *testing.T) {
	// onGiteaRun, not PrepareTestEnv: committing pushes, the pre-receive hook
	// calls back into the private API, and without the running server that
	// call fails as a connection error the write reports as a refusal.
	onGiteaRun(t, testAPICreateFileAsLFSPointer)
}

func testAPICreateFileAsLFSPointer(t *testing.T, _ *url.URL) {
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	session := loginUser(t, user.Name)
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	branch := repo.DefaultBranch
	trackBinThroughLFS(t, token, repo.FullName(), branch)

	t.Run("a declared pointer is committed exactly as given", func(t *testing.T) {
		pointer := seedLFSObject(t, repo.ID, "the real recording bytes")

		resp := createWithPointer(t, token, repo.FullName(), branch, "data/declared.bin", pointer, true)
		assert.Equal(t, http.StatusCreated, resp.Code)

		blob := committedBlob(t, token, repo.FullName(), branch, "data/declared.bin")
		assert.Equal(t, pointer.StringContent(), blob)
		assert.Contains(t, blob, pointer.Oid, "the committed pointer names the object that was uploaded")
	})

	t.Run("without the flag the same request stores a pointer to a pointer", func(t *testing.T) {
		// Pinned deliberately: this is the behaviour is_lfs_pointer exists to
		// avoid, and a change to it should fail here rather than in a clone.
		pointer := seedLFSObject(t, repo.ID, "other real recording bytes")

		resp := createWithPointer(t, token, repo.FullName(), branch, "data/undeclared.bin", pointer, false)
		assert.Equal(t, http.StatusCreated, resp.Code)

		blob := committedBlob(t, token, repo.FullName(), branch, "data/undeclared.bin")
		assert.NotEqual(t, pointer.StringContent(), blob)
		assert.NotContains(t, blob, pointer.Oid, "the real object is no longer named by anything in the tree")
	})

	t.Run("a pointer to an object this repository does not hold is refused", func(t *testing.T) {
		absent := lfs.Pointer{Oid: strings.Repeat("a", 64), Size: 1234}

		resp := createWithPointer(t, token, repo.FullName(), branch, "data/dangling.bin", absent, true)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code,
			"committing it would put a file in the tree that no clone can resolve")
	})

	t.Run("content that is not a pointer at all is refused", func(t *testing.T) {
		req := NewRequestWithJSON(t, "POST",
			fmt.Sprintf("/api/v1/repos/%s/contents/data/garbage.bin", repo.FullName()),
			&api.CreateFileOptions{
				FileOptions:   api.FileOptions{BranchName: branch, Message: "not a pointer"},
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("just some bytes")),
				IsLFSPointer:  true,
			}).AddTokenAuth(token)
		MakeRequest(t, req, http.StatusUnprocessableEntity)
	})
}
