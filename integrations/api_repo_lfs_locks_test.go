// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

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
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	resp := MakeRequest(t, req, http.StatusForbidden)
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
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	resp := session.MakeRequest(t, req, http.StatusOK)
	var lfsLocks api.LFSLockList
	DecodeJSON(t, resp, &lfsLocks)
	assert.Len(t, lfsLocks.Locks, 0)

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{\"path\": \"foo/bar.zip\"}"))
	resp = session.MakeRequest(t, req, http.StatusCreated)

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{\"path\": \"path/test\"}"))
	resp = session.MakeRequest(t, req, http.StatusCreated)

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{\"path\": \"path/test\"}"))
	resp = session.MakeRequest(t, req, http.StatusConflict)

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{\"path\": \"Foo/BaR.zip\"}"))
	resp = session.MakeRequest(t, req, http.StatusConflict)

	req = NewRequestf(t, "GET", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &lfsLocks)
	assert.Len(t, lfsLocks.Locks, 2)
	for _, lock := range lfsLocks.Locks {
		assert.EqualValues(t, user.DisplayName(), lock.Owner.Name)
	}

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks/verify", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{}"))
	resp = session.MakeRequest(t, req, http.StatusOK)
	var lfsLocksVerify api.LFSLockListVerify
	DecodeJSON(t, resp, &lfsLocksVerify)
	assert.Len(t, lfsLocksVerify.Ours, 2)
	assert.Len(t, lfsLocksVerify.Theirs, 0)
	for _, lock := range lfsLocksVerify.Ours {
		assert.EqualValues(t, user.DisplayName(), lock.Owner.Name)
		assert.WithinDuration(t, time.Now(), lock.LockedAt, 10*time.Second)
	}

	req = NewRequestf(t, "POST", "/%s/%s/info/lfs/locks/%s/unlock", user.Name, repo.Name, lfsLocksVerify.Ours[0].ID)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Body = ioutil.NopCloser(strings.NewReader("{}"))
	resp = session.MakeRequest(t, req, http.StatusOK)
	var lfsLockRep api.LFSLockResponse
	DecodeJSON(t, resp, &lfsLockRep)
	assert.Equal(t, lfsLocksVerify.Ours[0].ID, lfsLockRep.Lock.ID)
	assert.Equal(t, user.DisplayName(), lfsLockRep.Lock.Owner.Name)

	req = NewRequestf(t, "GET", "/%s/%s/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", "application/vnd.git-lfs+json")
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &lfsLocks)
	assert.Len(t, lfsLocks.Locks, 1)

}
