// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRunObjectMetrics(m *testing.M) error {
	gitHomePath, err := os.MkdirTemp(os.TempDir(), "git-home")
	if err != nil {
		return fmt.Errorf("unable to create temp dir: %w", err)
	}
	defer os.RemoveAll(gitHomePath)
	setting.Git.HomePath = gitHomePath

	if err = git.InitFull(); err != nil {
		return fmt.Errorf("failed to call git.InitFull: %w", err)
	}

	exitCode := m.Run()
	if exitCode != 0 {
		return fmt.Errorf("test run failed, ExitCode=%d", exitCode)
	}
	return nil
}

func TestMain(m *testing.M) {
	if err := testRunObjectMetrics(m); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Test failed: %v\n", err)
		os.Exit(1)
	}
}

func TestCollectRepoObjectCountsDisabledWhenMetricsOff(t *testing.T) {
	cfg := objectMetricsConfig{Enabled: false, EnabledObjectCount: true}

	before := testutil.CollectAndCount(repoObjectCount)
	collectRepoObjectCountsWith(context.Background(), cfg)
	after := testutil.CollectAndCount(repoObjectCount)

	assert.Equal(t, before, after, "gauge series count should not change when Metrics.Enabled is false")
}

func TestCollectRepoObjectCountsDisabledWhenObjectCountOff(t *testing.T) {
	cfg := objectMetricsConfig{Enabled: true, EnabledObjectCount: false}

	before := testutil.CollectAndCount(repoObjectCount)
	collectRepoObjectCountsWith(context.Background(), cfg)
	after := testutil.CollectAndCount(repoObjectCount)

	assert.Equal(t, before, after, "gauge series count should not change when EnabledObjectCount is false")
}

// makeTestBareRepo creates a bare git repo with one commit so there is at
// least one blob, one tree, and one commit object to count.
func makeTestBareRepo(t *testing.T) string {
	t.Helper()

	workDir := t.TempDir()
	bareDir := filepath.Join(t.TempDir(), "repo.git")

	mustGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}

	mustGit(workDir, "init", "-b", "main")
	mustGit(workDir, "config", "user.email", "test@example.com")
	mustGit(workDir, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "file.txt"), []byte("hello\n"), 0o644))
	mustGit(workDir, "add", "file.txt")
	mustGit(workDir, "commit", "-m", "initial commit")
	mustGit(workDir, "clone", "--bare", workDir, bareDir)

	return bareDir
}

func TestCountObjects_EmptyRepo(t *testing.T) {
	bareDir := t.TempDir()
	mustGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = bareDir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	mustGit("init", "--bare", "-b", "main", bareDir)

	blobs, commits, trees := countObjects(context.Background(), gitrepo.RepositoryUnmanaged(bareDir))
	assert.Equal(t, 0, blobs)
	assert.Equal(t, 0, commits)
	assert.Equal(t, 0, trees)
}

func TestCountObjects_WithObjects(t *testing.T) {
	bareDir := makeTestBareRepo(t)

	blobs, commits, trees := countObjects(context.Background(), gitrepo.RepositoryUnmanaged(bareDir))
	assert.GreaterOrEqual(t, blobs, 1, "expected at least one blob")
	assert.GreaterOrEqual(t, commits, 1, "expected at least one commit")
	assert.GreaterOrEqual(t, trees, 1, "expected at least one tree")
}

func TestCountObjects_InvalidRepo(t *testing.T) {
	blobs, commits, trees := countObjects(context.Background(), gitrepo.RepositoryUnmanaged(filepath.Join(t.TempDir(), "nonexistent")))
	assert.Equal(t, 0, blobs)
	assert.Equal(t, 0, commits)
	assert.Equal(t, 0, trees)
}
