// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package goproxy

import (
	"archive/zip"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryModule(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	require.NoError(t, err)

	root := t.TempDir()
	oldRepoRootPath, oldAppDataPath := setting.RepoRootPath, setting.AppDataPath
	oldGitHomePath := setting.Git.HomePath
	setting.RepoRootPath = root
	setting.AppDataPath = t.TempDir()
	setting.Git.HomePath = t.TempDir()
	defer func() {
		setting.RepoRootPath, setting.AppDataPath, setting.Git.HomePath = oldRepoRootPath, oldAppDataPath, oldGitHomePath
	}()
	require.NoError(t, git.InitSimple())

	workTree := filepath.Join(root, "worktree")
	require.NoError(t, os.MkdirAll(workTree, os.ModePerm))
	runGit := func(dir string, args ...string) {
		cmd := exec.Command(gitBin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Gitea", "GIT_AUTHOR_EMAIL=gitea@example.com", "GIT_COMMITTER_NAME=Gitea", "GIT_COMMITTER_EMAIL=gitea@example.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	runGit(workTree, "init")
	runGit(workTree, "config", "user.name", "Gitea")
	runGit(workTree, "config", "user.email", "gitea@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(workTree, "go.mod"), []byte("module gitea.example.com/user/repo\n\ngo 1.27\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(workTree, "main.go"), []byte("package main\n"), 0o644))
	runGit(workTree, "add", "go.mod", "main.go")
	runGit(workTree, "commit", "-m", "initial")
	runGit(workTree, "tag", "v1.2.3")

	barePath := filepath.Join(root, "user", "repo.git")
	require.NoError(t, os.MkdirAll(filepath.Dir(barePath), os.ModePerm))
	runGit(root, "clone", "--bare", workTree, barePath)

	repo := &repo_model.Repository{OwnerName: "user", Name: "repo"}
	moduleRepo := &Repository{
		Repo:       repo,
		RepoFacade: repo,
		ModulePath: "gitea.example.com/user/repo",
	}

	versions, err := moduleRepo.ListVersions(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.2.3"}, versions)

	version, err := moduleRepo.ResolveVersion(t.Context(), "v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", version.Version)
	assert.False(t, version.Time.IsZero())

	goMod, err := version.GoMod(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "module gitea.example.com/user/repo\n\ngo 1.27\n", string(goMod))

	var buf bytes.Buffer
	require.NoError(t, version.CreateZip(t.Context(), &buf))

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(zr.File), 2)
	assert.Equal(t, "gitea.example.com/user/repo@v1.2.3/go.mod", zr.File[0].Name)
}
