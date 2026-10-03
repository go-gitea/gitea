// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build gogit

package git

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gitea.dev/modules/container"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/setting"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

const isGogit = true

type Repository struct {
	RepositoryBase

	gogitRepo    *gogit.Repository
	gogitStorage *reindexingStorage
}

// reindexingStorage reloads the pack index when git added or removed packs after go-git loaded it
// https://github.com/go-git/go-git/issues/2439 https://github.com/go-git/go-git/issues/1623
// FIXME: gogit workaround, remove with the gogit build
type reindexingStorage struct {
	*filesystem.Storage
	packs []plumbing.Hash
}

func isRepackError(err error) bool {
	return errors.Is(err, plumbing.ErrObjectNotFound) || errors.Is(err, dotgit.ErrPackfileNotFound) || errors.Is(err, os.ErrNotExist)
}

// retry reruns fn while a concurrent repack keeps changing the packs
func (s *reindexingStorage) retry(fn func() error) error {
	for {
		err := fn()
		if !isRepackError(err) {
			return err
		}
		packs, _ := s.ObjectPacks()
		if slices.Equal(packs, s.packs) {
			return err
		}
		s.packs = packs
		s.Reindex()
	}
}

func (s *reindexingStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (obj plumbing.EncodedObject, err error) {
	err = s.retry(func() (err error) {
		obj, err = s.Storage.EncodedObject(t, h)
		return err
	})
	if err != nil {
		return nil, err
	}
	if _, ok := obj.(*plumbing.MemoryObject); ok {
		return obj, nil
	}
	return &lazyObject{EncodedObject: obj, storage: s}, nil
}

// lazyObject looks up a large object again when its file got removed before Reader reopened it
// FIXME: gogit workaround, remove with the gogit build
type lazyObject struct {
	plumbing.EncodedObject
	storage *reindexingStorage
}

func (o *lazyObject) Reader() (rc io.ReadCloser, err error) {
	rc, err = o.EncodedObject.Reader()
	if !isRepackError(err) {
		return rc, err
	}
	err = o.storage.retry(func() error {
		obj, err := o.storage.Storage.EncodedObject(o.Type(), o.Hash())
		if err == nil {
			o.EncodedObject = obj
			rc, err = obj.Reader()
		}
		return err
	})
	return rc, err
}

// packIdxFS lists packs like git, only while their .idx exists
// FIXME: gogit workaround, remove with the gogit build
type packIdxFS struct {
	billy.Filesystem
}

func (f packIdxFS) ReadDir(dir string) ([]os.FileInfo, error) {
	if dir != f.Join("objects", "pack") {
		return f.Filesystem.ReadDir(dir)
	}
	dirFile, err := os.Open(filepath.Join(f.Root(), dir))
	if err != nil {
		return nil, err
	}
	defer dirFile.Close()
	infos, err := dirFile.Readdir(-1) // skips files removed before their lstat, unlike billy's ReadDir
	if err != nil {
		return nil, err
	}
	names := make(container.Set[string], len(infos))
	for _, info := range infos {
		names.Add(info.Name())
	}
	return slices.DeleteFunc(infos, func(info os.FileInfo) bool {
		base, isPack := strings.CutSuffix(info.Name(), ".pack")
		return isPack && !names.Contains(base+".idx")
	}), nil
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
	storage := filesystem.NewStorageWithOptions(packIdxFS{fs}, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: setting.Git.LargeObjectThreshold, AlternatesFS: altFs})
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
