// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPILFSLocksNotStarted(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setting.LFS.StartServer = false
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "POST", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks/verify", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
	req = NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks/10/unlock", user.Name, repo.Name)
	MakeRequest(t, req, http.StatusNotFound)
}

func TestAPILFSLocksNotLogin(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setting.LFS.StartServer = true
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	req := NewRequestf(t, "GET", "/%s/%s.git/info/lfs/locks", user.Name, repo.Name)
	req.Header.Set("Accept", lfs.MediaType)
	resp := MakeRequest(t, req, http.StatusUnauthorized)
	var lfsLockError api.LFSLockError
	DecodeJSON(t, resp, &lfsLockError)
	assert.Equal(t, "You must have pull access to list locks", lfsLockError.Message)
}

func TestAPILFSLocksLogged(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	setting.LFS.StartServer = true
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2}) // in org 3
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4}) // in org 3

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	repo3 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 3}) // own by org 3

	tests := []struct {
		user       *user_model.User
		repo       *repo_model.Repository
		path       string
		httpResult int
		addTime    []int
	}{
		{user: user2, repo: repo1, path: "foo/bar.zip", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "path/test", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "path/test", httpResult: http.StatusConflict},
		{user: user2, repo: repo1, path: "Foo/BaR.zip", httpResult: http.StatusConflict},
		{user: user2, repo: repo1, path: "Foo/Test/../subFOlder/../Relative/../BaR.zip", httpResult: http.StatusConflict},
		{user: user4, repo: repo1, path: "FoO/BaR.zip", httpResult: http.StatusUnauthorized},
		{user: user4, repo: repo1, path: "path/test-user4", httpResult: http.StatusUnauthorized},
		{user: user2, repo: repo1, path: "patH/Test-user4", httpResult: http.StatusCreated, addTime: []int{0}},
		{user: user2, repo: repo1, path: "some/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/long/path", httpResult: http.StatusCreated, addTime: []int{0}},

		{user: user2, repo: repo3, path: "test/foo/bar.zip", httpResult: http.StatusCreated, addTime: []int{1, 2}},
		{user: user4, repo: repo3, path: "test/foo/bar.zip", httpResult: http.StatusConflict},
		{user: user4, repo: repo3, path: "test/foo/bar.bin", httpResult: http.StatusCreated, addTime: []int{1, 2}},
	}

	resultsTests := []struct {
		user        *user_model.User
		repo        *repo_model.Repository
		totalCount  int
		oursCount   int
		theirsCount int
		locksOwners []*user_model.User
		locksTimes  []time.Time
	}{
		{user: user2, repo: repo1, totalCount: 4, oursCount: 4, theirsCount: 0, locksOwners: []*user_model.User{user2, user2, user2, user2}, locksTimes: []time.Time{}},
		{user: user2, repo: repo3, totalCount: 2, oursCount: 1, theirsCount: 1, locksOwners: []*user_model.User{user2, user4}, locksTimes: []time.Time{}},
		{user: user4, repo: repo3, totalCount: 2, oursCount: 1, theirsCount: 1, locksOwners: []*user_model.User{user2, user4}, locksTimes: []time.Time{}},
	}

	deleteTests := []struct {
		user   *user_model.User
		repo   *repo_model.Repository
		lockID string
	}{}

	// create locks
	for _, test := range tests {
		session := loginUser(t, test.user.Name)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks", test.repo.FullName()), map[string]string{"path": test.path})
		req.Header.Set("Accept", lfs.AcceptHeader)
		req.Header.Set("Content-Type", lfs.MediaType)
		resp := session.MakeRequest(t, req, test.httpResult)
		if len(test.addTime) > 0 {
			var lfsLock api.LFSLockResponse
			DecodeJSON(t, resp, &lfsLock)
			assert.Equal(t, test.user.Name, lfsLock.Lock.Owner.Name)
			assert.EqualValues(t, lfsLock.Lock.LockedAt.Format(time.RFC3339), lfsLock.Lock.LockedAt.Format(time.RFC3339Nano)) // locked at should be rounded to second
			for _, id := range test.addTime {
				resultsTests[id].locksTimes = append(resultsTests[id].locksTimes, time.Now())
			}
		}
	}

	// check creation
	for _, test := range resultsTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestf(t, "GET", "/%s.git/info/lfs/locks", test.repo.FullName())
		req.Header.Set("Accept", lfs.AcceptHeader)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLocks api.LFSLockList
		DecodeJSON(t, resp, &lfsLocks)
		assert.Len(t, lfsLocks.Locks, test.totalCount)
		for i, lock := range lfsLocks.Locks {
			assert.EqualValues(t, test.locksOwners[i].Name, lock.Owner.Name)
			assert.WithinDuration(t, test.locksTimes[i], lock.LockedAt, 10*time.Second)
			assert.EqualValues(t, lock.LockedAt.Format(time.RFC3339), lock.LockedAt.Format(time.RFC3339Nano)) // locked at should be rounded to second
		}

		req = NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks/verify", test.repo.FullName()), map[string]string{})
		req.Header.Set("Accept", lfs.AcceptHeader)
		req.Header.Set("Content-Type", lfs.MediaType)
		resp = session.MakeRequest(t, req, http.StatusOK)
		var lfsLocksVerify api.LFSLockListVerify
		DecodeJSON(t, resp, &lfsLocksVerify)
		assert.Len(t, lfsLocksVerify.Ours, test.oursCount)
		assert.Len(t, lfsLocksVerify.Theirs, test.theirsCount)
		for _, lock := range lfsLocksVerify.Ours {
			assert.EqualValues(t, test.user.Name, lock.Owner.Name)
			deleteTests = append(deleteTests, struct {
				user   *user_model.User
				repo   *repo_model.Repository
				lockID string
			}{test.user, test.repo, lock.ID})
		}
		for _, lock := range lfsLocksVerify.Theirs {
			assert.NotEqual(t, test.user.DisplayName(), lock.Owner.Name)
		}
	}

	// remove all locks
	for _, test := range deleteTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestWithJSON(t, "POST", fmt.Sprintf("/%s.git/info/lfs/locks/%s/unlock", test.repo.FullName(), test.lockID), map[string]string{})
		req.Header.Set("Accept", lfs.AcceptHeader)
		req.Header.Set("Content-Type", lfs.MediaType)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLockRep api.LFSLockResponse
		DecodeJSON(t, resp, &lfsLockRep)
		assert.Equal(t, test.lockID, lfsLockRep.Lock.ID)
		assert.Equal(t, test.user.Name, lfsLockRep.Lock.Owner.Name)
	}

	// check that we don't have any lock
	for _, test := range resultsTests {
		session := loginUser(t, test.user.Name)
		req := NewRequestf(t, "GET", "/%s.git/info/lfs/locks", test.repo.FullName())
		req.Header.Set("Accept", lfs.AcceptHeader)
		resp := session.MakeRequest(t, req, http.StatusOK)
		var lfsLocks api.LFSLockList
		DecodeJSON(t, resp, &lfsLocks)
		assert.Len(t, lfsLocks.Locks, 0)
	}
}
