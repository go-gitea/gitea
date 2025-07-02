// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/contexttest"

	_ "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func TestGetContents(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo1")
	ctx.SetPathParam("id", "1")
	contexttest.LoadRepo(t, ctx, 1)
	contexttest.LoadRepoCommit(t, ctx)
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadGitRepo(t, ctx)

	// GetContentsOrList's behavior is fully tested in integration tests, so we don't need to test it here.

	t.Run("GetBlobBySHA", func(t *testing.T) {
		sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
		ctx.SetPathParam("id", "1")
		ctx.SetPathParam("sha", sha)
		gbr, err := GetBlobBySHA(ctx.Repo.Repository, ctx.Repo.GitRepo, ctx.PathParam("sha"))
		expectedGBR := &api.GitBlobResponse{
			Content:  util.ToPointer("dHJlZSAyYTJmMWQ0NjcwNzI4YTJlMTAwNDllMzQ1YmQ3YTI3NjQ2OGJlYWI2CmF1dGhvciB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+IDE0ODk5NTY0NzkgLTA0MDAKY29tbWl0dGVyIEV0aGFuIEtvZW5pZyA8ZXRoYW50a29lbmlnQGdtYWlsLmNvbT4gMTQ4OTk1NjQ3OSAtMDQwMAoKSW5pdGlhbCBjb21taXQK"),
			Encoding: util.ToPointer("base64"),
			URL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/65f1bf27bc3bf70f64657658635e66094edbcb4d",
			SHA:      "65f1bf27bc3bf70f64657658635e66094edbcb4d",
			Size:     180,
		}
		assert.NoError(t, err)
		assert.Equal(t, expectedGBR, gbr)
	})
}
