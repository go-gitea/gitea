// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gitea.dev/modules/git/gitcmd"
	gitealog "gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const isGogit = true

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache[*Tag]

	gogitRepo    *gogit.Repository
	gogitStorage *filesystem.Storage

	Ctx             context.Context
	LastCommitCache *LastCommitCache
	objectFormat    ObjectFormat
}

// OpenRepository opens the repository at the given path within the context.Context
func OpenRepository(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	exist, err := util.IsDir(repoPath)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, util.NewNotExistErrorf("no such file or directory")
	}

	fs := osfs.New(repoPath)
	gitDirPath := repoPath
	if _, err = fs.Stat(".git"); err == nil {
		gitDirPath = filepath.Join(repoPath, ".git")
		fs, err = fs.Chroot(".git")
		if err != nil {
			return nil, err
		}
	}

	// Regenerate any orphan packfile index before go-git tries to read it, otherwise a single
	// pack without its ".idx" makes the whole repository unreadable with "packfile not found".
	// This happens on Windows (issue #38359): the gogit storage keeps ".pack" descriptors open,
	// so git's repack cleanup can delete the ".idx" while failing to unlink the locked ".pack".
	repairOrphanPackIndexes(ctx, gitDirPath)
	// the "clone --shared" repo doesn't work well with go-git AlternativeFS, https://github.com/go-git/go-git/issues/1006
	// so use "/" for AlternatesFS, I guess it is the same behavior as current nogogit (no limitation or check for the "objects/info/alternates" paths), trust the "clone" command executed by the server.
	var altFs billy.Filesystem
	if setting.IsWindows {
		altFs = osfs.New(filepath.VolumeName(setting.RepoRootPath) + "\\") // TODO: does it really work for Windows? Need some time to check.
	} else {
		altFs = osfs.New("/")
	}
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: setting.Git.LargeObjectThreshold, AlternatesFS: altFs})
	gogitRepo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Path:         repoPath,
		gogitRepo:    gogitRepo,
		gogitStorage: storage,
		tagCache:     newObjectCache[*Tag](),
		Ctx:          ctx,
		objectFormat: ParseGogitHash(plumbing.ZeroHash).Type(),
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() error {
	if repo == nil || repo.gogitStorage == nil {
		return nil
	}
	if err := repo.gogitStorage.Close(); err != nil {
		gitealog.Error("Error closing storage: %v", err)
	}
	repo.gogitStorage = nil
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return nil
}

// GoGitRepo gets the go-git repo representation
func (repo *Repository) GoGitRepo() *gogit.Repository {
	return repo.gogitRepo
}

// repairOrphanPackIndexes finds packfiles that lost their ".idx" and regenerates it with
// "git index-pack". It is a no-op for healthy repositories (the common case) and never
// returns an error: a repository that cannot be repaired is left for go-git to report.
func repairOrphanPackIndexes(ctx context.Context, gitDirPath string) {
	packDir := filepath.Join(gitDirPath, "objects", "pack")
	entries, err := os.ReadDir(packDir)
	if err != nil {
		return // no pack dir (e.g. empty repo) or unreadable; nothing to repair
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".pack") {
			continue
		}
		idxPath := filepath.Join(packDir, strings.TrimSuffix(name, ".pack")+".idx")
		if _, err = os.Stat(idxPath); err == nil || !os.IsNotExist(err) {
			continue // index present, or an unexpected stat error we should not act on
		}
		packPath := filepath.Join(packDir, name)
		if err = gitcmd.NewCommand("index-pack").AddDynamicArguments(packPath).WithDir(gitDirPath).Run(ctx); err != nil {
			gitealog.Error("Failed to regenerate missing index for orphan packfile %q: %v", packPath, err)
		} else {
			gitealog.Warn("Regenerated missing index for orphan packfile %q", packPath)
		}
	}
}
