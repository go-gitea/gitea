// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func doCheckRepositoryEmptyStatus(ctx APITestContext, isEmpty bool) func(*testing.T) {
	return doAPIGetRepository(ctx, func(t *testing.T, repository api.Repository) {
		assert.Equal(t, isEmpty, repository.Empty)
	})
}

func doAddChangesToCheckout(dstPath, filename string) func(*testing.T) {
	return func(t *testing.T) {
		assert.NoError(t, os.WriteFile(filepath.Join(dstPath, filename), fmt.Appendf(nil, "# Testing Repository\n\nOriginally created in: %s at time: %v", dstPath, time.Now()), 0o644))
		assert.NoError(t, gitAddChangesDeprecated(t.Context(), dstPath, true))
		signature := git.Signature{
			Email: "test@example.com",
			Name:  "test",
			When:  time.Now(),
		}
		assert.NoError(t, gitCommitChangesDeprecated(t.Context(), dstPath, gitCommitChangesOptions{
			Committer: &signature,
			Author:    &signature,
			Message:   "Initial Commit",
		}))
	}
}

// TestSSHShellWelcome covers the "shell" request, which carries no command payload unlike "exec"
func TestSSHShellWelcome(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		ctx := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteUser)
		withKeyFile(t, "welcome-key", func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(ctx, "welcome-key", keyFile))

			privateKey, err := os.ReadFile(keyFile)
			require.NoError(t, err)
			signer, err := gossh.ParsePrivateKey(privateKey)
			require.NoError(t, err)

			client, err := gossh.Dial("tcp", net.JoinHostPort(setting.SSH.ListenHost, strconv.Itoa(setting.SSH.ListenPort)), &gossh.ClientConfig{
				User:            setting.SSH.BuiltinServerUser,
				Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
				HostKeyCallback: gossh.InsecureIgnoreHostKey(),
			})
			require.NoError(t, err)
			defer client.Close()

			session, err := client.NewSession()
			require.NoError(t, err)

			var stderr bytes.Buffer
			session.Stderr = &stderr // "gitea serv" writes the welcome with println, which goes to stderr
			require.NoError(t, session.Shell())
			require.NoError(t, session.Wait()) // fails unless the server reports exit status 0
			assert.Contains(t, stderr.String(), "You've successfully authenticated with the key named welcome-key")
		})
	})
}

func TestPushDeployKeyOnEmptyRepo(t *testing.T) {
	onGiteaRun(t, testPushDeployKeyOnEmptyRepo)
}

