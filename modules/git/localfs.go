// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

// IsRepositoryExist returns true if the repository directory exists in the disk
func IsRepositoryExist(ctx context.Context, repo RepositoryFacade) (bool, error) {
	return util.IsExist(gitcmd.RepoLocalPath(repo))
}

// DeleteRepository deletes the repository directory from the disk, it will return
// nil if the repository does not exist.
func DeleteRepository(ctx context.Context, repo RepositoryFacade) error {
	return util.RemoveAll(gitcmd.RepoLocalPath(repo))
}

// RenameRepository renames a repository's name on disk
func RenameRepository(ctx context.Context, repo, newRepo RepositoryFacade) error {
	dstDir := gitcmd.RepoLocalPath(newRepo)
	if err := os.MkdirAll(filepath.Dir(dstDir), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", filepath.Dir(dstDir), err)
	}

	if err := util.Rename(gitcmd.RepoLocalPath(repo), dstDir); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}

func InitRepository(ctx context.Context, repo RepositoryFacade, objectFormatName string) error {
	return InitRepositoryLocal(ctx, gitcmd.RepoLocalPath(repo), true, objectFormatName)
}

func GetRepoFS(repo RepositoryFacade) fs.FS {
	return os.DirFS(gitcmd.RepoLocalPath(repo))
}

func IsRepoFileExist(ctx context.Context, repo RepositoryFacade, relativeFilePath string) (bool, error) {
	absoluteFilePath := filepath.Join(gitcmd.RepoLocalPath(repo), relativeFilePath)
	return util.IsExist(absoluteFilePath)
}

func IsRepoDirExist(ctx context.Context, repo RepositoryFacade, relativeDirPath string) (bool, error) {
	absoluteDirPath := filepath.Join(gitcmd.RepoLocalPath(repo), relativeDirPath)
	return util.IsDir(absoluteDirPath)
}

func RemoveRepoFileOrDir(ctx context.Context, repo RepositoryFacade, relativeFileOrDirPath string) error {
	absoluteFilePath := filepath.Join(gitcmd.RepoLocalPath(repo), relativeFileOrDirPath)
	return util.Remove(absoluteFilePath)
}

func CreateRepoFile(ctx context.Context, repo RepositoryFacade, relativeFilePath string) (io.WriteCloser, error) {
	absoluteFilePath := filepath.Join(gitcmd.RepoLocalPath(repo), relativeFilePath)
	if err := os.MkdirAll(filepath.Dir(absoluteFilePath), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(absoluteFilePath)
}
