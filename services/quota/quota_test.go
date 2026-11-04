// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package quota

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	quota_model "gitea.dev/models/quota"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { unittest.MainTest(m) }

func quotaRepository(t *testing.T) (*repo_model.Repository, context.Context) {
	t.Helper()
	require.NoError(t, unittest.PrepareTestDatabase())
	t.Cleanup(test.MockVariableValue(&setting.RepoRootPath, t.TempDir()))
	_, err := db.GetEngine(t.Context()).Where("id > ?", 0).Delete(new(git_model.LFSMetaObject))
	require.NoError(t, err)
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	require.NoError(t, repo.LoadOwner(t.Context()))
	return repo, t.Context()
}

func TestQuotaOperation(t *testing.T) {
	repo, ctx := quotaRepository(t)
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, GitLimit: new(int64(15))}))
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: Subject(repo.Owner), SubjectID: repo.OwnerID, TotalLimit: new(int64(20))}))
	objects := filepath.Join(gitrepo.RepoLocalPath(repo), "objects")
	require.NoError(t, os.MkdirAll(filepath.Join(objects, "aa"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(objects, "aa", "object"), make([]byte, 10), 0o644))
	other := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	otherObjects := filepath.Join(gitrepo.RepoLocalPath(other), "objects", "bb")
	require.NoError(t, os.MkdirAll(otherObjects, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(otherObjects, "object"), make([]byte, 2), 0o644))
	ctx, op, err := Begin(ctx, repo)
	require.NoError(t, err)
	require.NotNil(t, op)
	defer op.Finish()
	assert.EqualValues(t, 5, op.GitReserved)
	assert.EqualValues(t, 12, op.AccountBefore)
	require.NoError(t, op.Check(ctx, 5, 0))
	require.Error(t, op.Check(ctx, 6, 0))
	_, _, err = Begin(t.Context(), repo)
	assert.ErrorIs(t, err, quota_model.ErrBusy)
	_, _, err = Begin(t.Context(), other)
	assert.ErrorIs(t, err, quota_model.ErrBusy)
	_, nested, err := Begin(ctx, repo)
	require.NoError(t, err)
	nested.Finish()
	_, err = quota_model.GetOperation(ctx, repo.OwnerID, op.Token)
	require.NoError(t, err)
	q := filepath.Join(objects, "tmp_objdir-incoming-test")
	require.NoError(t, os.MkdirAll(q, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(q, "object"), make([]byte, 5), 0o644))
	require.NoError(t, CheckReceive(ctx, repo, op.Token, q))
	require.NoError(t, os.WriteFile(filepath.Join(q, "object"), make([]byte, 6), 0o644))
	require.Error(t, CheckReceive(ctx, repo, op.Token, q))
	require.Error(t, CheckReceive(ctx, repo, op.Token, t.TempDir()))
	require.Error(t, CheckReceive(ctx, repo, "wrong-token", q))
	pointer := lfs.Pointer{Oid: fmt.Sprintf("%064x", 1), Size: 6}
	_, err = AssociateLFS(ctx, repo.ID, pointer)
	require.NoError(t, err)
	_, err = AssociateLFS(ctx, repo.ID, pointer)
	require.NoError(t, err)
	assert.EqualValues(t, 2, op.GitReserved)
	require.Error(t, op.Check(ctx, 5, 0))
	_, err = AssociateLFS(ctx, repo.ID, lfs.Pointer{Oid: fmt.Sprintf("%064x", 2), Size: 5})
	require.Error(t, err)
}

func importCommit(t *testing.T, repo git.RepositoryFacade, data []byte) {
	t.Helper()
	var input bytes.Buffer
	fmt.Fprintf(&input, "blob\nmark :1\ndata %d\n", len(data))
	input.Write(data)
	input.WriteString("\ncommit refs/heads/main\ncommitter Test <test@example.com> 1234567890 +0000\ndata 1\nx\nM 100644 :1 file\n\n")
	require.NoError(t, gitcmd.NewCommand("fast-import", "--quiet", "--force").WithRepo(repo).WithStdinBytes(input.Bytes()).RunWithStderr(t.Context()))
}

