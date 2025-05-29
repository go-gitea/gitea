// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
)

const notRegularFileMode = os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// getDirectorySize returns the disk consumption for a given path
func getDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
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

// UpdateRepoSize updates the repository size, calculating it using getDirectorySize
func UpdateRepoSize(ctx context.Context, repo *repo_model.Repository) error {
	size, err := getDirectorySize(repo.RepoPath())
	if err != nil {
		return fmt.Errorf("updateSize: %w", err)
	}

	lfsSize, err := git_model.GetRepoLFSSize(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("updateSize: GetLFSMetaObjects: %w", err)
	}

	return repo_model.UpdateRepoSize(ctx, repo.ID, size, lfsSize)
}
