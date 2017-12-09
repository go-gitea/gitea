// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func onGiteaWebRun(t *testing.T, callback func(*testing.T, *url.URL)) {
	s := http.Server{
		Handler: mac,
	}

	u, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	listener, err := net.Listen("tcp", u.Host)
	assert.NoError(t, err)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)

	callback(t, u)
}

func TestGit(t *testing.T) {
	prepareTestEnv(t)

	onGiteaWebRun(t, func(t *testing.T, u *url.URL) {
		dstPath, err := ioutil.TempDir("", "repo-tmp-17")
		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)
		u.Path = "user2/repo1.git"

		t.Run("Standard", func(t *testing.T) {

			t.Run("CloneNoLogin", func(t *testing.T) {
				dstLocalPath, err := ioutil.TempDir("", "repo1")
				assert.NoError(t, err)
				defer os.RemoveAll(dstLocalPath)
				err = git.Clone(u.String(), dstLocalPath, git.CloneRepoOptions{})
				assert.NoError(t, err)
				assert.True(t, com.IsExist(filepath.Join(dstLocalPath, "README.md")))
			})

			t.Run("CreateRepo", func(t *testing.T) {
				session := loginUser(t, "user2")
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
					AutoInit:    true,
					Description: "Temporary repo",
					Name:        "repo-tmp-17",
					Private:     false,
					Gitignores:  "",
					License:     "WTFPL",
					Readme:      "Default",
				})
				session.MakeRequest(t, req, http.StatusCreated)
			})

			u.Path = "user2/repo-tmp-17.git"
			u.User = url.UserPassword("user2", userPassword)
			t.Run("Clone", func(t *testing.T) {
				err = git.Clone(u.String(), dstPath, git.CloneRepoOptions{})
				assert.NoError(t, err)
				assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
			})

			t.Run("PushCommit", func(t *testing.T) {
				data := make([]byte, 1024)
				_, err := rand.Read(data)
				assert.NoError(t, err)
				tmpFile, err := ioutil.TempFile(dstPath, "data-file-")
				defer tmpFile.Close()
				_, err = tmpFile.Write(data)
				assert.NoError(t, err)

				//Commit
				err = git.AddChanges(dstPath, false, filepath.Base(tmpFile.Name()))
				assert.NoError(t, err)
				err = git.CommitChanges(dstPath, git.CommitChangesOptions{
					Committer: &git.Signature{
						Email: "user2@example.com",
						Name:  "User Two",
						When:  time.Now(),
					},
					Author: &git.Signature{
						Email: "user2@example.com",
						Name:  "User Two",
						When:  time.Now(),
					},
					Message: "Testing commit",
				})
				assert.NoError(t, err)

				//Push
				err = git.Push(dstPath, git.PushOptions{
					Branch: "master",
					Remote: u.String(),
					Force:  false,
				})
				assert.NoError(t, err)
			})
		})
		t.Run("LFS", func(t *testing.T) {
			t.Run("PushCommit", func(t *testing.T) {
				/* Generate random file */
				data := make([]byte, 1024)
				_, err := rand.Read(data)
				assert.NoError(t, err)
				tmpFile, err := ioutil.TempFile(dstPath, "data-file-")
				defer tmpFile.Close()
				_, err = tmpFile.Write(data)
				assert.NoError(t, err)

				//Setup git LFS
				_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("track", "data-file-*").RunInDir(dstPath)
				assert.NoError(t, err)

				//Commit
				err = git.AddChanges(dstPath, false, ".gitattributes", filepath.Base(tmpFile.Name()))
				assert.NoError(t, err)
				err = git.CommitChanges(dstPath, git.CommitChangesOptions{
					Committer: &git.Signature{
						Email: "user2@example.com",
						Name:  "User Two",
						When:  time.Now(),
					},
					Author: &git.Signature{
						Email: "user2@example.com",
						Name:  "User Two",
						When:  time.Now(),
					},
					Message: "Testing LFS ",
				})
				assert.NoError(t, err)

				//Push
				u.User = url.UserPassword("user2", userPassword)
				err = git.Push(dstPath, git.PushOptions{
					Branch: "master",
					Remote: u.String(),
					Force:  false,
				})
				assert.NoError(t, err)
			})
			t.Run("Locks", func(t *testing.T) {
				_, err = git.NewCommand("remote").AddArguments("set-url", "origin", u.String()).RunInDir(dstPath) //TODO add test ssh git-lfs-creds
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("lock", "README.md").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("unlock", "README.md").RunInDir(dstPath)
				assert.NoError(t, err)
			})

		})
	})
}
