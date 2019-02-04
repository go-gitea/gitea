// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"

	"github.com/stretchr/testify/assert"
)

const (
	littleSize = 1024              //1ko
	bigSize    = 128 * 1024 * 1024 //128Mo
)

func TestGit(t *testing.T) {
	onGiteaRun(t, testGit)
}

func testGit(t *testing.T, u *url.URL) {
	username := "user2"
	baseAPITestContext := NewAPITestContext(t, username, "repo1")

	u.Path = baseAPITestContext.GitPath()

	t.Run("HTTP", func(t *testing.T) {
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"

		dstPath, err := ioutil.TempDir("", httpContext.Reponame)
		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)
		t.Run("Standard", func(t *testing.T) {
			ensureAnonymousClone(t, u)

			t.Run("CreateRepo", doAPICreateRepository(httpContext, false))

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(username, userPassword)

			t.Run("Clone", doGitClone(dstPath, u))

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
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		//Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))

			//Setup remote link
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", sshContext.Reponame)
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)

			t.Run("Standard", func(t *testing.T) {
				t.Run("CreateRepo", doAPICreateRepository(sshContext, false))

				//TODO get url from api
				t.Run("Clone", doGitClone(dstPath, sshURL))

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

		})

	})
}

func ensureAnonymousClone(t *testing.T, u *url.URL) {
	dstLocalPath, err := ioutil.TempDir("", "repo1")
	assert.NoError(t, err)
	defer os.RemoveAll(dstLocalPath)
	t.Run("CloneAnonymous", doGitClone(dstLocalPath, u))

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
