// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/queue"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/services/quota"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageQuotaAPI(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	admin := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteAdmin)
	user := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	path := "/api/v1/admin/quotas/repository/1"
	session := loginUser(t, "user2")
	settings := session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/settings"), http.StatusOK)
	assert.NotContains(t, settings.Body.String(), ">Storage quota</h4>")
	MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(user), http.StatusForbidden)
	MakeRequest(t, NewRequestWithJSON(t, "PUT", path, api.StorageQuotaOptions{GitLimit: new(int64(0)), LFSLimit: new(int64(-1))}).AddTokenAuth(admin), http.StatusOK)
	resp := MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(admin), http.StatusOK)
	status := DecodeJSON(t, resp, api.StorageQuota{})
	require.NotNil(t, status.Git)
	assert.EqualValues(t, 0, status.Git.Limit)
	assert.EqualValues(t, -1, status.LFS.Limit)
	assert.True(t, status.Git.Warning)
	settings = session.MakeRequest(t, NewRequest(t, "GET", "/user2/repo1/settings"), http.StatusOK)
	assert.Contains(t, settings.Body.String(), ">Storage quota</h4>")
	MakeRequest(t, NewRequestWithJSON(t, "PUT", path, api.StorageQuotaOptions{GitLimit: new(int64(-2))}).AddTokenAuth(admin), http.StatusBadRequest)
	MakeRequest(t, NewRequestWithJSON(t, "PUT", path, api.StorageQuotaOptions{TotalLimit: new(int64(10))}).AddTokenAuth(admin), http.StatusBadRequest)
	MakeRequest(t, NewRequestWithJSON(t, "PUT", path, api.StorageQuotaOptions{}).AddTokenAuth(admin), http.StatusOK)
	resp = MakeRequest(t, NewRequest(t, "GET", path).AddTokenAuth(admin), http.StatusOK)
	status = DecodeJSON(t, resp, api.StorageQuota{})
	assert.Nil(t, status.Overrides.GitLimit)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/admin/quotas/organization/2").AddTokenAuth(admin), http.StatusNotFound)
	MakeRequest(t, NewRequest(t, "GET", "/api/v1/admin/quotas/repository/999999").AddTokenAuth(admin), http.StatusNotFound)
}

func TestStorageQuotaHTTPPush(t *testing.T) {
	onGiteaRun(t, testStorageQuotaPush)
}

func TestStorageQuotaSSHPush(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		withKeyFile(t, "quota-test-key", func(keyFile string) {
			apiContext := NewAPITestContext(t, "user2", "repo1", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteRepository)
			doAPICreateUserKey(apiContext, "quota-test-key", keyFile)(t)
			testStorageQuotaPush(t, createSSHUrl("user2/repo1.git", u))
		})
	})
}