func TestQuotaImportedLFS(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    int64
		limit   int64
		wantErr error
	}{
		{"understated", 1, 1, lfs.ErrSizeMismatch},
		{"overstated", 5, 5, lfs.ErrSizeMismatch},
		{"over-quota", 4, 1, util.ErrUnprocessableContent},
		{"matching", 4, 4, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx := quotaRepository(t)
			require.NoError(t, git.InitRepository(ctx, repo, "sha1", "main"))
			backend, err := storage.NewLocalStorage(ctx, &setting.Storage{Path: t.TempDir()})
			require.NoError(t, err)
			defer test.MockVariableValue(&storage.LFS, backend)()
			data := []byte("data")
			pointer, err := lfs.GeneratePointer(bytes.NewReader(data))
			require.NoError(t, err)
			require.NoError(t, lfs.NewContentStore().Put(pointer, bytes.NewReader(data)))
			pointer.Size = tc.size
			importCommit(t, repo, []byte(pointer.StringContent()))
			require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, LFSLimit: &tc.limit}))
			gitRepo, err := git.OpenRepository(ctx, repo)
			require.NoError(t, err)
			defer gitRepo.Close()
			err = StoreMissingLfsObjectsInRepository(ctx, repo, gitRepo, nil)
			require.ErrorIs(t, err, tc.wantErr)
			meta, err := git_model.GetLFSMetaObjectByOid(ctx, repo.ID, pointer.Oid)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, git_model.ErrLFSObjectNotExist)
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, len(data), meta.Size)
			}
		})
	}
}

func TestQuotaStagedCloneAndFetch(t *testing.T) {
	repo, ctx := quotaRepository(t)
	source := gitrepo.RepositoryUnmanaged(filepath.Join(t.TempDir(), "source.git"))
	require.NoError(t, git.InitRepositoryLocal(ctx, gitrepo.RepoLocalPath(source), true, "sha1", "main"))
	importCommit(t, source, []byte("initial"))
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, GitLimit: new(int64(0))}))
	err := git.CloneExternalRepo(ctx, gitrepo.RepoLocalPath(source), repo, git.CloneRepoOptions{Bare: true})
	require.Error(t, err)
	_, err = os.Stat(gitrepo.RepoLocalPath(repo))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, GitLimit: new(int64(64 * 1024))}))
	require.NoError(t, git.CloneExternalRepo(ctx, gitrepo.RepoLocalPath(source), repo, git.CloneRepoOptions{Bare: true}))
	before, err := refSnapshot(ctx, repo)
	require.NoError(t, err)
	usage, err := Measure(ctx, repo)
	require.NoError(t, err)
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, GitLimit: &usage.GitBytes}))
	data := make([]byte, 16*1024)
	_, err = rand.Read(data)
	require.NoError(t, err)
	importCommit(t, source, data)
	runFetch := func(ctx context.Context, target git.RepositoryFacade, env []string) error {
		return gitcmd.NewCommand("fetch", "--no-auto-maintenance").AddDashesAndList(gitrepo.RepoLocalPath(source), "+refs/heads/main:refs/heads/main").WithRepo(target).WithEnv(env).Run(ctx)
	}
	require.Error(t, Fetch(ctx, repo, runFetch))
	after, err := refSnapshot(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	afterUsage, err := Measure(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, usage.GitBytes, afterUsage.GitBytes)
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, GitLimit: new(int64(128 * 1024))}))
	require.NoError(t, Fetch(ctx, repo, runFetch))
	after, err = refSnapshot(ctx, repo)
	require.NoError(t, err)
	assert.NotEqual(t, before, after)
	staging, err := filepath.Glob(filepath.Join(filepath.Dir(gitrepo.RepoLocalPath(repo)), ".gitea-quota-*"))
	require.NoError(t, err)
	assert.Empty(t, staging)
}

