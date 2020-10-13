// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"

	"code.gitea.io/gitea/modules/setting"
)

var (
	// ErrURLNotSupported represents url is not supported
	ErrURLNotSupported = errors.New("url method not supported")
	// ErrIterateObjectsNotSupported represents IterateObjects not supported
	ErrIterateObjectsNotSupported = errors.New("iterateObjects method not supported")
)

// ErrInvalidConfiguration is called when there is invalid configuration for a storage
type ErrInvalidConfiguration struct {
	cfg interface{}
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

// Type is a type of Storage
type Type string

// NewStorageFunc is a function that creates a storage
type NewStorageFunc func(ctx context.Context, cfg interface{}) (ObjectStorage, error)

var storageMap = map[Type]NewStorageFunc{}

// RegisterStorageType registers a provided storage type with a function to create it
func RegisterStorageType(typ Type, fn func(ctx context.Context, cfg interface{}) (ObjectStorage, error)) {
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
	Save(path string, r io.Reader) (int64, error)
	Stat(path string) (os.FileInfo, error)
	Delete(path string) error
	URL(path, name string) (*url.URL, error)
	IterateObjects(func(path string, obj Object) error) error
}

// Copy copys a file from source ObjectStorage to dest ObjectStorage
func Copy(dstStorage ObjectStorage, dstPath string, srcStorage ObjectStorage, srcPath string) (int64, error) {
	f, err := srcStorage.Open(srcPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return dstStorage.Save(dstPath, f)
}

var (
	// Attachments represents attachments storage
	Attachments ObjectStorage

	// LFS represents lfs storage
	LFS ObjectStorage
)

// Init init the stoarge
func Init() error {
	if err := initAttachments(); err != nil {
		return err
	}

	return initLFS()
}

// NewStorage takes a storage type and some config and returns an ObjectStorage or an error
func NewStorage(typStr string, cfg interface{}) (ObjectStorage, error) {
	if len(typStr) == 0 {
		typStr = string(LocalStorageType)
	}
	fn, ok := storageMap[Type(typStr)]
	if !ok {
		return nil, fmt.Errorf("Unsupported storage type: %s", typStr)
	}

	return fn(context.Background(), cfg)
}

func initAttachments() (err error) {
	Attachments, err = NewStorage(setting.Attachment.Storage.Type, setting.Attachment.Storage)
	return
}

func initLFS() (err error) {
	LFS, err = NewStorage(setting.LFS.Storage.Type, setting.LFS.Storage)
	return
}