func testStorageQuotaPush(t *testing.T, serverURL *url.URL) {
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	u := *serverURL
	if u.Scheme != "ssh" {
		u.User = url.UserPassword("user2", userPassword)
	}
	u.Path = repo.FullName() + ".git"
	local := filepath.Join(t.TempDir(), "repo")
	require.NoError(t, git.Clone(t.Context(), u.String(), local, git.CloneRepoOptions{}))
	require.NoError(t, quota.UpdatePolicy(t.Context(), quota_model.Repository, repo.ID, api.StorageQuotaOptions{GitLimit: new(int64(0))}))
	require.NoError(t, gitcmd.NewCommand("push", "origin", "master:refs/heads/quota-ref-only").WithDir(local).Run(t.Context()))
	doGitCheckoutWriteFileCommit(localGitAddCommitOptions{LocalRepoPath: local, CheckoutBranch: "master", TreeFilePath: "quota.txt", TreeFileContent: "new content"})(t)
	before, err := git.GetFullCommitID(t.Context(), repo, "master")
	require.NoError(t, err)
	_, stderr, err := gitcmd.NewCommand("push", "origin", "master").WithDir(local).RunStdString(t.Context())
	require.Error(t, err)
	assert.Contains(t, stderr, "quota")
	after, err := git.GetFullCommitID(t.Context(), repo, "master")
	require.NoError(t, err)
	assert.Equal(t, before, after)
	if u.Scheme != "ssh" {
		var commit string
		var err error
		commit, _, err = gitcmd.NewCommand("rev-parse", "HEAD").WithDir(local).RunStdString(t.Context())
		require.NoError(t, err)
		commit = strings.TrimSpace(commit)
		pack, _, err := gitcmd.NewCommand("pack-objects", "--stdout", "--revs").WithDir(local).
			WithStdinBytes([]byte(commit + "\n^" + before + "\n")).RunStdBytes(t.Context())
		require.NoError(t, err)
		line := fmt.Sprintf("%s %s refs/heads/quota-gzip\x00report-status\n", strings.Repeat("0", len(before)), commit)
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, err = fmt.Fprintf(writer, "%04x%s0000", len(line)+4, line)
		require.NoError(t, err)
		_, err = writer.Write(pack)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		endpoint := u
		endpoint.Path += "/git-receive-pack"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, endpoint.String(), bytes.NewReader(compressed.Bytes()))
		require.NoError(t, err)
		req.ContentLength = -1
		req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
		req.Header.Set("Content-Encoding", "gzip")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "quota")
		assert.False(t, git.IsBranchExist(t.Context(), repo, "quota-gzip"))
	}
	require.NoError(t, quota.UpdatePolicy(t.Context(), quota_model.Repository, repo.ID, api.StorageQuotaOptions{GitLimit: new(int64(-1))}))
	require.NoError(t, gitcmd.NewCommand("push", "origin", "master").WithDir(local).Run(t.Context()))
}

func TestStorageQuotaLFSUpload(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	defer test.MockVariableValue(&setting.LFS.StartServer, true)()
	repo := createLFSTestRepository(t, "quota-lfs")
	session := loginUser(t, "user2")
	require.NoError(t, quota.UpdatePolicy(t.Context(), quota_model.Repository, repo.ID, api.StorageQuotaOptions{LFSLimit: new(int64(3))}))
	put := func(data []byte, code int) {
		t.Helper()
		path := fmt.Sprintf("/%s.git/info/lfs/objects/%x/%d", repo.FullName(), sha256.Sum256(data), len(data))
		session.MakeRequest(t, NewRequestWithBody(t, "PUT", path, bytes.NewReader(data)), code)
	}
	put([]byte("one"), http.StatusOK)
	put([]byte("one"), http.StatusOK)
	put([]byte("two"), http.StatusUnprocessableEntity)
	status, err := quota.Status(t.Context(), quota_model.Repository, repo.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 3, status.LFS.Used)
}

func TestStorageQuotaForkAndTransfer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	dest := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	repo, err := repo_service.CreateRepository(t.Context(), owner, owner, repo_service.CreateRepoOptions{Name: "quota-source", AutoInit: true, Readme: "Default"})
	require.NoError(t, err)
	defer test.MockVariableValue(&setting.Quota.DefaultRepoGitLimit, int64(0))()
	_, err = repo_service.ForkRepository(t.Context(), dest, dest, repo_service.ForkRepoOptions{Name: "quota-fork", BaseRepo: repo})
	require.True(t, quota.IsQuotaError(err), "%v", err)
	_, err = repo_model.GetRepositoryByOwnerAndName(t.Context(), dest.Name, "quota-fork")
	require.Error(t, err)
	require.NoError(t, quota.UpdatePolicy(t.Context(), quota_model.User, dest.ID, api.StorageQuotaOptions{TotalLimit: new(int64(0))}))
	err = repo_service.StartRepositoryTransfer(t.Context(), admin, dest, repo, nil)
	require.True(t, quota.IsQuotaError(err), "%v", err)
	unchanged := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	assert.Equal(t, owner.ID, unchanged.OwnerID)
	require.NoError(t, quota.UpdatePolicy(t.Context(), quota_model.User, dest.ID, api.StorageQuotaOptions{TotalLimit: new(int64(-1))}))
	require.NoError(t, repo_service.StartRepositoryTransfer(t.Context(), admin, dest, repo, nil))
	transferred := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: repo.ID})
	assert.Equal(t, dest.ID, transferred.OwnerID)
}

