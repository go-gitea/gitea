// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/routers"
	"gitea.dev/routers/web"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoDownloadArchive(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.EnableGzip, true)()
	defer test.MockVariableValue(&web.GzipMinSize, 10)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, git.ForceFastImport(t.Context(), repo.CodeStorageRepo(), []git.FastImportCommit{{
		Ref: "refs/heads/archive-test",
		Files: []git.FastImportFile{
			{Path: "a.txt", Content: "root text file\n"},
			{Path: "aaa.doc", Content: "root document\n"},
			{Path: "x/b.txt", Content: "nested text file\n"},
			{Path: "x/y/a.txt", Content: "deeply nested text file\n"},
		},
	}}))

	t.Run("NoDuplicateCompression", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/archive/master.zip")
		req.Header.Set("Accept-Encoding", "gzip")
		resp := MakeRequest(t, req, http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Empty(t, resp.Header().Get("Content-Encoding"))
		assert.Len(t, bs, 320)
	})

	t.Run("Conditional", func(t *testing.T) {
		defer test.MockVariableValue(&setting.Repository.StreamArchives, true)()
		archiveURL := "/user2/repo1/archive/archive-test.tar.gz"
		etag := MakeRequest(t, NewRequest(t, "GET", archiveURL), http.StatusOK).Header().Get("ETag")
		assert.True(t, strings.HasPrefix(etag, `W/"`))
		MakeRequest(t, NewRequest(t, "GET", archiveURL).SetHeader("If-None-Match", etag), http.StatusNotModified)
		MakeRequest(t, NewRequest(t, "GET", archiveURL+"?path=aaa.doc").SetHeader("If-None-Match", etag), http.StatusOK)
		assert.NotEqual(t, etag, MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/archive/master.tar.gz"), http.StatusOK).Header().Get("ETag"))

		restoreStreaming := test.MockVariableValue(&setting.Repository.StreamArchives, false)
		MakeRequest(t, NewRequest(t, "GET", archiveURL).SetHeader("If-None-Match", etag), http.StatusOK)
		restoreStreaming()

		defer test.MockVariableValue(&setting.Repository.PrefixArchiveFiles, !setting.Repository.PrefixArchiveFiles)()
		MakeRequest(t, NewRequest(t, "GET", archiveURL).SetHeader("If-None-Match", etag), http.StatusOK)
	})

	t.Run("SubPath", func(t *testing.T) {
		// When using "archiving and caching" approach, archiving with paths will always use streaming and never be cached
		defer test.MockVariableValue(&setting.Repository.StreamArchives, false)() // this can be removed if there is always streaming mode
		req := NewRequest(t, "GET", "/user2/repo1/archive/archive-test.tar.gz?path=aaa.doc&path=x/y")
		resp := MakeRequest(t, req, http.StatusOK)
		content, err := test.ReadAllTarGzContent(resp.Body)
		require.NoError(t, err)
		assert.Empty(t, content["repo1/a.txt"])
		assert.NotEmpty(t, content["repo1/aaa.doc"])
		assert.Empty(t, content["repo1/x/b.txt"])
		assert.NotEmpty(t, content["repo1/x/y/a.txt"])

		req = NewRequest(t, "GET", "/user2/repo1/archive/archive-test.tar.gz")
		resp = MakeRequest(t, req, http.StatusOK)
		content, err = test.ReadAllTarGzContent(resp.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, content["repo1/a.txt"])
		assert.NotEmpty(t, content["repo1/aaa.doc"])
		assert.NotEmpty(t, content["repo1/x/b.txt"])
		assert.NotEmpty(t, content["repo1/x/y/a.txt"])
	})
}
