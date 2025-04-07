// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	git_module "code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/contexttest"

	"github.com/stretchr/testify/assert"
)

func TestViewHomeSubmoduleRedirect(t *testing.T) {
	unittest.PrepareTestEnv(t)

	ctx, _ := contexttest.MockContext(t, "/user2/repo1/src/branch/master/test-submodule")
	submodule := &git_module.SubModule{Path: "test-submodule", URL: setting.AppURL + "user2/repo-other.git"}
	handleRepoViewSubmodule(ctx, submodule)
	assert.Equal(t, http.StatusSeeOther, ctx.Resp.WrittenStatus())
	assert.Equal(t, "/user2/repo-other", ctx.Resp.Header().Get("Location"))

	ctx, _ = contexttest.MockContext(t, "/user2/repo1/src/branch/master/test-submodule")
	submodule = &git_module.SubModule{Path: "test-submodule", URL: "https://other/user2/repo-other.git"}
	handleRepoViewSubmodule(ctx, submodule)
	// do not auto-redirect for external URLs, to avoid open redirect or phishing
	assert.Equal(t, http.StatusNotFound, ctx.Resp.WrittenStatus())
}
