// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	git_module "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestViewHomeSubmoduleRedirect(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "/user2/repo1/src/branch/master/test-submodule")
	submodule := git_module.NewCommitSubmoduleFile("/user2/repo1", "test-submodule", "../repo-other", "any-ref-id")
	handleRepoViewSubmodule(ctx, submodule)
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Equal(t, "/user2/repo-other/tree/any-ref-id", ctx.Resp.Header().Get("Location"))

	ctx, _ = contexttest.MockContext(t, "/user2/repo1/src/branch/master/test-submodule")
	submodule = git_module.NewCommitSubmoduleFile("/user2/repo1", "test-submodule", "https://other/user2/repo-other.git", "any-ref-id")
	handleRepoViewSubmodule(ctx, submodule)
	// do not auto-redirect for external URLs, to avoid open redirect or phishing
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())

	ctx, respWriter := contexttest.MockContext(t, "/user2/repo1/src/branch/master/test-submodule?only_content=true")
	submodule = git_module.NewCommitSubmoduleFile("/user2/repo1", "test-submodule", "../repo-other", "any-ref-id")
	handleRepoViewSubmodule(ctx, submodule)
	assert.Equal(t, http.StatusOK, ctx.Resp.WrittenStatus())
	assert.Equal(t, `<a href="/user2/repo-other/tree/any-ref-id">/user2/repo-other/tree/any-ref-id</a>`, respWriter.Body.String())
}
