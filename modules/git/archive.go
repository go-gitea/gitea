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
	// TODO: use the following steps instead of creating a temp repo, also need to iterate and clean up outdated refs
	// the temp ref has to be under refs/heads/*, and a clone only checks out with a HEAD, which needs the temp repo
	// git update-ref refs/heads/bundle-temp-{timestamp} {commit}
	// git bundle create - refs/heads/bundle-temp-{timestamp}
	// git update-ref -d refs/heads/bundle-temp-{timestamp}
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

	return gitTmpCmd().AddArguments("bundle", "create", "-", "bundle", "HEAD").WithStdoutCopy(out).RunWithStderr(ctx)
}
