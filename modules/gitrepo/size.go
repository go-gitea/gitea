// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/git/gitcmd"
)

const notRegularFileMode = os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// CalcRepositorySize returns the disk consumption for a given path
func CalcRepositorySize(repo Repository) (int64, error) {
	var size int64
	err := filepath.WalkDir(repoPath(repo), func(_ string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) { // ignore the error because some files (like temp/lock file) may be deleted during traversing.
			return nil
		} else if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if os.IsNotExist(err) { // ignore the error as above
			return nil
		} else if err != nil {
			return err
		}
		if (info.Mode() & notRegularFileMode) == 0 {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CountObjects returns the results of git count-objects on the repository
func CountObjects(ctx context.Context, repo Repository) (*git.CountObject, error) {
	return CountObjectsWithEnv(ctx, repo, nil)
}

// CountObjectsWithEnv returns the results of git count-objects on the repository
// with custom environment variables (e.g., GIT_QUARANTINE_PATH for pre-receive hooks)
func CountObjectsWithEnv(ctx context.Context, repo Repository, env []string) (*git.CountObject, error) {
	cmd := gitcmd.NewCommand("count-objects", "-v")
	stdout, _, err := cmd.WithDir(repoPath(repo)).WithEnv(env).RunStdString(ctx)
	if err != nil {
		return nil, err
	}

	return git.ParseCountObjectsResult(stdout), nil
}
