// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"
)

// CreateArchive create archive content to the target path
func CreateArchive(ctx context.Context, repo RepositoryFacade, repoName, format string, target io.Writer, commitID string, paths []string) error {
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
func CreateBundle(ctx context.Context, repo RepositoryFacade, commit string, out io.Writer) error {
	// TODO: use the following steps instead of creating a temp file, also need to iterate and clean up outdated refs
	// git update-ref refs/bundle/temp-{timestamp} {commit}
	// git bundle create - refs/bundle/temp-{timestamp}
	// git update-ref -d refs/bundle/temp-{timestamp}
	tmpDir, cleanup, err := setting.AppDataTempDir("git-repo-content").MkdirTempRandom("gitea-bundle")
	if err != nil {
		return err
	}
	defer cleanup()

	env := append(os.Environ(), "GIT_OBJECT_DIRECTORY="+filepath.Join(gitrepo.RepoLocalPath(repo), "objects"))
	gitTmpCmd := func() *gitcmd.Command {
		return gitcmd.NewCommand().WithDir(tmpDir).WithEnv(env)
	}

	_, _, err = gitTmpCmd().AddArguments("init", "--bare").RunStdString(ctx)
	if err != nil {
		return err
	}

	_, _, err = gitTmpCmd().AddArguments("reset", "--soft").AddDynamicArguments(commit).RunStdString(ctx)
	if err != nil {
		return err
	}

	_, _, err = gitTmpCmd().AddArguments("branch", "-m", "bundle").RunStdString(ctx)
	if err != nil {
		return err
	}

	tmpFile := filepath.Join(tmpDir, "bundle")
	_, _, err = gitTmpCmd().AddArguments("bundle", "create").AddDynamicArguments(tmpFile, "bundle", "HEAD").RunStdString(ctx)
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
