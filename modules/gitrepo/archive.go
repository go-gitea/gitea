// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
)

// CreateArchive create archive content to the target path
func CreateArchive(ctx context.Context, repo git.RepositoryFacade, repoName, format string, target io.Writer, commitID string, paths []string) error {
	if format == "unknown" {
		return fmt.Errorf("unknown format: %v", format)
	}

	cmd := gitcmd.NewCommand("archive")
	if setting.Repository.PrefixArchiveFiles {
		cmd.AddOptionFormat("--prefix=%s", strings.ToLower(repoName)+"/")
	}
	cmd.AddOptionFormat("--format=%s", format)
	cmd.AddDynamicArguments(commitID)

	for i := range paths {
		// although "git archive" already ensures the paths won't go outside the repo, we still clean them here for safety
		cmd.AddDynamicArguments(path.Clean(paths[i]))
	}
	return cmd.WithStdoutCopy(target).WithRepo(repo).RunWithStderr(ctx)
}

// CreateBundle create bundle content to the target path
func CreateBundle(ctx context.Context, repo git.RepositoryFacade, commit string, out io.Writer) error {
	// TODO: use the following steps instead of creating a temp file, also need to iterate and clean up outdated refs
	// git update-ref refs/bundle/temp-{timestamp} {commit}
	// git bundle create - refs/bundle/temp-{timestamp}
	// git update-ref -d refs/bundle/temp-{timestamp}
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
