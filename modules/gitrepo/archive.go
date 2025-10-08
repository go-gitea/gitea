// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git/gitcmd"
	"code.gitea.io/gitea/modules/setting"
)

// CreateArchive create archive content to the target path
func CreateArchive(ctx context.Context, repo Repository, format string, target io.Writer, usePrefix bool, commitID string) error {
	if format == "unknown" {
		return fmt.Errorf("unknown format: %v", format)
	}

	cmd := gitcmd.NewCommand("archive")
	if usePrefix {
		cmd.AddOptionFormat("--prefix=%s", filepath.Base(strings.TrimSuffix(repo.RelativePath(), ".git"))+"/")
	}
	cmd.AddOptionFormat("--format=%s", format)
	cmd.AddDynamicArguments(commitID)

	var stderr strings.Builder
	if err := RunCmd(ctx, repo, cmd.WithStdout(target).WithStderr(&stderr)); err != nil {
		return gitcmd.ConcatenateError(err, stderr.String())
	}
	return nil
}

// CreateBundle create bundle content to the target path
func CreateBundle(ctx context.Context, repo Repository, commit string, out io.Writer) error {
	tmp, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("gitea-bundle")
	if err != nil {
		return err
	}
	defer cleanup()

	env := append(os.Environ(), "GIT_OBJECT_DIRECTORY="+filepath.Join(repoPath(repo), "objects"))
	_, _, err = gitcmd.NewCommand("init", "--bare").WithDir(tmp).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return err
	}

	_, _, err = gitcmd.NewCommand("reset", "--soft").AddDynamicArguments(commit).WithDir(tmp).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return err
	}

	_, _, err = gitcmd.NewCommand("branch", "-m", "bundle").WithDir(tmp).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return err
	}

	tmpFile := filepath.Join(tmp, "bundle")
	_, _, err = gitcmd.NewCommand("bundle", "create").AddDynamicArguments(tmpFile, "bundle", "HEAD").WithDir(tmp).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return err
	}

	fi, err := os.Open(tmpFile)
	if err != nil {
		return err
	}
	defer fi.Close()

	_, err = io.Copy(out, fi)
	return err
}
