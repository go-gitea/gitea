// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	gocontext "context"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/common"
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
		web.RouteMock(common.RouterMockPointCommonLFS, func(ctx *context.Base) {
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

			_, _, cmdErr := git.NewCommand(gocontext.Background(), "config", "lfs.sshtransfer", "always").RunStdString(&git.RunOpts{Dir: dstPath})
			assert.NoError(t, cmdErr)
			lfsCommitAndPushTest(t, dstPath, 10)
		})

		countBatch := slices.ContainsFunc(routerCalls, func(s string) bool {
			return strings.Contains(s, "POST /api/internal/repo/user2/repo1.git/info/lfs/objects/batch")
		})
		countUpload := slices.ContainsFunc(routerCalls, func(s string) bool {
			return strings.Contains(s, "PUT /api/internal/repo/user2/repo1.git/info/lfs/objects/")
		})
		nonAPIRequests := slices.ContainsFunc(routerCalls, func(s string) bool {
			fields := strings.Fields(s)
			return !strings.HasPrefix(fields[1], "/api/")
		})
		assert.NotZero(t, countBatch)
		assert.NotZero(t, countUpload)
		assert.Zero(t, nonAPIRequests)
	})
}
