// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

func absWikiPath(owner, name string) string {
	return filepath.Join(setting.RepoRootPath, strings.ToLower(owner), strings.ToLower(name)+".wiki.git")
}

func wikiPath(repo Repository) string {
	return absWikiPath(repo.GetOwnerName(), repo.GetName())
}

func OpenWikiRepository(ctx context.Context, repo Repository) (*git.Repository, error) {
	return git.OpenRepository(ctx, wikiPath(repo))
}

// IsWikiRepositoryExist returns true if the repository directory exists in the disk
func IsWikiRepositoryExist(ctx context.Context, repo Repository) (bool, error) {
	return util.IsExist(wikiPath(repo))
}

// RenameRepository renames a repository's name on disk
func RenameWikiRepository(ctx context.Context, repo Repository, newName string) error {
	newRepoPath := absWikiPath(repo.GetOwnerName(), newName)
	if err := util.Rename(wikiPath(repo), newRepoPath); err != nil {
		return fmt.Errorf("rename repository directory: %w", err)
	}
	return nil
}

// DeleteWikiRepository deletes the repository directory from the disk
func DeleteWikiRepository(ctx context.Context, repo Repository) error {
	return util.RemoveAll(wikiPath(repo))
}
