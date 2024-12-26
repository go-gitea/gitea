// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo_test

import (
	"context"
	"testing"

	_ "code.gitea.io/gitea/models/actions"
	_ "code.gitea.io/gitea/models/activities"
	_ "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	unittest.MainTest(m)
}

func Test_GetTreePathLatestCommit(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	commitID, err := gitrepo.GetBranchCommitID(context.Background(), repo, repo.DefaultBranch)
	assert.NoError(t, err)
	assert.EqualValues(t, "1032bbf17fbc0d9c95bb5418dabe8f8c99278700", commitID)

	commit, err := gitrepo.GetTreePathLatestCommit(context.Background(), repo, repo.DefaultBranch, "Home.md")
	assert.NoError(t, err)
	assert.NotNil(t, commit)
	assert.EqualValues(t, "2c54faec6c45d31c1abfaecdab471eac6633738a", commit.ID.String())
}
