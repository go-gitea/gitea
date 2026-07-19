// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/util"
)

var repoPath = gitcmd.RepoLocalPath // some functions need to operate local filesystem directly

// IsRepositoryExist returns true if the repository directory exists in the disk
func IsRepositoryExist(ctx context.Context, repo git.RepositoryFacade) (bool, error) {
	return util.IsExist(repoPath(repo))
}

// DeleteRepository deletes the repository directory from the disk, it will return
// nil if the repository does not exist.
func DeleteRepository(ctx context.Context, repo git.RepositoryFacade) error {
	return util.RemoveAll(repoPath(repo))
}

// RenameRepository renames a repository's name on disk
func RenameRepository(ctx context.Context, repo, newRepo git.RepositoryFacade) error {
	dstDir := repoPath(newRepo)
	if err := os.MkdirAll(filepath.Dir(dstDir), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to create dir %s: %w", filepath.Dir(dstDir), err)
	}

	if err := util.Rename(repoPath(repo), dstDir); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}

func InitRepository(ctx context.Context, repo git.RepositoryFacade, objectFormatName string) error {
	return git.InitRepository(ctx, repoPath(repo), true, objectFormatName)
}

func GetRepoFS(repo git.RepositoryFacade) fs.FS {
	return os.DirFS(repoPath(repo))
}

func IsRepoFileExist(ctx context.Context, repo git.RepositoryFacade, relativeFilePath string) (bool, error) {
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFilePath)
	return util.IsExist(absoluteFilePath)
}

func IsRepoDirExist(ctx context.Context, repo git.RepositoryFacade, relativeDirPath string) (bool, error) {
	absoluteDirPath := filepath.Join(repoPath(repo), relativeDirPath)
	return util.IsDir(absoluteDirPath)
}

func RemoveRepoFileOrDir(ctx context.Context, repo git.RepositoryFacade, relativeFileOrDirPath string) error {
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFileOrDirPath)
	return util.Remove(absoluteFilePath)
}

func CreateRepoFile(ctx context.Context, repo git.RepositoryFacade, relativeFilePath string) (io.WriteCloser, error) {
	absoluteFilePath := filepath.Join(repoPath(repo), relativeFilePath)
	if err := os.MkdirAll(filepath.Dir(absoluteFilePath), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(absoluteFilePath)
}
