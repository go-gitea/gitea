// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestGetBlobBySHA(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadRepoCommit(t, ctx)
	test.LoadUser(t, ctx, 2)
	test.LoadGitRepo(t, ctx)
	sha := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	ctx.SetParams(":id", "1")
	ctx.SetParams(":sha", sha)

	gbr, err := GetBlobBySHA(ctx.Repo.Repository, ctx.Params(":sha"))
	expectedGBR := &api.GitBlobResponse{
		Content:  "Y29tbWl0IDY1ZjFiZjI3YmMzYmY3MGY2NDY1NzY1ODYzNWU2NjA5NGVkYmNiNGQKQXV0aG9yOiB1c2VyMSA8YWRkcmVzczFAZXhhbXBsZS5jb20+CkRhdGU6ICAgU3VuIE1hciAxOSAxNjo0Nzo1OSAyMDE3IC0wNDAwCgogICAgSW5pdGlhbCBjb21taXQKCmRpZmYgLS1naXQgYS9SRUFETUUubWQgYi9SRUFETUUubWQKbmV3IGZpbGUgbW9kZSAxMDA2NDQKaW5kZXggMDAwMDAwMC4uNGI0ODUxYQotLS0gL2Rldi9udWxsCisrKyBiL1JFQURNRS5tZApAQCAtMCwwICsxLDMgQEAKKyMgcmVwbzEKKworRGVzY3JpcHRpb24gZm9yIHJlcG8xClwgTm8gbmV3bGluZSBhdCBlbmQgb2YgZmlsZQo=",
		Encoding: "base64",
		URL:      "https://try.gitea.io/api/v1/repos/user2/repo1/git/blobs/65f1bf27bc3bf70f64657658635e66094edbcb4d",
		SHA:      "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Size:     180,
	}
	assert.Nil(t, err)
	assert.Equal(t, expectedGBR, gbr)
}
