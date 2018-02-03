// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

const (
	littleSize = 1024              //1ko
	bigSize    = 128 * 1024 * 1024 //128Mo
)

func onGiteaRun(t *testing.T, callback func(*testing.T, *url.URL)) {
	prepareTestEnv(t)
	s := http.Server{
		Handler: mac,
	}

	u, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	listener, err := net.Listen("tcp", u.Host)
	assert.NoError(t, err)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)
	//Started by config go ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)

	callback(t, u)
}

func TestGit(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		u.Path = "user2/repo1.git"

		t.Run("HTTP", func(t *testing.T) {
			dstPath, err := ioutil.TempDir("", "repo-tmp-17")
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)
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
					t.Run("Little", func(t *testing.T) {
						commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						commitAndPush(t, bigSize, dstPath)
					})
				})
			})
			t.Run("LFS", func(t *testing.T) {
				t.Run("PushCommit", func(t *testing.T) {
					//Setup git LFS
					_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
					assert.NoError(t, err)
					_, err = git.NewCommand("lfs").AddArguments("track", "data-file-*").RunInDir(dstPath)
					assert.NoError(t, err)
					err = git.AddChanges(dstPath, false, ".gitattributes")
					assert.NoError(t, err)

					t.Run("Little", func(t *testing.T) {
						commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						commitAndPush(t, bigSize, dstPath)
					})
				})
				t.Run("Locks", func(t *testing.T) {
					lockTest(t, u.String(), dstPath)
				})
			})
		})
		t.Run("SSH", func(t *testing.T) {
			//Setup remote link
			u.Scheme = "ssh"
			u.User = url.User("git")
			u.Host = fmt.Sprintf("%s:%d", setting.SSH.ListenHost, setting.SSH.ListenPort)
			u.Path = "user2/repo-tmp-18.git"

			//Setup key
			keyFile := filepath.Join(setting.AppDataPath, "my-testing-key")
			_, _, err := com.ExecCmd("ssh-keygen", "-f", keyFile, "-t", "rsa", "-N", "")
			assert.NoError(t, err)
			defer os.RemoveAll(keyFile)
			defer os.RemoveAll(keyFile + ".pub")

			session := loginUser(t, "user1")
			keyOwner := models.AssertExistsAndLoadBean(t, &models.User{Name: "user2"}).(*models.User)
			urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys", keyOwner.Name)

			dataPubKey, err := ioutil.ReadFile(keyFile + ".pub")
			assert.NoError(t, err)
			req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
				"key":   string(dataPubKey),
				"title": "test-key",
			})
			session.MakeRequest(t, req, http.StatusCreated)

			//Setup ssh wrapper
			sshWrapper, err := ioutil.TempFile(setting.AppDataPath, "tmp-ssh-wrapper")
			sshWrapper.WriteString("#!/bin/sh\n\n")
			sshWrapper.WriteString("ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i \"" + filepath.Join(setting.AppWorkPath, keyFile) + "\" $* \n\n")
			err = sshWrapper.Chmod(os.ModePerm)
			assert.NoError(t, err)
			sshWrapper.Close()
			defer os.RemoveAll(sshWrapper.Name())

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", "repo-tmp-18")
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)

			t.Run("Standard", func(t *testing.T) {
				t.Run("CreateRepo", func(t *testing.T) {
					session := loginUser(t, "user2")
					req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
						AutoInit:    true,
						Description: "Temporary repo",
						Name:        "repo-tmp-18",
						Private:     false,
						Gitignores:  "",
						License:     "WTFPL",
						Readme:      "Default",
					})
					session.MakeRequest(t, req, http.StatusCreated)
				})
				//TODO get url from api
				t.Run("Clone", func(t *testing.T) {
					_, err = git.NewCommand("clone").AddArguments("--config", "core.sshCommand="+filepath.Join(setting.AppWorkPath, sshWrapper.Name()), u.String(), dstPath).Run()
					assert.NoError(t, err)
					assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
				})
				//time.Sleep(5 * time.Minute)
				t.Run("PushCommit", func(t *testing.T) {
					t.Run("Little", func(t *testing.T) {
						commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						commitAndPush(t, bigSize, dstPath)
					})
				})
			})
			t.Run("LFS", func(t *testing.T) {
				os.Setenv("GIT_SSH_COMMAND", filepath.Join(setting.AppWorkPath, sshWrapper.Name())) //TODO remove when fixed https://github.com/git-lfs/git-lfs/issues/2215
				defer os.Unsetenv("GIT_SSH_COMMAND")
				t.Run("PushCommit", func(t *testing.T) {
					//Setup git LFS
					_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
					assert.NoError(t, err)
					_, err = git.NewCommand("lfs").AddArguments("track", "data-file-*").RunInDir(dstPath)
					assert.NoError(t, err)
					err = git.AddChanges(dstPath, false, ".gitattributes")
					assert.NoError(t, err)

					t.Run("Little", func(t *testing.T) {
						commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						commitAndPush(t, bigSize, dstPath)
					})
				})
				t.Run("Locks", func(t *testing.T) {
					lockTest(t, u.String(), dstPath)
				})
			})
		})
	})
}

func lockTest(t *testing.T, remote, repoPath string) {
	_, err := git.NewCommand("remote").AddArguments("set-url", "origin", remote).RunInDir(repoPath) //TODO add test ssh git-lfs-creds
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("lock", "README.md").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("locks").RunInDir(repoPath)
	assert.NoError(t, err)
	_, err = git.NewCommand("lfs").AddArguments("unlock", "README.md").RunInDir(repoPath)
	assert.NoError(t, err)
}

func commitAndPush(t *testing.T, size int, repoPath string) {
	err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two")
	assert.NoError(t, err)
	_, err = git.NewCommand("push").RunInDir(repoPath) //Push
	assert.NoError(t, err)
}

func generateCommitWithNewData(size int, repoPath, email, fullName string) error {
	//Generate random file
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return err
	}
	tmpFile, err := ioutil.TempFile(repoPath, "data-file-")
	if err != nil {
		return err
	}
	defer tmpFile.Close()
	_, err = tmpFile.Write(data)
	if err != nil {
		return err
	}

	//Commit
	err = git.AddChanges(repoPath, false, filepath.Base(tmpFile.Name()))
	if err != nil {
		return err
	}
	err = git.CommitChanges(repoPath, git.CommitChangesOptions{
		Committer: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Author: &git.Signature{
			Email: email,
			Name:  fullName,
			When:  time.Now(),
		},
		Message: fmt.Sprintf("Testing commit @ %v", time.Now()),
	})
	return err
}
