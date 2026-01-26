// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"io"
	"net/http"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/web"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepoDownloadArchive(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.EnableGzip, true)()
	defer test.MockVariableValue(&web.GzipMinSize, 10)()
	defer test.MockVariableValue(&testWebRoutes, routers.NormalRoutes())()

	t.Run("NoDuplicateCompression", func(t *testing.T) {
		req := NewRequest(t, "GET", "/user2/repo1/archive/master.zip")
		req.Header.Set("Accept-Encoding", "gzip")
		resp := MakeRequest(t, req, http.StatusOK)
		bs, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Empty(t, resp.Header().Get("Content-Encoding"))
		assert.Len(t, bs, 320)
	})

	t.Run("SubPath", func(t *testing.T) {
		// When using "archiving and caching" approach, archiving with paths will always use streaming and never be cached
		defer test.MockVariableValue(&setting.Repository.StreamArchives, false) // this can be removed if there is always streaming mode
		req := NewRequest(t, "GET", "/user2/glob/archive/master.tar.gz?path=aaa.doc&path=x/y")
		resp := MakeRequest(t, req, http.StatusOK)
		content, err := test.ReadAllTarGzContent(resp.Body)
		require.NoError(t, err)
		assert.Empty(t, content["glob/a.txt"])
		assert.NotEmpty(t, content["glob/aaa.doc"])
		assert.Empty(t, content["glob/x/b.txt"])
		assert.NotEmpty(t, content["glob/x/y/a.txt"])

		req = NewRequest(t, "GET", "/user2/glob/archive/master.tar.gz")
		resp = MakeRequest(t, req, http.StatusOK)
		content, err = test.ReadAllTarGzContent(resp.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, content["glob/a.txt"])
		assert.NotEmpty(t, content["glob/aaa.doc"])
		assert.NotEmpty(t, content["glob/x/b.txt"])
		assert.NotEmpty(t, content["glob/x/y/a.txt"])
	})
}
