// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/globallock"
)

// TryLockMaintenanceAndDo skips repositories busy with other maintenance, concurrent repacks and fsck fail on each other's temporary packs
func TryLockMaintenanceAndDo(ctx context.Context, repo RepositoryFacade, fn func(ctx context.Context) error) error {
	_, err := globallock.TryLockAndDo(ctx, "repo-maintenance:"+repo.GitRepoManagedID(), fn)
	return err
}

// NeedsGarbageCollection reports loose objects or multiple packs, ignoring cruft and kept packs that "git gc" leaves in place
func NeedsGarbageCollection(repo RepositoryFacade) bool {
	objectsPath := filepath.Join(gitrepo.RepoLocalPath(repo), "objects")
	dirs, err := os.ReadDir(objectsPath)
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist) // let "git gc" report unreadable repositories, a missing wiki is normal
	}
	packPath := filepath.Join(objectsPath, "pack")
	entries, _ := os.ReadDir(packPath)
	names := make(container.Set[string], len(entries))
	for _, entry := range entries {
		names.Add(entry.Name())
	}
	packs := 0
	for name := range names {
		if base, ok := strings.CutSuffix(name, ".pack"); ok && !names.Contains(base+".mtimes") && !names.Contains(base+".keep") {
			packs++
		}
	}
	if packs > 1 {
		return true
	}
	return slices.ContainsFunc(dirs, func(dir os.DirEntry) bool {
		if len(dir.Name()) != 2 {
			return false
		}
		objects, _ := os.ReadDir(filepath.Join(objectsPath, dir.Name()))
		return slices.ContainsFunc(objects, func(object os.DirEntry) bool {
			return len(object.Name()) == 38 || len(object.Name()) == 62 // SHA-1 or SHA-256 hex without the directory's two characters
		})
	})
}
