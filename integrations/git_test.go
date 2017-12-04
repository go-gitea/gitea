// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"fmt"
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

	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func onGiteaWebRun(t *testing.T, callback func(*testing.T, string)) {
	s := http.Server{
		Handler: mac,
	}

	listener, err := net.Listen("tcp", "")
	assert.NoError(t, err)

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	assert.NoError(t, err)

	callback(t, fmt.Sprintf("http://localhost:%s/", port))
}

func TestGit(t *testing.T) {
	prepareTestEnv(t)

	onGiteaWebRun(t, func(t *testing.T, urlPrefix string) {
		dstPath, err := ioutil.TempDir("", "repo1")
		assert.NoError(t, err)
		defer os.RemoveAll(dstPath)

		t.Run("CloneNoLogin", func(t *testing.T) {
			err = git.Clone(fmt.Sprintf("%suser2/repo1.git", urlPrefix),
				dstPath, git.CloneRepoOptions{})
			assert.NoError(t, err)
			assert.True(t, com.IsExist(filepath.Join(dstPath, "README.md")))
		})

		t.Run("LFS", func(t *testing.T) {
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
			err = git.AddChanges(dstPath, false, ".gitattributes", tmpFile.Name())
			assert.NoError(t, err)
			err = git.CommitChanges(dstPath, git.CommitChangesOptions{
				Committer: &git.Signature{
					Email: "test@email.com",
					Name:  "User2",
					When:  time.Now(),
				},
				Author: &git.Signature{
					Email: "test@email.com",
					Name:  "User2",
					When:  time.Now(),
				},
				Message: "Testing LFS ",
			})
			assert.NoError(t, err)

			//Push
			u, err := url.Parse(fmt.Sprintf("%suser2/repo1.git", urlPrefix))
			u.User = url.UserPassword("user2", "password")
			assert.NoError(t, err)
			err = git.Push(dstPath, git.PushOptions{
				Branch: "master",
				Remote: u.String(),
				Force:  false,
			})
			assert.NoError(t, err)
		})
	})
}
