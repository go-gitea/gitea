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
	"os/exec"
	"path/filepath"
	"strings"
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
			err := exec.Command("ssh-keygen", "-f", keyFile, "-t", "rsa", "-N", "").Run()
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
			os.Setenv("GIT_SSH_COMMAND",
				"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "+
					filepath.Join(setting.AppWorkPath, keyFile))
			os.Setenv("GIT_SSH_VARIANT", "ssh")

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
					_, err = git.NewCommand("clone").AddArguments(u.String(), dstPath).Run()
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

			t.Run("GitAnnex", func(t *testing.T) {

				err = exec.Command("which", "git-annex").Run()
				if err != nil {
					t.Skip("Git annex not installed")
				}

				assert.True(t, setting.GitAnnex.Enabled)

				os.Setenv("GIT_ANNEX_USE_GIT_SSH", "1")

				u.Path = "user2/repo-tmp-19"
				dstPath, err := ioutil.TempDir("", "repo-tmp-19")
				assert.NoError(t, err)
				defer os.RemoveAll(dstPath)

				var sout string

				createAndClone(t, u, dstPath)

				_, err = git.NewCommand("annex").AddArguments("init", "local").RunInDir(dstPath)
				assert.NoError(t, err)
				filename, err := generateDataFile(littleSize, dstPath)

				t.Run("AddFile", func(t *testing.T) {
					defer exec.Command("chmod", "u+w", dstPath, "-R").Output()

					_, err = git.NewCommand("annex", "add", filename).RunInDir(dstPath)
					assert.NoError(t, err)

					_, err = git.NewCommand("annex", "sync").RunInDir(dstPath)
					assert.NoError(t, err)

					sout, err = git.NewCommand("config", "--get", "remote.origin.annex-uuid").RunInDir(dstPath)
					assert.NoError(t, err)
					assert.Equal(t, len(strings.Trim(sout, "\n ")), 36)

					_, err = git.NewCommand("annex", "copy", "--to", "origin", filename).RunInDir(dstPath)
					assert.NoError(t, err)
					// need to remove so we can clean up - git annex creates directories 0440
					_, err = git.NewCommand("annex", "drop", "--from", "origin", filename).RunInDir(dstPath)
					assert.NoError(t, err)

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

func createAndClone(t *testing.T, u *url.URL, dstPath string) {

	parts := strings.SplitN(u.Path, "/", 2)

	session := loginUser(t, parts[0])
	req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		AutoInit:    true,
		Description: "Temporary repo",
		Name:        parts[1],
		Private:     false,
		Gitignores:  "",
		License:     "WTFPL",
		Readme:      "Default",
	})
	session.MakeRequest(t, req, http.StatusCreated)
	_, err := git.NewCommand("clone", u.String(), dstPath).Run()

	assert.NoError(t, err)
	assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
}

func commitAndPush(t *testing.T, size int, repoPath string) {
	err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two")
	assert.NoError(t, err)
	_, err = git.NewCommand("push").RunInDir(repoPath) //Push
	assert.NoError(t, err)
}

func generateDataFile(size int, repoPath string) (string, error) {

	//Generate random file
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		return "", err
	}
	tmpFile, err := ioutil.TempFile(repoPath, "data-file-")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()
	_, err = tmpFile.Write(data)
	if err != nil {
		return "", err
	}
	return filepath.Base(tmpFile.Name()), nil
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
