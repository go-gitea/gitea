// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"gitea.dev/modules/git/gitrepo"
)

// NeedsGarbageCollection reports whether the repository is not fully packed
func NeedsGarbageCollection(repo RepositoryFacade) bool {
	objectsPath := filepath.Join(gitrepo.RepoLocalPath(repo), "objects")
	dirs, err := os.ReadDir(objectsPath)
	if err != nil {
		return !errors.Is(err, fs.ErrNotExist) // let "git gc" report unreadable repositories, a missing wiki is normal
	}
	packFiles, _ := os.ReadDir(filepath.Join(objectsPath, "pack"))
	packs := 0
	for _, packFile := range packFiles {
		switch filepath.Ext(packFile.Name()) {
		case ".pack":
			packs++
		case ".mtimes", ".keep":
			packs--
		}
	}
	return packs > 1 || slices.ContainsFunc(dirs, func(dir os.DirEntry) bool {
		if len(dir.Name()) != 2 {
			return false
		}
		objects, _ := os.ReadDir(filepath.Join(objectsPath, dir.Name()))
		return len(objects) > 0
	})
}