func testPushDeployKeyOnEmptyRepo(t *testing.T, u *url.URL) {
	// OK login
	ctx := NewAPITestContext(t, "user2", "deploy-key-empty-repo-1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	ctxWithDeleteRepo := NewAPITestContext(t, "user2", "deploy-key-empty-repo-1", auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	keyname := ctx.Reponame + "-push"
	u.Path = ctx.GitPath()

	t.Run("CreateEmptyRepository", doAPICreateRepository(ctx, true))

	t.Run("CheckIsEmpty", doCheckRepositoryEmptyStatus(ctx, true))

	withKeyFile(t, keyname, func(keyFile string) {
		t.Run("CreatePushDeployKey", doAPICreateDeployKey(ctx, keyname, keyFile, false))

		// Setup the testing repository
		dstPath := t.TempDir()

		t.Run("InitTestRepository", doGitInitTestRepository(dstPath))

		// Setup remote link
		sshURL := createSSHUrl(ctx.GitPath(), u)

		t.Run("AddRemote", doGitAddRemote(dstPath, "origin", sshURL))

		t.Run("SSHPushTestRepository", doGitPushTestRepository(dstPath, "origin", "master"))

		t.Run("CheckIsNotEmpty", doCheckRepositoryEmptyStatus(ctx, false))

		t.Run("DeleteRepository", doAPIDeleteRepository(ctxWithDeleteRepo))
	})
}

func TestKeyOnlyOneType(t *testing.T) {
	onGiteaRun(t, testKeyOnlyOneType)
}

func testKeyOnlyOneType(t *testing.T, u *url.URL) {
	// Once a key is a user key we cannot use it as a deploy key
	// If we delete it from the user we should be able to use it as a deploy key
	reponame := "ssh-key-test-repo"
	username := "user2"
	u.Path = fmt.Sprintf("%s/%s.git", username, reponame)
	keyname := reponame + "-push"

	// OK login
	ctx := NewAPITestContext(t, username, reponame, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)
	ctxWithDeleteRepo := NewAPITestContext(t, username, reponame, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	otherCtx := ctx
	otherCtx.Reponame = "ssh-key-test-repo-2"
	otherCtxWithDeleteRepo := ctxWithDeleteRepo
	otherCtxWithDeleteRepo.Reponame = otherCtx.Reponame

	failCtx := ctx
	failCtx.ExpectedCode = http.StatusUnprocessableEntity

	t.Run("CreateRepository", doAPICreateRepository(ctx, false))
	t.Run("CreateOtherRepository", doAPICreateRepository(otherCtx, false))

	withKeyFile(t, keyname, func(keyFile string) {
		var userKeyPublicKeyID int64
		t.Run("KeyCanOnlyBeUser", func(t *testing.T) {
			dstPath := t.TempDir()

			sshURL := createSSHUrl(ctx.GitPath(), u)

			t.Run("FailToClone", doGitCloneFail(sshURL))

			t.Run("CreateUserKey", doAPICreateUserKey(ctx, keyname, keyFile, func(t *testing.T, publicKey api.PublicKey) {
				userKeyPublicKeyID = publicKey.ID
			}))

			t.Run("FailToAddReadOnlyDeployKey", doAPICreateDeployKey(failCtx, keyname, keyFile, true))

			t.Run("FailToAddDeployKey", doAPICreateDeployKey(failCtx, keyname, keyFile, false))

			t.Run("Clone", doGitClone(dstPath, sshURL))

			t.Run("AddChanges", doAddChangesToCheckout(dstPath, "CHANGES1.md"))

			t.Run("Push", doGitPushTestRepository(dstPath, "origin", "master"))

			t.Run("DeleteUserKey", doAPIDeleteUserKey(ctx, userKeyPublicKeyID))
		})

		t.Run("KeyCanBeAnyDeployButNotUserAswell", func(t *testing.T) {
			dstPath := t.TempDir()

			sshURL := createSSHUrl(ctx.GitPath(), u)

			t.Run("FailToClone", doGitCloneFail(sshURL))

			// Should now be able to add...
			t.Run("AddReadOnlyDeployKey", doAPICreateDeployKey(ctx, keyname, keyFile, true))

			t.Run("Clone", doGitClone(dstPath, sshURL))

			t.Run("AddChanges", doAddChangesToCheckout(dstPath, "CHANGES2.md"))

			t.Run("FailToPush", doGitPushTestRepositoryFail(dstPath, "origin", "master"))

			otherSSHURL := createSSHUrl(otherCtx.GitPath(), u)
			dstOtherPath := t.TempDir()

			t.Run("AddWriterDeployKeyToOther", doAPICreateDeployKey(otherCtx, keyname, keyFile, false))

			t.Run("CloneOther", doGitClone(dstOtherPath, otherSSHURL))

			t.Run("AddChangesToOther", doAddChangesToCheckout(dstOtherPath, "CHANGES3.md"))

			t.Run("PushToOther", doGitPushTestRepository(dstOtherPath, "origin", "master"))

			t.Run("FailToCreateUserKey", doAPICreateUserKey(failCtx, keyname, keyFile))
		})

		t.Run("DeleteRepositoryShouldReleaseKey", func(t *testing.T) {
			otherSSHURL := createSSHUrl(otherCtx.GitPath(), u)
			dstOtherPath := t.TempDir()

			t.Run("DeleteRepository", doAPIDeleteRepository(ctxWithDeleteRepo))

			t.Run("FailToCreateUserKeyAsStillDeploy", doAPICreateUserKey(failCtx, keyname, keyFile))

			t.Run("MakeSureCloneOtherStillWorks", doGitClone(dstOtherPath, otherSSHURL))

			t.Run("AddChangesToOther", doAddChangesToCheckout(dstOtherPath, "CHANGES3.md"))

			t.Run("PushToOther", doGitPushTestRepository(dstOtherPath, "origin", "master"))

			t.Run("DeleteOtherRepository", doAPIDeleteRepository(otherCtxWithDeleteRepo))

			t.Run("RecreateRepository", doAPICreateRepository(ctxWithDeleteRepo, false))

			t.Run("CreateUserKey", doAPICreateUserKey(ctx, keyname, keyFile, func(t *testing.T, publicKey api.PublicKey) {
				userKeyPublicKeyID = publicKey.ID
			}))

			dstPath := t.TempDir()

			sshURL := createSSHUrl(ctx.GitPath(), u)

			t.Run("Clone", doGitClone(dstPath, sshURL))

			t.Run("AddChanges", doAddChangesToCheckout(dstPath, "CHANGES1.md"))

			t.Run("Push", doGitPushTestRepository(dstPath, "origin", "master"))
		})

		t.Run("DeleteUserKeyShouldRemoveAbilityToClone", func(t *testing.T) {
			sshURL := createSSHUrl(ctx.GitPath(), u)

			t.Run("DeleteUserKey", doAPIDeleteUserKey(ctx, userKeyPublicKeyID))

			t.Run("FailToClone", doGitCloneFail(sshURL))
		})
	})
}
