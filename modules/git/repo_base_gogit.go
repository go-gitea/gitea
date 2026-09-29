// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"errors"
	"path/filepath"
	"slices"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

const isGogit = true

type Repository struct {
	RepositoryBase

	gogitRepo    *gogit.Repository
	gogitStorage *reindexingStorage
}

// reindexingStorage picks up packs that git wrote after go-git loaded its index
type reindexingStorage struct {
	*filesystem.Storage
	packs []plumbing.Hash
}

func (s *reindexingStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	obj, err := s.Storage.EncodedObject(t, h)
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		return obj, err
	}
	packs, _ := s.ObjectPacks()
	if slices.Equal(packs, s.packs) {
		return obj, err
	}
	s.packs = packs
	s.Reindex()
	return s.Storage.EncodedObject(t, h)
}

func openRepositoryInternal(gitRepo *Repository) error {
	repoPath := gitrepo.RepoLocalPath(gitRepo)
	fs := osfs.New(repoPath)
	_, err := fs.Stat(".git")
	if err == nil {
		fs, err = fs.Chroot(".git")
		if err != nil {
			return err
		}
	}
	// the "clone --shared" repo doesn't work well with go-git AlternativeFS, https://github.com/go-git/go-git/issues/1006
	// so use "/" for AlternatesFS, I guess it is the same behavior as current nogogit (no limitation or check for the "objects/info/alternates" paths), trust the "clone" command executed by the server.
	var altFs billy.Filesystem
	if setting.IsWindows {
		altFs = osfs.New(filepath.VolumeName(setting.RepoRootPath) + "\\") // TODO: does it really work for Windows? Need some time to check.
	} else {
		altFs = osfs.New("/")
	}
	gitRepo.objectFormatCache = ParseGogitHash(plumbing.ZeroHash).Type()
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: setting.Git.LargeObjectThreshold, AlternatesFS: altFs})
	packs, _ := storage.ObjectPacks()
	gitRepo.gogitStorage = &reindexingStorage{Storage: storage, packs: packs}
	gitRepo.gogitRepo, err = gogit.Open(gitRepo.gogitStorage, fs)
	if err != nil {
		_ = gitRepo.gogitStorage.Close()
		return err
	}
	return nil
}

func (repo *Repository) closeInternal() error {
	if repo.gogitStorage == nil {
		return nil
	}
	err := repo.gogitStorage.Close()
	repo.gogitStorage = nil
	return err
}

// GoGitRepo gets the go-git repo representation
func (repo *Repository) GoGitRepo() *gogit.Repository {
	return repo.gogitRepo
}
