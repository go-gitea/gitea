// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestGetBlobBySHA(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	defer ctx.Repo.GitRepo.Close()

	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	ctx.SetParams(":id", "1")
	ctx.SetParams(":sha", sha)

	gbr, err := GetBlobBySHA(ctx.Repo.Repository, ctx.Params(":sha"))
	expectedGBR := &api.GitBlobResponse{
		Content:  "dHJlZSAyYTJmMWQ0NjcwNzI4YTJlMTAwNDllMzQ1YmQ3YTI3NjQ2OGJlYWI2CmF1dGhvciB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+IDE0ODk5NTY0NzkgLTA0MDAKY29tbWl0dGVyIEV0aGFuIEtvZW5pZyA8ZXRoYW50a29lbmlnQGdtYWlsLmNvbT4gMTQ4OTk1NjQ3OSAtMDQwMAoKSW5pdGlhbCBjb21taXQK",
		Encoding: "base64",
		URL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/65f1bf27bc3bf70f64657658635e66094edbcb4d",
		SHA:      "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Size:     180,
	}
	assert.Nil(t, err)
	assert.Equal(t, expectedGBR, gbr)
}
