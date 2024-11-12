// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/url"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/private"
	"code.gitea.io/gitea/services/context"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitLFSSSH(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		dstPath := t.TempDir()
		apiTestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		var mu sync.Mutex
		var routerCalls []string
		web.RouteMock(private.RouterMockPointInternalLFS, func(ctx *context.PrivateContext) {
			mu.Lock()
			routerCalls = append(routerCalls, ctx.Req.Method+" "+ctx.Req.URL.Path)
			mu.Unlock()
		})

		withKeyFile(t, "my-testing-key", func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(apiTestContext, "test-key", keyFile))
			cloneURL := createSSHUrl(apiTestContext.GitPath(), u)
			t.Run("Clone", doGitClone(dstPath, cloneURL))

			cfg, err := setting.CfgProvider.PrepareSaving()
			require.NoError(t, err)
			cfg.Section("server").Key("LFS_ALLOW_PURE_SSH").SetValue("true")
			setting.LFS.AllowPureSSH = true
			require.NoError(t, cfg.Save())

			// do LFS SSH transfer?
			lfsCommitAndPushTest(t, dstPath, 10)
		})

		// FIXME: Here we only see the following calls, but actually there should be calls to "PUT"?
		// 0 = {string} "GET /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 1 = {string} "POST /api/internal/repo/user2/repo1.git/info/lfs/objects/batch"
		// 2 = {string} "GET /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 3 = {string} "POST /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 4 = {string} "GET /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 5 = {string} "GET /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 6 = {string} "GET /api/internal/repo/user2/repo1.git/info/lfs/locks"
		// 7 = {string} "POST /api/internal/repo/user2/repo1.git/info/lfs/locks/24/unlock"
		assert.NotEmpty(t, routerCalls)
		// assert.Contains(t, routerCalls, "PUT /api/internal/repo/user2/repo1.git/info/lfs/objects/....")
	})
}
