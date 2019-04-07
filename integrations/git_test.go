// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

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
		PrintCurrentTest(t)
		httpContext := baseAPITestContext
		httpContext.Reponame = "repo-tmp-17"

		dstPath, err := ioutil.TempDir("", httpContext.Reponame)
		var little, big, littleLFS, bigLFS string

		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)
		t.Run("Standard", func(t *testing.T) {
			PrintCurrentTest(t)
			ensureAnonymousClone(t, u)

			t.Run("CreateRepo", doAPICreateRepository(httpContext, false))

			u.Path = httpContext.GitPath()
			u.User = url.UserPassword(username, userPassword)

			t.Run("Clone", doGitClone(dstPath, u))

			t.Run("PushCommit", func(t *testing.T) {
				PrintCurrentTest(t)
				t.Run("Little", func(t *testing.T) {
					PrintCurrentTest(t)
					little = commitAndPush(t, littleSize, dstPath)
				})
				t.Run("Big", func(t *testing.T) {
					PrintCurrentTest(t)
					big = commitAndPush(t, bigSize, dstPath)
				})
			})
		})
		t.Run("LFS", func(t *testing.T) {
			PrintCurrentTest(t)
			t.Run("PushCommit", func(t *testing.T) {
				PrintCurrentTest(t)
				//Setup git LFS
				_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
				assert.NoError(t, err)
				_, err = git.NewCommand("lfs").AddArguments("track", "data-file-*").RunInDir(dstPath)
				assert.NoError(t, err)
				err = git.AddChanges(dstPath, false, ".gitattributes")
				assert.NoError(t, err)

				t.Run("Little", func(t *testing.T) {
					PrintCurrentTest(t)
					littleLFS = commitAndPush(t, littleSize, dstPath)
				})
				t.Run("Big", func(t *testing.T) {
					PrintCurrentTest(t)
					bigLFS = commitAndPush(t, bigSize, dstPath)
				})
			})
			t.Run("Locks", func(t *testing.T) {
				PrintCurrentTest(t)
				lockTest(t, u.String(), dstPath)
			})
		})
		t.Run("Raw", func(t *testing.T) {
			PrintCurrentTest(t)
			session := loginUser(t, "user2")

			// Request raw paths
			req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", little))
			resp := session.MakeRequest(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Body.Len())

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", big))
			nilResp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, nilResp.Length)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", littleLFS))
			resp = session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, littleSize, resp.Body.Len())
			assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/raw/branch/master/", bigLFS))
			resp = session.MakeRequest(t, req, http.StatusOK)
			assert.NotEqual(t, bigSize, resp.Body.Len())
			assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

		})
		t.Run("Media", func(t *testing.T) {
			PrintCurrentTest(t)
			session := loginUser(t, "user2")

			// Request media paths
			req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", little))
			resp := session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", big))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Length)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", littleLFS))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, littleSize, resp.Length)

			req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-17/media/branch/master/", bigLFS))
			resp = session.MakeRequestNilResponseRecorder(t, req, http.StatusOK)
			assert.Equal(t, bigSize, resp.Length)
		})

	})
	t.Run("SSH", func(t *testing.T) {
		PrintCurrentTest(t)
		sshContext := baseAPITestContext
		sshContext.Reponame = "repo-tmp-18"
		keyname := "my-testing-key"
		//Setup key the user ssh key
		withKeyFile(t, keyname, func(keyFile string) {
			t.Run("CreateUserKey", doAPICreateUserKey(sshContext, "test-key", keyFile))
			PrintCurrentTest(t)

			//Setup remote link
			sshURL := createSSHUrl(sshContext.GitPath(), u)

			//Setup clone folder
			dstPath, err := ioutil.TempDir("", sshContext.Reponame)
			assert.NoError(t, err)
			defer os.RemoveAll(dstPath)
			var little, big, littleLFS, bigLFS string

			t.Run("Standard", func(t *testing.T) {
				PrintCurrentTest(t)
				t.Run("CreateRepo", doAPICreateRepository(sshContext, false))

				//TODO get url from api
				t.Run("Clone", doGitClone(dstPath, sshURL))

				//time.Sleep(5 * time.Minute)
				t.Run("PushCommit", func(t *testing.T) {
					PrintCurrentTest(t)
					t.Run("Little", func(t *testing.T) {
						PrintCurrentTest(t)
						little = commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						PrintCurrentTest(t)
						big = commitAndPush(t, bigSize, dstPath)
					})
				})
			})
			t.Run("LFS", func(t *testing.T) {
				PrintCurrentTest(t)
				t.Run("PushCommit", func(t *testing.T) {
					PrintCurrentTest(t)
					//Setup git LFS
					_, err = git.NewCommand("lfs").AddArguments("install").RunInDir(dstPath)
					assert.NoError(t, err)
					_, err = git.NewCommand("lfs").AddArguments("track", "data-file-*").RunInDir(dstPath)
					assert.NoError(t, err)
					err = git.AddChanges(dstPath, false, ".gitattributes")
					assert.NoError(t, err)

					t.Run("Little", func(t *testing.T) {
						PrintCurrentTest(t)
						littleLFS = commitAndPush(t, littleSize, dstPath)
					})
					t.Run("Big", func(t *testing.T) {
						PrintCurrentTest(t)
						bigLFS = commitAndPush(t, bigSize, dstPath)
					})
				})
				t.Run("Locks", func(t *testing.T) {
					PrintCurrentTest(t)
					lockTest(t, u.String(), dstPath)
				})
			})
			t.Run("Raw", func(t *testing.T) {
				PrintCurrentTest(t)
				session := loginUser(t, "user2")

				// Request raw paths
				req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", little))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", big))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", littleLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, littleSize, resp.Body.Len())
				assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/raw/branch/master/", bigLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.NotEqual(t, bigSize, resp.Body.Len())
				assert.Contains(t, resp.Body.String(), models.LFSMetaFileIdentifier)

			})
			t.Run("Media", func(t *testing.T) {
				PrintCurrentTest(t)
				session := loginUser(t, "user2")

				// Request media paths
				req := NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", little))
				resp := session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", big))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", littleLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, littleSize, resp.Body.Len())

				req = NewRequest(t, "GET", path.Join("/user2/repo-tmp-18/media/branch/master/", bigLFS))
				resp = session.MakeRequest(t, req, http.StatusOK)
				assert.Equal(t, bigSize, resp.Body.Len())
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

func commitAndPush(t *testing.T, size int, repoPath string) string {
	name, err := generateCommitWithNewData(size, repoPath, "user2@example.com", "User Two")
	assert.NoError(t, err)
	_, err = git.NewCommand("push").RunInDir(repoPath) //Push
	assert.NoError(t, err)
	return name
}

func generateCommitWithNewData(size int, repoPath, email, fullName string) (string, error) {
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

	//Commit
	err = git.AddChanges(repoPath, false, filepath.Base(tmpFile.Name()))
	if err != nil {
		return "", err
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
	return filepath.Base(tmpFile.Name()), err
}
