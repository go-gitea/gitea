// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
)

func TestGitSSHRedirect(t *testing.T) {
	onGiteaRun(t, testGitSSHRedirect)
}

func testGitSSHRedirect(t *testing.T, u *url.URL) {
	apiTestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	withKeyFile(t, "my-testing-key", func(keyFile string) {
		t.Run("CreateUserKey", doAPICreateUserKey(apiTestContext, "test-key", keyFile))

		testCases := []struct {
			testName string
			userName string
			repoName string
		}{
			{"Test untouched", "user2", "repo1"},
			{"Test renamed user", "olduser2", "repo1"},
			{"Test renamed repo", "user2", "oldrepo1"},
			{"Test renamed user and repo", "olduser2", "oldrepo1"},
		}

		for _, tc := range testCases {
			t.Run(tc.testName, func(t *testing.T) {
				cloneURL := createSSHUrl(fmt.Sprintf("%s/%s.git", tc.userName, tc.repoName), u)
				t.Run("Clone", doGitClone(t.TempDir(), cloneURL))
			})
		}
	})
}
