// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLockRepo struct{ path string }

func (r testLockRepo) GitRepoManagedID() string { return "test-lock-repo" }
func (r testLockRepo) GitRepoLocation() string  { return r.path }
func (r testLockRepo) LogString() string        { return r.path }

func TestRemoveStaleLockFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel string, age time.Duration) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
		mt := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(p, mt, mt))
		return p
	}
	exists := func(p string) bool { _, err := os.Stat(p); return err == nil }

	staleRef := write("refs/heads/main.lock", 2*time.Hour)
	staleNestedRef := write("refs/heads/feature/a.lock", 48*time.Hour)
	staleConfig := write("config.lock", 2*time.Hour)
	stalePacked := write("packed-refs.lock", 2*time.Hour)
	staleCommitGraph := write("objects/info/commit-graph.lock", 2*time.Hour)
	freshRef := write("refs/heads/fresh.lock", time.Minute) // maybe held by a running git process
	notLock := write("refs/heads/main", 48*time.Hour)       // a normal ref
	notLock2 := write("refs/heads/lock", 48*time.Hour)      // a branch named "lock"
	looseObject := write("objects/ab/cdef.lock", 48*time.Hour)
	hook := write("hooks/pre-receive.lock", 48*time.Hour)

	n, err := RemoveStaleLockFiles(testLockRepo{root}, StaleLockFileMinAge)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	for _, p := range []string{staleRef, staleNestedRef, staleConfig, stalePacked, staleCommitGraph} {
		assert.False(t, exists(p), p)
	}
	for _, p := range []string{freshRef, notLock, notLock2, looseObject, hook} {
		assert.True(t, exists(p), p)
	}

	// idempotent
	n, err = RemoveStaleLockFiles(testLockRepo{root}, StaleLockFileMinAge)
	require.NoError(t, err)
	assert.Zero(t, n)

	// a missing repository is not an error
	n, err = RemoveStaleLockFiles(testLockRepo{filepath.Join(root, "missing")}, StaleLockFileMinAge)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestRemoveStaleLockFilesSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.lock")
	require.NoError(t, os.WriteFile(outside, []byte("x"), 0o644))
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(outside, old, old))
	if err := os.Symlink(outside, filepath.Join(root, "link.lock")); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	n, err := RemoveStaleLockFiles(testLockRepo{root}, StaleLockFileMinAge)
	require.NoError(t, err)
	assert.Zero(t, n) // symlinks are never followed or removed
	_, err = os.Stat(outside)
	assert.NoError(t, err)
}