func TestQuotaInterruptedOperation(t *testing.T) {
	repo, ctx := quotaRepository(t)
	require.NoError(t, git.InitRepository(ctx, repo, "sha1", "main"))
	defer test.MockVariableValue(&setting.Quota.DefaultRepoGitLimit, int64(1024))()
	staleCtx, op, err := Begin(ctx, repo)
	require.NoError(t, err)
	require.NotNil(t, op)
	staging := filepath.Join(filepath.Dir(gitrepo.RepoLocalPath(repo)), ".gitea-quota-"+op.Token+"-abandoned")
	require.NoError(t, os.MkdirAll(staging, 0o755))
	q := filepath.Join(gitrepo.RepoLocalPath(repo), "objects", "tmp_objdir-incoming-abandoned")
	require.NoError(t, os.MkdirAll(q, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(q, "partial"), []byte("partial upload"), 0o644))
	require.NoError(t, quota_model.AttachChild(ctx, op.Operation))
	require.NoError(t, quota_model.MarkRecovery(ctx, op.Operation))
	_, err = quota_model.GetOperation(ctx, repo.OwnerID, op.Token)
	assert.ErrorIs(t, err, quota_model.ErrUncertain)
	_, _, err = Begin(ctx, repo)
	assert.ErrorIs(t, err, quota_model.ErrBusy)
	require.NoError(t, RecoverOffline(ctx))
	_, err = os.Stat(staging)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(q)
	assert.True(t, os.IsNotExist(err))
	_, resumed, err := Begin(staleCtx, repo)
	require.NoError(t, err)
	defer resumed.Finish()
	assert.NotEqual(t, op.Token, resumed.Token)
}

func TestQuotaConcurrentLFS(t *testing.T) {
	repo, ctx := quotaRepository(t)
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: quota_model.Repository, SubjectID: repo.ID, LFSLimit: new(int64(10))}))
	ctx, op, err := Begin(ctx, repo)
	require.NoError(t, err)
	defer op.Finish()
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := range 2 {
		wg.Go(func() {
			_, err := AssociateLFS(ctx, repo.ID, lfs.Pointer{Oid: fmt.Sprintf("%064x", i+1), Size: 6})
			results <- err
		})
	}
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else {
			var exceeded quota_model.ExceededError
			assert.ErrorAs(t, err, &exceeded)
		}
	}
	assert.Equal(t, 1, accepted)
	size, err := git_model.GetRepoLFSSize(ctx, repo.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 6, size)
}

func TestQuotaStaleLFSUsage(t *testing.T) {
	repo, ctx := quotaRepository(t)
	other := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.Equal(t, repo.OwnerID, other.OwnerID)
	require.NoError(t, quota_model.SetPolicy(ctx, &quota_model.Policy{Subject: Subject(repo.Owner), SubjectID: repo.OwnerID, TotalLimit: new(int64(1024))}))
	p := lfs.Pointer{Oid: fmt.Sprintf("%064x", 123), Size: 1024}
	_, err := git_model.NewLFSMetaObject(ctx, repo.ID, p)
	require.NoError(t, err)
	_, err = git_model.NewLFSMetaObject(ctx, repo.ID, lfs.Pointer{Oid: fmt.Sprintf("%064x", 124), Size: 256})
	require.NoError(t, err)
	_, err = git_model.NewLFSMetaObject(ctx, other.ID, lfs.Pointer{Oid: fmt.Sprintf("%064x", 125), Size: 128})
	require.NoError(t, err)
	_, err = Reconcile(ctx, other)
	require.NoError(t, err)
	_, err = Reconcile(ctx, repo)
	require.NoError(t, err)
	allowed, err := quota_model.RequestGC(ctx, repo.ID)
	require.NoError(t, err)
	require.True(t, allowed)
	_, err = git_model.RemoveLFSMetaObjectByOid(ctx, repo.ID, p.Oid)
	require.NoError(t, err)
	status, err := Status(ctx, quota_model.Repository, repo.ID)
	require.NoError(t, err)
	require.EqualValues(t, 256, status.LFS.Used)
	require.EqualValues(t, 384, status.Account.Used)
	_, op, err := Begin(ctx, other)
	require.NoError(t, err)
	defer op.Finish()
	require.EqualValues(t, 384, op.AccountBefore)
	require.NoError(t, op.Check(ctx, 0, 640))
	require.Error(t, op.Check(ctx, 0, 641))
	usage, found, err := quota_model.GetUsage(ctx, repo.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.NotZero(t, usage.Updated)
	allowed, err = quota_model.RequestGC(ctx, repo.ID)
	require.NoError(t, err)
	require.False(t, allowed)
}
