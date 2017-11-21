// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/sdk/gitea"

	"github.com/stretchr/testify/assert"
)

func TestAPILFSLocksNotStarted(t *testing.T) {
	prepareTestEnv(t)
	setting.LFS.StartServer = false
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "GET", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s/info/lfs/locks/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s/info/lfs/locks/10/unlock", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPILFSLocksNotLogin(t *testing.T) {
	prepareTestEnv(t)
	setting.LFS.StartServer = true
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "GET", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Add("Accept", "application/vnd.git-lfs+json")
	req.Header.Add("Content-Type", "application/vnd.git-lfs+json")
	resp := MakeRequest(t, req, http.StatusForbidden)
	fmt.Println(string(resp.Body))
	var lfsLockError api.LFSLockError
	DecodeJSON(t, resp, &lfsLockError)
	assert.Equal(t, "You must have pull access to list locks : User undefined doesn't have rigth to list for lfs lock [rid: 1]", lfsLockError.Message)
}

func TestAPILFSLocksLogged(t *testing.T) {
	prepareTestEnv(t)
	setting.LFS.StartServer = true
	session := loginUser(t, "user2")
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	req := NewRequestf(t, "GET", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Add("Accept", "application/vnd.git-lfs+json")
	req.Header.Add("Content-Type", "application/vnd.git-lfs+json")
	resp := session.MakeRequest(t, req, http.StatusOK)

	fmt.Println(string(resp.Body))
	var lfsLocks api.LFSLockList
	DecodeJSON(t, resp, &lfsLocks)
	assert.Len(t, lfsLocks.Locks, 0)
	/*
		for _, repo := range apiRepos {
			assert.EqualValues(t, user.ID, repo.Owner.ID)
			assert.False(t, repo.Private)
		}
	*/
}
