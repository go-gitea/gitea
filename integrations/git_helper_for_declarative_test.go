// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
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
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"github.com/Unknwon/com"
	"github.com/stretchr/testify/assert"
)

func withKeyFile(t *testing.T, keyname string, callback func(string)) {
	keyFile := filepath.Join(setting.AppDataPath, keyname)
	err := ssh.GenKeyPair(keyFile)
	assert.NoError(t, err)

	//Setup ssh wrapper
	os.Setenv("GIT_SSH_COMMAND",
		"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "+
			filepath.Join(setting.AppWorkPath, keyFile))
	os.Setenv("GIT_SSH_VARIANT", "ssh")

	callback(keyFile)

	defer os.RemoveAll(keyFile)
	defer os.RemoveAll(keyFile + ".pub")
}

func createSSHUrl(gitPath string, u *url.URL) *url.URL {
	u2 := *u
	u2.Scheme = "ssh"
	u2.User = url.User("git")
	u2.Host = fmt.Sprintf("%s:%d", setting.SSH.ListenHost, setting.SSH.ListenPort)
	u2.Path = gitPath
	return &u2
}

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

func doGitClone(dstLocalPath string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		assert.NoError(t, git.Clone(u.String(), dstLocalPath, git.CloneRepoOptions{}))
		assert.True(t, com.IsExist(filepath.Join(dstLocalPath, "README.md")))
	}
}

func doGitCloneFail(dstLocalPath string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		assert.Error(t, git.Clone(u.String(), dstLocalPath, git.CloneRepoOptions{}))
		assert.False(t, com.IsExist(filepath.Join(dstLocalPath, "README.md")))
	}
}

func doGitInitTestRepository(dstPath string) func(*testing.T) {
	return func(t *testing.T) {
		// Init repository in dstPath
		assert.NoError(t, git.InitRepository(dstPath, false))
		assert.NoError(t, ioutil.WriteFile(filepath.Join(dstPath, "README.md"), []byte(fmt.Sprintf("# Testing Repository\n\nOriginally created in: %s", dstPath)), 0644))
		assert.NoError(t, git.AddChanges(dstPath, true))
		signature := git.Signature{
			Email: "test@example.com",
			Name:  "test",
			When:  time.Now(),
		}
		assert.NoError(t, git.CommitChanges(dstPath, git.CommitChangesOptions{
			Committer: &signature,
			Author:    &signature,
			Message:   "Initial Commit",
		}))
	}
}

func doGitAddRemote(dstPath, remoteName string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand("remote", "add", remoteName, u.String()).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitPushTestRepository(dstPath, remoteName, branch string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand("push", "-u", remoteName, branch).RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func doGitPushTestRepositoryFail(dstPath, remoteName, branch string) func(*testing.T) {
	return func(t *testing.T) {
		_, err := git.NewCommand("push", "-u", remoteName, branch).RunInDir(dstPath)
		assert.Error(t, err)
	}
}
