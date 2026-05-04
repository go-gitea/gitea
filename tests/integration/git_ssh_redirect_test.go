// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestGitSSHRedirect(t *testing.T) {
	onGiteaRun(t, testGitSSHRedirect)
}

func testGitSSHRedirect(t *testing.T, u *url.URL) {
	apiTestContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)
	session := loginUser(t, "user2")

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

		doAPICreateOrganization(apiTestContext, &structs.CreateOrgOption{
			UserName: "olduser2",
			FullName: "Old User2",
		})(t)

		cloneURL := createSSHUrl("olduser2/repo1.git", u)
		t.Run("Clone Should Fail", doGitCloneFail(cloneURL))

		doAPICreateOrganizationRepository(apiTestContext, "olduser2", &structs.CreateRepoOption{
			Name:     "repo1",
			AutoInit: true,
		})(t)
		testEditFile(t, session, "olduser2", "repo1", "master", "README.md", "This is olduser2's repo1\n")

		dstDir := t.TempDir()
		t.Run("Clone", doGitClone(dstDir, cloneURL))
		readMEContent, err := os.ReadFile(dstDir + "/README.md")
		assert.NoError(t, err)
		assert.Equal(t, "This is olduser2's repo1\n", string(readMEContent))

		apiTestContext2 := NewAPITestContext(t, "user2", "oldrepo1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)
		doAPICreateRepository(apiTestContext2, false)(t)
		testEditFile(t, session, "user2", "oldrepo1", "master", "README.md", "This is user2's oldrepo1\n")

		dstDir = t.TempDir()
		cloneURL = createSSHUrl("user2/oldrepo1.git", u)
		t.Run("Clone", doGitClone(dstDir, cloneURL))
		readMEContent, err = os.ReadFile(dstDir + "/README.md")
		assert.NoError(t, err)
		assert.Equal(t, "This is user2's oldrepo1\n", string(readMEContent))

		cloneURL = createSSHUrl("olduser2/oldrepo1.git", u)
		t.Run("Clone Should Fail", doGitCloneFail(cloneURL))
	})
}
