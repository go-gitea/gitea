// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/lfs"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLFSClient is a controllable lfs.Client for exercising the mirror LFS
// download paths without talking to a real upstream.
type mockLFSClient struct {
	batchSize int
	// download is invoked for each batch; it decides what the callback sees.
	download func(ctx context.Context, objects []lfs.Pointer, cb lfs.DownloadCallback) error
}

func (c *mockLFSClient) BatchSize() int {
	if c.batchSize == 0 {
		return 20
	}
	return c.batchSize
}

func (c *mockLFSClient) Download(ctx context.Context, objects []lfs.Pointer, cb lfs.DownloadCallback) error {
	return c.download(ctx, objects, cb)
}

func (c *mockLFSClient) Upload(ctx context.Context, objects []lfs.Pointer, cb lfs.UploadCallback) error {
	return nil
}

// feedPointers returns a channel pre-loaded with the given pointers and closed,
// mimicking the output of SearchPointerBlobs(InRange).
func feedPointers(pointers ...lfs.Pointer) <-chan lfs.PointerBlob {
	ch := make(chan lfs.PointerBlob, len(pointers))
	for _, p := range pointers {
		ch <- lfs.PointerBlob{Hash: p.Oid, Pointer: p}
	}
	close(ch)
	return ch
}

func newTestPointer(t *testing.T, content string) lfs.Pointer {
	t.Helper()
	p, err := lfs.GeneratePointer(bytes.NewReader([]byte(content)))
	require.NoError(t, err)
	return p
}

func newTempContentStore(t *testing.T) *lfs.ContentStore {
	t.Helper()
	localStore, err := storage.NewLocalStorage(t.Context(), &setting.Storage{Path: t.TempDir()})
	require.NoError(t, err)
	return &lfs.ContentStore{ObjectStorage: localStore}
}

// Test_processLFSPointerChan_DownloadErrorPropagates ensures a genuine download
// failure is returned (not swallowed), so callers such as runSync will not
// advance the mirror's LFSLastRefs watermark past unfetched objects.
func Test_processLFSPointerChan_DownloadErrorPropagates(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pointer := newTestPointer(t, "download-error")

	wantErr := errors.New("upstream boom")
	client := &mockLFSClient{
		download: func(ctx context.Context, objects []lfs.Pointer, cb lfs.DownloadCallback) error {
			// Surface a hard per-object error through the callback, mirroring
			// how performSingleOperation propagates a batch object error.
			for _, o := range objects {
				if err := cb(o, nil, wantErr); err != nil {
					return err
				}
			}
			return nil
		},
	}

	err := processLFSPointerChan(t.Context(), repo, newTempContentStore(t), client, feedPointers(pointer))
	assert.ErrorIs(t, err, wantErr)
}

// Test_processLFSPointerChan_CancelledContextPropagates ensures a cancelled sync
// returns a non-nil error rather than reporting success, which would otherwise
// let the caller advance LFSLastRefs past an incompletely-scanned range.
func Test_processLFSPointerChan_CancelledContextPropagates(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	pointer := newTestPointer(t, "cancelled")

	ctx, cancel := context.WithCancel(t.Context())

	client := &mockLFSClient{
		download: func(dlCtx context.Context, objects []lfs.Pointer, cb lfs.DownloadCallback) error {
			// Simulate the real client observing the parent cancellation: the
			// errgroup returns the context error.
			cancel()
			return dlCtx.Err()
		},
	}

	// AddLFSMirrorPending records the pointer first; the download then fails due
	// to cancellation. We must still see a non-nil error out of the function.
	err := processLFSPointerChan(ctx, repo, newTempContentStore(t), client, feedPointers(pointer))
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// Test_processLFSPointerChan_HappyPath ensures a successful download promotes the
// pointer to lfs_meta_object and clears the pending row.
func Test_processLFSPointerChan_HappyPath(t *testing.T) {
	unittest.PrepareTestEnv(t)

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	content := "happy-path-content"
	pointer := newTestPointer(t, content)

	client := &mockLFSClient{
		download: func(ctx context.Context, objects []lfs.Pointer, cb lfs.DownloadCallback) error {
			for _, o := range objects {
				rc := io.NopCloser(bytes.NewReader([]byte(content)))
				if err := cb(o, rc, nil); err != nil {
					return err
				}
			}
			return nil
		},
	}

	err := processLFSPointerChan(t.Context(), repo, newTempContentStore(t), client, feedPointers(pointer))
	require.NoError(t, err)

	// Promoted to a real meta object...
	_, err = git_model.GetLFSMetaObjectByOid(t.Context(), repo.ID, pointer.Oid)
	assert.NoError(t, err)

	// ...and no longer pending.
	pending, err := git_model.GetLFSMirrorPendingByRepoID(t.Context(), repo.ID)
	assert.NoError(t, err)
	for _, p := range pending {
		assert.NotEqual(t, pointer.Oid, p.Oid, "pointer should have been removed from pending")
	}
}
