// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// ErrURLNotSupported represents url is not supported
var ErrURLNotSupported = errors.New("url method not supported")

// ErrInvalidConfiguration is called when there is invalid configuration for a storage
type ErrInvalidConfiguration struct {
	cfg any
	err error
}

func (err ErrInvalidConfiguration) Error() string {
	if err.err != nil {
		return fmt.Sprintf("Invalid Configuration Argument: %v: Error: %v", err.cfg, err.err)
	}
	return fmt.Sprintf("Invalid Configuration Argument: %v", err.cfg)
}

// IsErrInvalidConfiguration checks if an error is an ErrInvalidConfiguration
func IsErrInvalidConfiguration(err error) bool {
	_, ok := err.(ErrInvalidConfiguration)
	return ok
}

type Type = setting.StorageType

// NewStorageFunc is a function that creates a storage
type NewStorageFunc func(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error)

var storageMap = map[Type]NewStorageFunc{}

// RegisterStorageType registers a provided storage type with a function to create it
func RegisterStorageType(typ Type, fn func(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error)) {
	storageMap[typ] = fn
}

// Object represents the object on the storage
type Object interface {
	io.ReadCloser
	io.Seeker
	Stat() (os.FileInfo, error)
}

// ObjectStorage represents an object storage to handle a bucket and files
type ObjectStorage interface {
	Open(path string) (Object, error)
	// Save store a object, if size is unknown set -1
	Save(path string, r io.Reader, size int64) (int64, error)
	Stat(path string) (os.FileInfo, error)
	Delete(path string) error
	URL(path, name string) (*url.URL, error)
	IterateObjects(path string, iterator func(path string, obj Object) error) error
}

// Copy copies a file from source ObjectStorage to dest ObjectStorage
func Copy(dstStorage ObjectStorage, dstPath string, srcStorage ObjectStorage, srcPath string) (int64, error) {
	f, err := srcStorage.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	size := int64(-1)
	fsinfo, err := f.Stat()
	if err == nil {
		size = fsinfo.Size()
	}

	return dstStorage.Save(dstPath, f, size)
}

// Clean delete all the objects in this storage
func Clean(storage ObjectStorage) error {
	return storage.IterateObjects("", func(path string, obj Object) error {
		_ = obj.Close()
		return storage.Delete(path)
	})
}

// SaveFrom saves data to the ObjectStorage with path p from the callback
func SaveFrom(objStorage ObjectStorage, p string, callback func(w io.Writer) error) error {
	pr, pw := io.Pipe()
	defer pr.Close()
	go func() {
		defer pw.Close()
		if err := callback(pw); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	_, err := objStorage.Save(p, pr, -1)
	return err
}

var (
	// Attachments represents attachments storage
	Attachments ObjectStorage = uninitializedStorage

	// LFS represents lfs storage
	LFS ObjectStorage = uninitializedStorage

	// Avatars represents user avatars storage
	Avatars ObjectStorage = uninitializedStorage
	// RepoAvatars represents repository avatars storage
	RepoAvatars ObjectStorage = uninitializedStorage

	// RepoArchives represents repository archives storage
	RepoArchives ObjectStorage = uninitializedStorage

	// Packages represents packages storage
	Packages ObjectStorage = uninitializedStorage

	// Actions represents actions storage
	Actions ObjectStorage = uninitializedStorage
	// Actions Artifacts represents actions artifacts storage
	ActionsArtifacts ObjectStorage = uninitializedStorage
)

// Init init the stoarge
func Init() error {
	for _, f := range []func() error{
		initAttachments,
		initAvatars,
		initRepoAvatars,
		initLFS,
		initRepoArchives,
		initPackages,
		initActions,
	} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

// NewStorage takes a storage type and some config and returns an ObjectStorage or an error
func NewStorage(typStr Type, cfg *setting.Storage) (ObjectStorage, error) {
	if len(typStr) == 0 {
		typStr = setting.LocalStorageType
	}
	fn, ok := storageMap[typStr]
	if !ok {
		return nil, fmt.Errorf("Unsupported storage type: %s", typStr)
	}

	return fn(context.Background(), cfg)
}

func initAvatars() (err error) {
	log.Info("Initialising Avatar storage with type: %s", setting.Avatar.Storage.Type)
	Avatars, err = NewStorage(setting.Avatar.Storage.Type, setting.Avatar.Storage)
	return err
}

func initAttachments() (err error) {
	if !setting.Attachment.Enabled {
		Attachments = discardStorage("Attachment isn't enabled")
		return nil
	}
	log.Info("Initialising Attachment storage with type: %s", setting.Attachment.Storage.Type)
	Attachments, err = NewStorage(setting.Attachment.Storage.Type, setting.Attachment.Storage)
	return err
}

func initLFS() (err error) {
	if !setting.LFS.StartServer {
		LFS = discardStorage("LFS isn't enabled")
		return nil
	}
	log.Info("Initialising LFS storage with type: %s", setting.LFS.Storage.Type)
	LFS, err = NewStorage(setting.LFS.Storage.Type, setting.LFS.Storage)
	return err
}

func initRepoAvatars() (err error) {
	log.Info("Initialising Repository Avatar storage with type: %s", setting.RepoAvatar.Storage.Type)
	RepoAvatars, err = NewStorage(setting.RepoAvatar.Storage.Type, setting.RepoAvatar.Storage)
	return err
}

func initRepoArchives() (err error) {
	log.Info("Initialising Repository Archive storage with type: %s", setting.RepoArchive.Storage.Type)
	RepoArchives, err = NewStorage(setting.RepoArchive.Storage.Type, setting.RepoArchive.Storage)
	return err
}

func initPackages() (err error) {
	if !setting.Packages.Enabled {
		Packages = discardStorage("Packages isn't enabled")
		return nil
	}
	log.Info("Initialising Packages storage with type: %s", setting.Packages.Storage.Type)
	Packages, err = NewStorage(setting.Packages.Storage.Type, setting.Packages.Storage)
	return err
}

func initActions() (err error) {
	if !setting.Actions.Enabled {
		Actions = discardStorage("Actions isn't enabled")
		ActionsArtifacts = discardStorage("ActionsArtifacts isn't enabled")
		return nil
	}
	log.Info("Initialising Actions storage with type: %s", setting.Actions.LogStorage.Type)
	if Actions, err = NewStorage(setting.Actions.LogStorage.Type, setting.Actions.LogStorage); err != nil {
		return err
	}
	log.Info("Initialising ActionsArtifacts storage with type: %s", setting.Actions.ArtifactStorage.Type)
	ActionsArtifacts, err = NewStorage(setting.Actions.ArtifactStorage.Type, setting.Actions.ArtifactStorage)
	return err
}
