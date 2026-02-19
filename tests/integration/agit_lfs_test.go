// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func testAPISaveUserPublicKey(t *testing.T, session *TestSession, username, keyname, content string) {
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/keys", &api.CreateKeyOption{
		Title: keyname,
		Key:   content,
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)
}

func TestAgitLFS(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		// Enable LFS
		defer tests.PrepareTestEnv(t)()
		setting.LFS.StartServer = true
		setting.LFS.Storage.Path = filepath.Join(setting.AppDataPath, "lfs")

		t.Run("HTTP", func(t *testing.T) {
			dstPath := t.TempDir()

			// user4 has read access to repo1 (owned by user2)
			u.Path = "user2/repo1.git"
			u.User = url.UserPassword("user4", userPassword)

			doGitClone(dstPath, u)(t)

			// Setup LFS in the repo
			_, _, err := gitcmd.NewCommand("lfs", "install").WithDir(dstPath).RunStdString(t.Context())
			assert.NoError(t, err)

			_, _, err = gitcmd.NewCommand("lfs", "track", "*.bin").WithDir(dstPath).RunStdString(t.Context())
			assert.NoError(t, err)

			assert.NoError(t, os.WriteFile(filepath.Join(dstPath, "large.bin"), []byte("this is a large file"), 0o644))
			assert.NoError(t, git.AddChanges(t.Context(), dstPath, true))

			signature := git.Signature{
				Email: "user4@example.com",
				Name:  "user4",
			}
			assert.NoError(t, git.CommitChanges(t.Context(), dstPath, git.CommitChangesOptions{
				Committer: &signature,
				Author:    &signature,
				Message:   "Add LFS file",
			}))

			// push to create an agit pull request
			assert.NoError(t, gitcmd.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-lfs-http").
				WithDir(dstPath).
				Run(t.Context()))
		})

		t.Run("SSH", func(t *testing.T) {
			dstPath := t.TempDir()

			// user4 has read access to repo1 (owned by user2)
			repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{OwnerName: "user2", Name: "repo1"})
			sshURL := createSSHUrl(repo.FullName()+".git", u)

			withKeyFile(t, "id_rsa", func(keyFile string) {
				t.Run("AddKey", func(t *testing.T) {
					session := loginUser(t, "user4")
					content, _ := os.ReadFile(keyFile + ".pub")
					testAPISaveUserPublicKey(t, session, "user4", "user4-agit-lfs", string(content))
				})

				doGitClone(dstPath, sshURL)(t)

				// Setup LFS in the repo
				_, _, err := gitcmd.NewCommand("lfs", "install").WithDir(dstPath).RunStdString(t.Context())
				assert.NoError(t, err)

				_, _, err = gitcmd.NewCommand("lfs", "track", "*.bin").WithDir(dstPath).RunStdString(t.Context())
				assert.NoError(t, err)

				assert.NoError(t, os.WriteFile(filepath.Join(dstPath, "large-ssh.bin"), []byte("this is a large file via ssh"), 0o644))
				assert.NoError(t, git.AddChanges(t.Context(), dstPath, true))

				signature := git.Signature{
					Email: "user4@example.com",
					Name:  "user4",
				}
				assert.NoError(t, git.CommitChanges(t.Context(), dstPath, git.CommitChangesOptions{
					Committer: &signature,
					Author:    &signature,
					Message:   "Add LFS file via SSH",
				}))

				// push to create an agit pull request
				assert.NoError(t, gitcmd.NewCommand("push", "origin", "HEAD:refs/for/master/test-agit-lfs-ssh").
					WithDir(dstPath).
					Run(t.Context()))
			})
		})
	})
}
