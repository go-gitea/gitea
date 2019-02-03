// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/git"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
)

func createEmptyRepository(username, reponame string) func(*testing.T) {
	return func(t *testing.T) {
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)
		req := NewRequestWithJSON(t, "POST", "/api/v1/user/repos?token="+token, &api.CreateRepoOption{
			AutoInit:    false,
			Description: "Temporary empty repo",
			Name:        reponame,
			Private:     false,
		})
		session.MakeRequest(t, req, http.StatusCreated)
	}
}

func createDeployKey(username, reponame, keyname, keyFile string, readOnly bool) func(*testing.T) {
	return func(t *testing.T) {
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/keys?token=%s", username, reponame, token)

		dataPubKey, err := ioutil.ReadFile(keyFile + ".pub")
		assert.NoError(t, err)
		req := NewRequestWithJSON(t, "POST", urlStr, api.CreateKeyOption{
			Title:    keyname,
			Key:      string(dataPubKey),
			ReadOnly: readOnly,
		})
		session.MakeRequest(t, req, http.StatusCreated)
	}
}

func initTestRepository(dstPath string) func(*testing.T) {
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

func pushTestRepository(dstPath, username, reponame string, u url.URL, keyFile string) func(*testing.T) {
	return func(t *testing.T) {
		//Setup remote link
		u.Scheme = "ssh"
		u.User = url.User("git")
		u.Host = fmt.Sprintf("%s:%d", setting.SSH.ListenHost, setting.SSH.ListenPort)

		//Setup ssh wrapper
		os.Setenv("GIT_SSH_COMMAND",
			"ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "+
				filepath.Join(setting.AppWorkPath, keyFile))
		os.Setenv("GIT_SSH_VARIANT", "ssh")

		log.Printf("Adding remote: %s\n", u.String())
		_, err := git.NewCommand("remote", "add", "origin", u.String()).RunInDir(dstPath)
		assert.NoError(t, err)

		log.Printf("Pushing to: %s\n", u.String())
		_, err = git.NewCommand("push", "-u", "origin", "master").RunInDir(dstPath)
		assert.NoError(t, err)
	}
}

func checkRepositoryEmptyStatus(username, reponame string, isEmpty bool) func(*testing.T) {
	return func(t *testing.T) {
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", username, reponame, token)

		req := NewRequest(t, "GET", urlStr)
		resp := session.MakeRequest(t, req, http.StatusOK)

		var repository api.Repository
		DecodeJSON(t, resp, &repository)

		assert.Equal(t, isEmpty, repository.Empty)
	}
}

func deleteRepository(username, reponame string) func(*testing.T) {
	return func(t *testing.T) {
		session := loginUser(t, username)
		token := getTokenForLoggedInUser(t, session)
		urlStr := fmt.Sprintf("/api/v1/repos/%s/%s?token=%s", username, reponame, token)

		req := NewRequest(t, "DELETE", urlStr)
		session.MakeRequest(t, req, http.StatusNoContent)
	}
}

func TestPushDeployKeyOnEmptyRepo(t *testing.T) {
	onGiteaRun(t, testPushDeployKeyOnEmptyRepo)
}

func testPushDeployKeyOnEmptyRepo(t *testing.T, u *url.URL) {
	reponame := "deploy-key-empty-repo-1"
	username := "user2"
	u.Path = fmt.Sprintf("%s/%s.git", username, reponame)
	keyname := fmt.Sprintf("%s-push", reponame)

	t.Run("CreateEmptyRepository", createEmptyRepository(username, reponame))
	t.Run("CheckIsEmpty", checkRepositoryEmptyStatus(username, reponame, true))

	//Setup the push deploy key file
	keyFile := filepath.Join(setting.AppDataPath, keyname)
	err := exec.Command("ssh-keygen", "-f", keyFile, "-t", "rsa", "-N", "").Run()
	assert.NoError(t, err)
	defer os.RemoveAll(keyFile)
	defer os.RemoveAll(keyFile + ".pub")

	t.Run("CreatePushDeployKey", createDeployKey(username, reponame, keyname, keyFile, false))

	// Setup the testing repository
	dstPath, err := ioutil.TempDir("", "repo-tmp-deploy-key-empty-repo-1")
	assert.NoError(t, err)
	defer os.RemoveAll(dstPath)

	t.Run("InitTestRepository", initTestRepository(dstPath))
	t.Run("SSHPushTestRepository", pushTestRepository(dstPath, username, reponame, *u, keyFile))

	log.Println("Done Push")
	t.Run("CheckIsNotEmpty", checkRepositoryEmptyStatus(username, reponame, false))

	t.Run("DeleteRepository", deleteRepository(username, reponame))
}