func TestStorageQuotaLFSStorage(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	backend, err := storage.NewStorage(setting.LFS.Storage.Type, setting.LFS.Storage)
	require.NoError(t, err)
	t.Logf("LFS backend: %s", setting.LFS.Storage.Type)
	store := &lfs.ContentStore{ObjectStorage: backend}
	data := []byte(rand.Text())
	p := lfs.Pointer{Oid: fmt.Sprintf("%x", sha256.Sum256(data)), Size: int64(len(data))}
	t.Cleanup(func() { require.NoError(t, backend.Delete(p.RelativePath())) })
	require.NoError(t, store.Put(p, bytes.NewReader(data)))
	require.ErrorIs(t, store.Put(p, bytes.NewReader(bytes.Repeat([]byte("x"), len(data)))), lfs.ErrHashMismatch)
	object, err := store.Get(p)
	require.NoError(t, err)
	defer object.Close()
	actual, err := io.ReadAll(object)
	require.NoError(t, err)
	assert.Equal(t, data, actual)
}

func BenchmarkStorageQuotaPackedPush(b *testing.B) {
	onGiteaRun(b, func(b *testing.B, serverURL *url.URL) {
		repo := unittest.AssertExistsAndLoadBean(b, &repo_model.Repository{ID: 1})
		u := *serverURL
		u.User, u.Path = url.UserPassword("user2", userPassword), repo.FullName()+".git"
		local := filepath.Join(b.TempDir(), "repo")
		require.NoError(b, git.Clone(b.Context(), u.String(), local, git.CloneRepoOptions{}))
		const historyCommits = 10_000
		var history bytes.Buffer
		for i := range historyCommits {
			content := fmt.Sprintf("initial %d", i)
			fmt.Fprintf(&history, "commit refs/heads/quota-bench\ncommitter Benchmark <bench@example.com> 1234567890 +0000\ndata %d\n%s\nM 100644 inline file\ndata %d\n%s\n\n", len(content), content, len(content), content)
		}
		require.NoError(b, gitcmd.NewCommand("fast-import", "--quiet").WithDir(local).WithStdinBytes(history.Bytes()).Run(b.Context()))
		require.NoError(b, git.FetchRemoteCommitUpdateRef(b.Context(), repo, gitrepo.RepositoryUnmanaged(local), "refs/heads/quota-bench", "refs/heads/quota-bench"))
		require.NoError(b, queue.GetManager().FlushAll(b.Context(), -1))
		usage, err := quota.Measure(b.Context(), repo)
		require.NoError(b, err)
		require.NoError(b, quota.UpdatePolicy(b.Context(), quota_model.Repository, repo.ID, api.StorageQuotaOptions{GitLimit: new(usage.GitBytes + 64*1024*1024)}))
		i := 0
		for b.Loop() {
			b.StopTimer()
			content := fmt.Sprintf("push %d", i)
			input := fmt.Sprintf("commit refs/heads/quota-bench\ncommitter Benchmark <bench@example.com> 1234567891 +0000\ndata %d\n%s\nfrom refs/heads/quota-bench^0\nM 100644 inline file\ndata %d\n%s\n\n", len(content), content, len(content), content)
			require.NoError(b, gitcmd.NewCommand("fast-import", "--quiet").WithDir(local).WithStdinBytes([]byte(input)).Run(b.Context()))
			b.StartTimer()
			require.NoError(b, gitcmd.NewCommand("push", "origin", "quota-bench").WithDir(local).RunWithStderr(b.Context()))
			i++
		}
		b.ReportMetric(historyCommits, "history_commits")
	})
}
