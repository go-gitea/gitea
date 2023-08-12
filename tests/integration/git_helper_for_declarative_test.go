// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func withKeyFile(t *testing.T, keyname string, callback func(string)) {
	tmpDir := t.TempDir()

	err := os.Chmod(tmpDir, 0o700)
	assert.NoError(t, err)

	keyFile := filepath.Join(tmpDir, keyname)
	err = ssh.GenKeyPair(keyFile)
	assert.NoError(t, err)

	err = os.WriteFile(path.Join(tmpDir, "ssh"), []byte("#!/bin/bash\n"+
		"ssh -o \"UserKnownHostsFile=/dev/null\" -o \"StrictHostKeyChecking=no\" -o \"IdentitiesOnly=yes\" -i \""+keyFile+"\" \"$@\""), 0o700)
	assert.NoError(t, err)

	// Setup ssh wrapper
	os.Setenv("GIT_SSH", path.Join(tmpDir, "ssh"))
	os.Setenv("GIT_SSH_COMMAND",
		"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o IdentitiesOnly=yes -i \""+keyFile+"\"")
	os.Setenv("GIT_SSH_VARIANT", "ssh")

	callback(keyFile)
}

func createSSHUrl(gitPath string, u *url.URL) *url.URL {
	u2 := *u
	u2.Scheme = "ssh"
	u2.User = url.User("git")
	u2.Host = net.JoinHostPort(setting.SSH.ListenHost, strconv.Itoa(setting.SSH.ListenPort))
	u2.Path = gitPath
	return &u2
}

func onGiteaRun[T testing.TB](t T, callback func(T, *url.URL)) {
	defer tests.PrepareTestEnv(t, 1)()
	s := http.Server{
		Handler: testWebRoutes,
	}

	u, err := url.Parse(setting.AppURL)
	assert.NoError(t, err)
	listener, err := net.Listen("tcp", u.Host)
	i := 0
	for err != nil && i <= 10 {
		time.Sleep(100 * time.Millisecond)
		listener, err = net.Listen("tcp", u.Host)
		i++
	}
	assert.NoError(t, err)
	u.Host = listener.Addr().String()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		s.Shutdown(ctx)
		cancel()
	}()

	go s.Serve(listener)
	// Started by config go ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)

	callback(t, u)
}

func doGitClone(dstLocalPath string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		assert.NoError(t, git.CloneWithArgs(context.Background(), git.AllowLFSFiltersArgs(), u.String(), dstLocalPath, git.CloneRepoOptions{}))
		exist, err := util.IsExist(filepath.Join(dstLocalPath, "README.md"))
		assert.NoError(t, err)
		assert.True(t, exist)
	}
}

func doPartialGitClone(dstLocalPath string, u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		assert.NoError(t, git.CloneWithArgs(context.Background(), git.AllowLFSFiltersArgs(), u.String(), dstLocalPath, git.CloneRepoOptions{
			Filter: "blob:none",
		}))
		exist, err := util.IsExist(filepath.Join(dstLocalPath, "README.md"))
		assert.NoError(t, err)
		assert.True(t, exist)
	}
}

func doGitCloneFail(u *url.URL) func(*testing.T) {
	return func(t *testing.T) {
		tmpDir := t.TempDir()
		assert.Error(t, git.Clone(git.DefaultContext, u.String(), tmpDir, git.CloneRepoOptions{}))
		exist, err := util.IsExist(filepath.Join(tmpDir, "README.md"))
		assert.NoError(t, err)
		assert.False(t, exist)
	}
}

func doGitInitTestRepository(dstPath string) func(*testing.T) {
	return func(t *testing.T) {
		// Init repository in dstPath
		assert.NoError(t, git.InitRepository(git.DefaultContext, dstPath, false))
		// forcibly set default branch to master
		_, _, err := git.NewCommand(git.DefaultContext, "symbolic-ref", "HEAD", git.BranchPrefix+"master").RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(dstPath, "README.md"), []byte(fmt.Sprintf("# Testing Repository\n\nOriginally created in: %s", dstPath)), 0o644))
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
		_, _, err := git.NewCommand(git.DefaultContext, "remote", "add").AddDynamicArguments(remoteName, u.String()).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}

func doGitPushTestRepository(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommand(git.DefaultContext, "push", "-u").AddArguments(git.ToTrustedCmdArgs(args)...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}

func doGitPushTestRepositoryFail(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommand(git.DefaultContext, "push").AddArguments(git.ToTrustedCmdArgs(args)...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.Error(t, err)
	}
}

func doGitCreateBranch(dstPath, branch string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommand(git.DefaultContext, "checkout", "-b").AddDynamicArguments(branch).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}

func doGitCheckoutBranch(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommandContextNoGlobals(git.DefaultContext, git.AllowLFSFiltersArgs()...).AddArguments("checkout").AddArguments(git.ToTrustedCmdArgs(args)...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}

func doGitMerge(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommand(git.DefaultContext, "merge").AddArguments(git.ToTrustedCmdArgs(args)...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}

func doGitPull(dstPath string, args ...string) func(*testing.T) {
	return func(t *testing.T) {
		_, _, err := git.NewCommandContextNoGlobals(git.DefaultContext, git.AllowLFSFiltersArgs()...).AddArguments("pull").AddArguments(git.ToTrustedCmdArgs(args)...).RunStdString(&git.RunOpts{Dir: dstPath})
		assert.NoError(t, err)
	}
}
